/*
SPDX-FileCopyrightText: Red Hat

SPDX-License-Identifier: Apache-2.0
*/

// Package e2e_envtest provides end-to-end integration tests combining:
//   - testcontainers: Real PostgreSQL database in Docker
//   - envtest: Kubernetes API server with CRDs and controllers
//   - httptest: HTTP API server for REST verification
//
// This tests the full flow: CR creation -> controller reconciliation ->
// collector watch -> database persistence -> REST API response.
package e2e_envtest

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8senvtest "sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	inventoryv1alpha1 "github.com/openshift-kni/oran-o2ims/api/inventory/v1alpha1"
	"github.com/openshift-kni/oran-o2ims/internal/controllers"
	ctlrutils "github.com/openshift-kni/oran-o2ims/internal/controllers/utils"
	"github.com/openshift-kni/oran-o2ims/internal/service/common/api/middleware"
	"github.com/openshift-kni/oran-o2ims/internal/service/common/notifier"
	commonrepo "github.com/openshift-kni/oran-o2ims/internal/service/common/repo"
	"github.com/openshift-kni/oran-o2ims/internal/service/resources"
	"github.com/openshift-kni/oran-o2ims/internal/service/resources/api"
	"github.com/openshift-kni/oran-o2ims/internal/service/resources/api/generated"
	"github.com/openshift-kni/oran-o2ims/internal/service/resources/collector"
	"github.com/openshift-kni/oran-o2ims/internal/service/resources/db/repo"
	"github.com/openshift-kni/oran-o2ims/internal/service/resources/db/testhelpers"
)

const testNamespace = "test-e2e"

var (
	testEnv        *k8senvtest.Environment
	k8sClient      client.Client
	k8sWatchClient client.WithWatch

	pgContainer *testhelpers.PostgresContainer
	testServer  *httptest.Server

	resourceCollector *collector.Collector
	collectorCancel   context.CancelFunc

	ctx    context.Context
	cancel context.CancelFunc
	logger *slog.Logger
)

// noopNotificationHandler satisfies collector.NotificationHandler for testing.
type noopNotificationHandler struct{}

func (n *noopNotificationHandler) Notify(_ context.Context, _ *notifier.Notification) {}

// noopSubscriptionEventHandler satisfies notifier.SubscriptionEventHandler for the API server.
type noopSubscriptionEventHandler struct{}

func (n *noopSubscriptionEventHandler) SubscriptionEvent(_ context.Context, _ *notifier.SubscriptionEvent) {
}

func (n *noopSubscriptionEventHandler) GetClientFactory() notifier.ClientProvider { return nil }

const bmhCRDURLTemplate = "https://raw.githubusercontent.com/metal3-io/baremetal-operator/%s/config/base/crds/bases/metal3.io_baremetalhosts.yaml"

func getBMHVersionFromGoMod() (string, error) {
	goModPath := filepath.Join("..", "..", "..", "..", "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}
	re := regexp.MustCompile(`github\.com/metal3-io/baremetal-operator/apis\s+(v[\d.]+)`)
	matches := re.FindSubmatch(data)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not find baremetal-operator version in go.mod")
	}
	return string(matches[1]), nil
}

func downloadBMHCRD() (string, error) {
	version, err := getBMHVersionFromGoMod()
	if err != nil {
		return "", fmt.Errorf("failed to get BMH version: %w", err)
	}

	url := fmt.Sprintf(bmhCRDURLTemplate, version)
	destPath := filepath.Join("testdata", "metal3.io_baremetalhosts.yaml")

	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("failed to download BMH CRD: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download BMH CRD: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read BMH CRD response: %w", err)
	}

	content := string(data)
	if strings.HasPrefix(strings.TrimSpace(content), "<") {
		return "", fmt.Errorf("received HTML instead of YAML")
	}
	if !strings.Contains(content, "kind: CustomResourceDefinition") {
		return "", fmt.Errorf("missing 'kind: CustomResourceDefinition'")
	}

	if err := os.WriteFile(destPath, data, 0o600); err != nil {
		return "", fmt.Errorf("failed to write BMH CRD: %w", err)
	}

	return version, nil
}

