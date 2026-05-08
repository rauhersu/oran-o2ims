/*
SPDX-FileCopyrightText: Red Hat

SPDX-License-Identifier: Apache-2.0
*/

package e2e_envtest

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	inventoryv1alpha1 "github.com/openshift-kni/oran-o2ims/api/inventory/v1alpha1"
	"github.com/openshift-kni/oran-o2ims/internal/service/resources/api/generated"
)

var _ = Describe("E2E Hierarchy Tests", Label("testcontainers", "integration"), func() {

	Describe("Complete Hierarchy: Location -> OCloudSite -> ResourcePool -> REST API", func() {
		var (
			location     *inventoryv1alpha1.Location
			oCloudSite   *inventoryv1alpha1.OCloudSite
			resourcePool *inventoryv1alpha1.ResourcePool
		)

		It("should create the full hierarchy and expose it via the REST API", func() {

			By("Step 1: Creating a Location CR (east-datacenter)")
			location = &inventoryv1alpha1.Location{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "east-datacenter",
					Namespace: testNamespace,
				},
				Spec: inventoryv1alpha1.LocationSpec{
					Description: "Primary east coast data center facility",
					Address:     ptrString("123 Technology Way, Ashburn, VA 20147, USA"),
				},
			}
			Expect(k8sClient.Create(ctx, location)).To(Succeed())

			By("Waiting for Location to become Ready")
			waitForLocationReady(location)

			By("Step 2: Creating an OCloudSite CR (site-east-1)")
			oCloudSite = &inventoryv1alpha1.OCloudSite{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "site-east-1",
					Namespace: testNamespace,
				},
				Spec: inventoryv1alpha1.OCloudSiteSpec{
					GlobalLocationName: "east-datacenter",
					Description:        "Primary compute site at east data center",
				},
			}
			Expect(k8sClient.Create(ctx, oCloudSite)).To(Succeed())

			By("Waiting for OCloudSite to become Ready")
			waitForOCloudSiteReady(oCloudSite)

			By("Step 3: Creating a ResourcePool CR (pool-east-compute)")
			resourcePool = &inventoryv1alpha1.ResourcePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pool-east-compute",
					Namespace: testNamespace,
				},
				Spec: inventoryv1alpha1.ResourcePoolSpec{
					OCloudSiteName: "site-east-1",
					Description:    "Compute resources for production workloads",
				},
			}
			Expect(k8sClient.Create(ctx, resourcePool)).To(Succeed())

			By("Waiting for ResourcePool to become Ready")
			waitForResourcePoolReady(resourcePool)

			By("Allowing collector to process watch events")
			time.Sleep(2 * time.Second)

			// ==================== API Verification ====================

			By("Verifying Location via GET /locations")
			Eventually(func() bool {
				resp, err := http.Get(testServer.URL + basePath + "/locations")
				if err != nil {
					return false
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return false
				}
				body, _ := io.ReadAll(resp.Body)
				var locations []generated.LocationInfo
				if err := json.Unmarshal(body, &locations); err != nil {
					return false
				}
				for _, loc := range locations {
					if loc.GlobalLocationId == "east-datacenter" {
						return true
					}
				}
				return false
			}, defaultTimeout, defaultInterval).Should(BeTrue(), "Location should appear in API response")

			By("Verifying Location details via GET /locations/{globalLocationId}")
			resp, err := http.Get(testServer.URL + basePath + "/locations/" + location.Name)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, _ := io.ReadAll(resp.Body)
			var locationResp generated.LocationInfo
			Expect(json.Unmarshal(body, &locationResp)).To(Succeed())

			Expect(locationResp.GlobalLocationId).To(Equal(location.Name))
			Expect(locationResp.Name).To(Equal(location.Name))
			Expect(locationResp.Description).To(Equal(location.Spec.Description))
			Expect(locationResp.Address).To(HaveValue(Equal(*location.Spec.Address)))

			By("Verifying OCloudSite via GET /oCloudSites")
			Eventually(func() bool {
				resp, err := http.Get(testServer.URL + basePath + "/oCloudSites")
				if err != nil {
					return false
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return false
				}
				body, _ := io.ReadAll(resp.Body)
				var sites []generated.OCloudSiteInfo
				if err := json.Unmarshal(body, &sites); err != nil {
					return false
				}
				for _, site := range sites {
					if site.Name == "site-east-1" {
						return true
					}
				}
				return false
			}, defaultTimeout, defaultInterval).Should(BeTrue(), "OCloudSite should appear in API response")

			By("Getting OCloudSite UID for API verification")
			fetchedSite := &inventoryv1alpha1.OCloudSite{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(oCloudSite), fetchedSite)).To(Succeed())
			siteUUID, err := uuid.Parse(string(fetchedSite.UID))
			Expect(err).ToNot(HaveOccurred())

			By("Verifying OCloudSite details via GET /oCloudSites/{oCloudSiteId}")
			resp, err = http.Get(testServer.URL + basePath + "/oCloudSites/" + siteUUID.String())
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, _ = io.ReadAll(resp.Body)
			var siteResp generated.OCloudSiteInfo
			Expect(json.Unmarshal(body, &siteResp)).To(Succeed())

			Expect(siteResp.OCloudSiteId).To(Equal(siteUUID))
			Expect(siteResp.Name).To(Equal(oCloudSite.Name))
			Expect(siteResp.GlobalLocationId).To(Equal(location.Name))
			Expect(siteResp.Description).To(Equal(oCloudSite.Spec.Description))

			By("Verifying ResourcePool via GET /resourcePools")
			Eventually(func() bool {
				resp, err := http.Get(testServer.URL + basePath + "/resourcePools")
				if err != nil {
					return false
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return false
				}
				body, _ := io.ReadAll(resp.Body)
				var pools []generated.ResourcePool
				if err := json.Unmarshal(body, &pools); err != nil {
					return false
				}
				for _, pool := range pools {
					if pool.Name == "pool-east-compute" {
						return true
					}
				}
				return false
			}, defaultTimeout, defaultInterval).Should(BeTrue(), "ResourcePool should appear in API response")

			By("Getting ResourcePool UID for API verification")
			fetchedPool := &inventoryv1alpha1.ResourcePool{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(resourcePool), fetchedPool)).To(Succeed())
			poolUUID, err := uuid.Parse(string(fetchedPool.UID))
			Expect(err).ToNot(HaveOccurred())

			By("Verifying ResourcePool details via GET /resourcePools/{resourcePoolId}")
			resp, err = http.Get(testServer.URL + basePath + "/resourcePools/" + poolUUID.String())
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, _ = io.ReadAll(resp.Body)
			var poolResp generated.ResourcePool
			Expect(json.Unmarshal(body, &poolResp)).To(Succeed())

			Expect(poolResp.ResourcePoolId).To(Equal(poolUUID))
			Expect(poolResp.Name).To(Equal(resourcePool.Name))
			Expect(poolResp.OCloudSiteId).To(Equal(siteUUID))
			Expect(poolResp.Description).To(Equal(resourcePool.Spec.Description))

			By("Verifying parent-child: Location references OCloudSite")
			resp, err = http.Get(testServer.URL + basePath + "/locations/" + location.Name)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			body, _ = io.ReadAll(resp.Body)
			Expect(json.Unmarshal(body, &locationResp)).To(Succeed())
			Expect(locationResp.OCloudSiteIds).To(ContainElement(siteUUID))

			By("Verifying parent-child: OCloudSite references ResourcePool")
			resp, err = http.Get(testServer.URL + basePath + "/oCloudSites/" + siteUUID.String())
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			body, _ = io.ReadAll(resp.Body)
			Expect(json.Unmarshal(body, &siteResp)).To(Succeed())
			Expect(siteResp.ResourcePools).To(ContainElement(poolUUID))
		})

		AfterEach(func() {
			if resourcePool != nil {
				_ = k8sClient.Delete(ctx, resourcePool)
			}
			if oCloudSite != nil {
				_ = k8sClient.Delete(ctx, oCloudSite)
			}
			if location != nil {
				_ = k8sClient.Delete(ctx, location)
			}
		})
	})

	Describe("API Error Responses", func() {
		It("should return 404 for non-existent Location", func() {
			resp, err := http.Get(testServer.URL + basePath + "/locations/non-existent-location")
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("should return 404 for non-existent ResourcePool", func() {
			nonExistentID := uuid.New()
			resp, err := http.Get(testServer.URL + basePath + "/resourcePools/" + nonExistentID.String())
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("should return 400 for invalid UUID in ResourcePool endpoint", func() {
			resp, err := http.Get(testServer.URL + basePath + "/resourcePools/not-a-uuid")
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})
})