func TestE2EEnvtest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Envtest Suite (testcontainers + envtest + httptest)")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())

	options := &slog.HandlerOptions{Level: slog.LevelInfo}
	handler := slog.NewJSONHandler(GinkgoWriter, options)
	logger = slog.New(handler)
	slog.SetDefault(logger)

	adapter := logr.FromSlogHandler(logger.Handler())
	ctrl.SetLogger(adapter)
	klog.SetLogger(adapter)

	// ==================== PHASE 1: PostgreSQL via testcontainers ====================
	logger.Info("Starting PostgreSQL container via testcontainers...")
	var err error
	pgContainer, err = testhelpers.NewPostgresContainer(ctx)
	Expect(err).ToNot(HaveOccurred(), "Failed to start PostgreSQL container")

	err = pgContainer.RunMigrations(resources.MigrationsFS, resources.MigrationsDir)
	Expect(err).ToNot(HaveOccurred(), "Failed to run database migrations")
	logger.Info("PostgreSQL ready with migrations applied")

	// ==================== PHASE 2: Kubernetes envtest ====================
	logger.Info("Starting Kubernetes envtest...")

	if version, err := downloadBMHCRD(); err != nil {
		logger.Warn("Failed to download BMH CRD from upstream, continuing without it", "error", err)
	} else {
		logger.Info("Downloaded BMH CRD from upstream", "version", version)
	}

	scheme := runtime.NewScheme()
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	Expect(inventoryv1alpha1.AddToScheme(scheme)).To(Succeed())

	testEnv = &k8senvtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "..", "config", "crd", "bases"),
			"testdata",
		},
		ErrorIfCRDPathMissing: false,
		Scheme:                scheme,
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})
	Expect(err).ToNot(HaveOccurred())

	err = ctlrutils.SetupHierarchyIndexers(ctx, mgr)
	Expect(err).ToNot(HaveOccurred())

	err = (&controllers.LocationReconciler{
		Client: mgr.GetClient(),
		Logger: logger.With("controller", "Location"),
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	err = (&controllers.OCloudSiteReconciler{
		Client: mgr.GetClient(),
		Logger: logger.With("controller", "OCloudSite"),
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	err = (&controllers.ResourcePoolReconciler{
		Client: mgr.GetClient(),
		Logger: logger.With("controller", "ResourcePool"),
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(ctx)).To(Succeed())
	}()

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).ToNot(HaveOccurred())

	k8sWatchClient, err = client.NewWithWatch(cfg, client.Options{Scheme: scheme})
	Expect(err).ToNot(HaveOccurred())

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	os.Setenv("ORAN_O2IMS_NAMESPACE", testNamespace)

	logger.Info("Kubernetes envtest ready with controllers running")

	// ==================== PHASE 3: Setup Collector ====================
	logger.Info("Setting up collectors...")

	testCloudID := uuid.New()

	locationDS, err := collector.NewLocationDataSource(testCloudID, k8sWatchClient)
	Expect(err).ToNot(HaveOccurred())

	oCloudSiteDS, err := collector.NewOCloudSiteDataSource(testCloudID, k8sWatchClient)
	Expect(err).ToNot(HaveOccurred())

	resourcePoolDS, err := collector.NewResourcePoolDataSource(testCloudID, k8sWatchClient)
	Expect(err).ToNot(HaveOccurred())

	repository := &repo.ResourcesRepository{
		CommonRepository: commonrepo.CommonRepository{
			Db: pgContainer.Pool,
		},
	}

	dataSources := []collector.DataSource{locationDS, oCloudSiteDS, resourcePoolDS}
	resourceCollector = collector.NewCollector(
		pgContainer.Pool, repository, &noopNotificationHandler{}, dataSources,
	)

	var collectorCtx context.Context
	collectorCtx, collectorCancel = context.WithCancel(ctx)
	go func() {
		defer GinkgoRecover()
		if err := resourceCollector.Run(collectorCtx); err != nil && collectorCtx.Err() == nil {
			logger.Error("Collector exited with error", "error", err)
		}
	}()

	time.Sleep(500 * time.Millisecond)
	logger.Info("Collectors started and watching for CRs")

	// ==================== PHASE 4: Setup HTTP API Server ====================
	logger.Info("Setting up HTTP API server...")

	server := &api.ResourceServer{
		Repo: repository,
		Info: generated.OCloudInfo{
			OCloudId:      uuid.New(),
			GlobalCloudId: uuid.New(),
			Name:          "e2e-test-cloud",
			Description:   "E2E Test O-Cloud",
			ServiceUri:    "https://e2e-test.example.com",
		},
		SubscriptionEventHandler: &noopSubscriptionEventHandler{},
	}

	strictHandler := generated.NewStrictHandlerWithOptions(server, nil,
		generated.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  middleware.GetOranReqErrFunc(),
			ResponseErrorHandlerFunc: middleware.GetOranRespErrFunc(),
		},
	)

	swagger, err := generated.GetSwagger()
	Expect(err).ToNot(HaveOccurred())

	apiLogger := slog.New(slog.NewTextHandler(GinkgoWriter, &slog.HandlerOptions{Level: slog.LevelWarn}))
	filterAdapter, err := middleware.NewFilterAdapterFromSwagger(apiLogger, swagger)
	Expect(err).ToNot(HaveOccurred())

	opt := generated.StdHTTPServerOptions{
		BaseRouter: http.NewServeMux(),
		Middlewares: []generated.MiddlewareFunc{
			middleware.OpenAPIValidation(swagger),
			middleware.ResponseFilter(filterAdapter),
			middleware.LogDuration(),
		},
		ErrorHandlerFunc: middleware.GetOranReqErrFunc(),
	}
	httpHandler := generated.HandlerWithOptions(strictHandler, opt)
	testServer = httptest.NewServer(httpHandler)
	logger.Info("HTTP API server ready", "url", testServer.URL)

	logger.Info("========== E2E Test Environment Ready ==========")
})

var _ = AfterSuite(func() {
	logger.Info("Tearing down E2E test environment...")

	if collectorCancel != nil {
		collectorCancel()
	}
	if testServer != nil {
		testServer.Close()
	}
	cancel()
	if testEnv != nil {
		Expect(testEnv.Stop()).To(Succeed())
	}
	if pgContainer != nil {
		_ = pgContainer.Terminate(context.Background())
	}

	logger.Info("E2E test environment torn down")
})

const (
	defaultTimeout  = 30 * time.Second
	defaultInterval = 500 * time.Millisecond
	basePath        = "/o2ims-infrastructureInventory/v1"
)

func waitForLocationReady(location *inventoryv1alpha1.Location) {
	Eventually(func() bool {
		fetched := &inventoryv1alpha1.Location{}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(location), fetched); err != nil {
			return false
		}
		for _, cond := range fetched.Status.Conditions {
			if cond.Type == inventoryv1alpha1.ConditionTypeReady && cond.Status == metav1.ConditionTrue {
				return true
			}
		}
		return false
	}, defaultTimeout, defaultInterval).Should(BeTrue(), "Location should become Ready")
}

func waitForOCloudSiteReady(site *inventoryv1alpha1.OCloudSite) {
	Eventually(func() bool {
		fetched := &inventoryv1alpha1.OCloudSite{}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(site), fetched); err != nil {
			return false
		}
		for _, cond := range fetched.Status.Conditions {
			if cond.Type == inventoryv1alpha1.ConditionTypeReady && cond.Status == metav1.ConditionTrue {
				return true
			}
		}
		return false
	}, defaultTimeout, defaultInterval).Should(BeTrue(), "OCloudSite should become Ready")
}

func waitForResourcePoolReady(pool *inventoryv1alpha1.ResourcePool) {
	Eventually(func() bool {
		fetched := &inventoryv1alpha1.ResourcePool{}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), fetched); err != nil {
			return false
		}
		for _, cond := range fetched.Status.Conditions {
			if cond.Type == inventoryv1alpha1.ConditionTypeReady && cond.Status == metav1.ConditionTrue {
				return true
			}
		}
		return false
	}, defaultTimeout, defaultInterval).Should(BeTrue(), "ResourcePool should become Ready")
}

func ptrString(s string) *string {
	return &s
}
