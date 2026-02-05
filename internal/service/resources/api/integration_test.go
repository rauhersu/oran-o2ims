/*
SPDX-FileCopyrightText: Red Hat

SPDX-License-Identifier: Apache-2.0
*/

package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	commonapi "github.com/openshift-kni/oran-o2ims/internal/service/common/api/generated"
	"github.com/openshift-kni/oran-o2ims/internal/service/common/api/middleware"
	"github.com/openshift-kni/oran-o2ims/internal/service/common/deprecation"
	commonrepo "github.com/openshift-kni/oran-o2ims/internal/service/common/repo"
	"github.com/openshift-kni/oran-o2ims/internal/service/resources/api"
	"github.com/openshift-kni/oran-o2ims/internal/service/resources/api/generated"
	"github.com/openshift-kni/oran-o2ims/internal/service/resources/db"
	"github.com/openshift-kni/oran-o2ims/internal/service/resources/db/repo"
	"github.com/openshift-kni/oran-o2ims/internal/service/resources/db/testhelpers"
)

const (
	basePath    = "/o2ims-infrastructureInventory/v1"
	docsBaseURL = "https://test.example.com" // Base URL for deprecation Link headers
)

var _ = Describe("ResourceServer Integration", Label("integration"), func() {
	var (
		ctx        context.Context
		cancel     context.CancelFunc
		container  *testhelpers.PostgresContainer
		testServer *httptest.Server
	)

	BeforeEach(func() {
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)

		// create db by testcontainers
		var err error
		container, err = testhelpers.NewPostgresContainer(ctx)
		Expect(err).ToNot(HaveOccurred())

		// Run migrations
		err = container.RunMigrations(db.MigrationsFS, db.MigrationsDir)
		Expect(err).ToNot(HaveOccurred())

		// Create repository with real DB connection
		repository := &repo.ResourcesRepository{
			CommonRepository: commonrepo.CommonRepository{
				Db: container.Pool,
			},
		}

		// Create server with real repository
		server := &api.ResourceServer{
			Repo: repository,
			Info: generated.OCloudInfo{
				OCloudId:      uuid.New(),
				GlobalCloudId: uuid.New(),
				Name:          "test-cloud",
				Description:   "Test O-Cloud for integration tests",
				ServiceUri:    "https://test.example.com",
			},
		}

		// Create strict handler (same as production)
		strictHandler := generated.NewStrictHandlerWithOptions(server, nil,
			generated.StrictHTTPServerOptions{
				RequestErrorHandlerFunc:  middleware.GetOranReqErrFunc(),
				ResponseErrorHandlerFunc: middleware.GetOranRespErrFunc(),
			},
		)

		// Load swagger spec for middleware
		swagger, err := generated.GetSwagger()
		Expect(err).ToNot(HaveOccurred())

		// Create logger for middleware (discard all logs in tests)
		logger := slog.New(slog.DiscardHandler)

		// Create filter adapter for response filtering middleware
		filterAdapter, err := middleware.NewFilterAdapterFromSwagger(logger, swagger)
		Expect(err).ToNot(HaveOccurred())

		// Create HTTP handler with middleware chain (excluding auth)
		opt := generated.StdHTTPServerOptions{
			BaseRouter: http.NewServeMux(),
			Middlewares: []generated.MiddlewareFunc{
				middleware.OpenAPIValidation(swagger),
				middleware.ResponseFilter(filterAdapter),
				deprecation.HeadersMiddleware(docsBaseURL), // RFC 8594 deprecation headers
				middleware.LogDuration(),
			},
			ErrorHandlerFunc: middleware.GetOranReqErrFunc(),
		}
		httpHandler := generated.HandlerWithOptions(strictHandler, opt)

		// Start test HTTP server
		testServer = httptest.NewServer(httpHandler)
	})

	// Terminate container and server
	AfterEach(func() {
		if testServer != nil {
			testServer.Close()
		}
		if container != nil {
			_ = container.Terminate(ctx)
		}
		cancel()
	})

	Describe("GET /resourcePools", func() {
		When("database has resource pools", func() {
			It("returns 200 with the pools and RFC 8594 deprecation headers", func() {
				// Insert required data_source first (foreign key)
				dataSourceID := uuid.New()
				_, err := container.Pool.Exec(ctx, `
					INSERT INTO data_source (data_source_id, name, generation_id)
					VALUES ($1, 'test-source', 1)
				`, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert a resource pool
				poolID := uuid.New()
				oCloudID := uuid.New()
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO resource_pool (resource_pool_id, global_location_id, name, description, o_cloud_id, data_source_id, generation_id, external_id)
					VALUES ($1, $2, 'test-pool', 'Test pool description', $3, $4, 1, 'ext-123')
				`, poolID, uuid.New(), oCloudID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/resourcePools")
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/json"))

				// Verify RFC 8594 deprecation headers (ResourcePool has deprecated fields)
				Expect(resp.Header.Get("Deprecation")).To(Equal("true"))
				Expect(resp.Header.Get("Sunset")).ToNot(BeEmpty())
				Expect(resp.Header.Get("Link")).To(ContainSubstring("rel=\"deprecation\""))
				Expect(resp.Header.Get("Link")).To(ContainSubstring("/docs/deprecations/resource-pool-fields.md"))

				// Parse response body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var pools []generated.ResourcePool
				err = json.Unmarshal(body, &pools)
				Expect(err).ToNot(HaveOccurred())

				Expect(pools).To(HaveLen(1))
				Expect(pools[0].ResourcePoolId).To(Equal(poolID))
				Expect(pools[0].Name).To(Equal("test-pool"))
				Expect(pools[0].Description).To(HaveValue(Equal("Test pool description")))
			})
		})

		When("database is empty", func() {
			It("returns 200 with empty array", func() {
				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/resourcePools")
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var pools []generated.ResourcePool
				err = json.Unmarshal(body, &pools)
				Expect(err).ToNot(HaveOccurred())

				Expect(pools).To(BeEmpty())
			})
		})

	})

	Describe("GET /resourcePools/{resourcePoolId}", func() {
		When("resource pool exists", func() {
			It("returns 200 with the pool data", func() {
				// Insert required data_source first
				dataSourceID := uuid.New()
				_, err := container.Pool.Exec(ctx, `
					INSERT INTO data_source (data_source_id, name, generation_id)
					VALUES ($1, 'test-source', 1)
				`, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert a resource pool
				poolID := uuid.New()
				oCloudID := uuid.New()
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO resource_pool (resource_pool_id, global_location_id, name, description, o_cloud_id, data_source_id, generation_id, external_id)
					VALUES ($1, $2, 'specific-pool', 'Specific pool description', $3, $4, 1, 'ext-456')
				`, poolID, uuid.New(), oCloudID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/resourcePools/" + poolID.String())
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				// Parse response body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var pool generated.ResourcePool
				err = json.Unmarshal(body, &pool)
				Expect(err).ToNot(HaveOccurred())

				Expect(pool.ResourcePoolId).To(Equal(poolID))
				Expect(pool.Name).To(Equal("specific-pool"))
			})
		})

		When("resource pool does not exist", func() {
			It("returns 404 with ProblemDetails", func() {
				nonExistentID := uuid.New()

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/resourcePools/" + nonExistentID.String())
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))

				// Verify ProblemDetails body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var problem commonapi.ProblemDetails
				err = json.Unmarshal(body, &problem)
				Expect(err).ToNot(HaveOccurred())

				Expect(problem.Status).To(Equal(http.StatusNotFound))
				Expect(problem.Detail).To(ContainSubstring("not found"))
				Expect(problem.AdditionalAttributes).ToNot(BeNil())
				Expect((*problem.AdditionalAttributes)["resourcePoolId"]).To(Equal(nonExistentID.String()))
			})
		})

		When("resourcePoolId is invalid UUID", func() {
			It("returns 400 with ProblemDetails", func() {
				// Make HTTP request with invalid UUID
				resp, err := http.Get(testServer.URL + basePath + "/resourcePools/not-a-uuid")
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				Expect(resp.Header.Get("Content-Type")).To(HavePrefix("application/problem+json"))

				// Verify ProblemDetails body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var problem commonapi.ProblemDetails
				err = json.Unmarshal(body, &problem)
				Expect(err).ToNot(HaveOccurred())

				Expect(problem.Status).To(Equal(http.StatusBadRequest))
				Expect(problem.Detail).To(ContainSubstring("resourcePoolId"))
			})
		})
	})

	Describe("GET /subscriptions", func() {
		When("subscriptions exist", func() {
			It("returns 200 with the subscriptions", func() {
				// Insert a subscription
				subID := uuid.New()
				consumerSubID := uuid.New()
				_, err := container.Pool.Exec(ctx, `
					INSERT INTO subscription (subscription_id, consumer_subscription_id, callback)
					VALUES ($1, $2, 'https://callback.example.com')
				`, subID, consumerSubID)
				Expect(err).ToNot(HaveOccurred())

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/subscriptions")
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				// Parse response body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var subscriptions []generated.Subscription
				err = json.Unmarshal(body, &subscriptions)
				Expect(err).ToNot(HaveOccurred())

				Expect(subscriptions).To(HaveLen(1))
				Expect(subscriptions[0].SubscriptionId).To(HaveValue(Equal(subID)))
				Expect(subscriptions[0].Callback).To(Equal("https://callback.example.com"))
			})
		})
	})

	//
	// V11 Locations and Sites Feature Tests: O-RAN.WG6.TS.O2IMS-INTERFACE-R005-v11.00
	//
	Describe("GET /locations", func() {
		When("locations exist", func() {
			It("returns 200 with locations and NO deprecation headers (new V11 endpoint)", func() {
				// Insert required data_source first (foreign key)
				dataSourceID := uuid.New()
				_, err := container.Pool.Exec(ctx, `
					INSERT INTO data_source (data_source_id, name, generation_id)
					VALUES ($1, 'test-source', 1)
				`, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert a location with address (satisfies constraint)
				globalLocationID := "location-east-1"
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO location (global_location_id, name, description, address, data_source_id, generation_id)
					VALUES ($1, 'East Data Center', 'Primary east coast data center', '123 Main St, NYC', $2, 1)
				`, globalLocationID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/locations")
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/json"))

				// Verify NO deprecation headers (locations is a new V11 endpoint, not deprecated)
				Expect(resp.Header.Get("Deprecation")).To(BeEmpty())
				Expect(resp.Header.Get("Sunset")).To(BeEmpty())

				// Parse response body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var locations []generated.LocationInfo
				err = json.Unmarshal(body, &locations)
				Expect(err).ToNot(HaveOccurred())

				Expect(locations).To(HaveLen(1))
				Expect(locations[0].GlobalLocationId).To(Equal(globalLocationID))
				Expect(locations[0].Name).To(Equal("East Data Center"))
				Expect(locations[0].Description).To(Equal("Primary east coast data center"))
				Expect(locations[0].Address).To(HaveValue(Equal("123 Main St, NYC")))
			})
		})

		When("location has coordinate (GeoJSON Point)", func() {
			It("returns 200 with coordinate data", func() {
				// Insert required data_source
				dataSourceID := uuid.New()
				_, err := container.Pool.Exec(ctx, `
					INSERT INTO data_source (data_source_id, name, generation_id)
					VALUES ($1, 'test-source', 1)
				`, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert a location with GeoJSON coordinate
				globalLocationID := "location-geo-1"
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO location (global_location_id, name, description, coordinate, data_source_id, generation_id)
					VALUES ($1, 'Geo Location', 'Location with coordinates', '{"type": "Point", "coordinates": [-77.0364, 38.8951]}', $2, 1)
				`, globalLocationID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/locations/" + globalLocationID)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				// Parse response body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var location generated.LocationInfo
				err = json.Unmarshal(body, &location)
				Expect(err).ToNot(HaveOccurred())

				Expect(location.GlobalLocationId).To(Equal(globalLocationID))
				Expect(location.Coordinate).ToNot(BeNil())
				Expect(location.Coordinate.Type).To(HaveValue(Equal(generated.Point)))
				Expect(*location.Coordinate.Coordinates).To(HaveLen(2))
			})
		})

		When("location has civic address (RFC 4776)", func() {
			It("returns 200 with civic address data", func() {
				// Insert required data_source
				dataSourceID := uuid.New()
				_, err := container.Pool.Exec(ctx, `
					INSERT INTO data_source (data_source_id, name, generation_id)
					VALUES ($1, 'test-source', 1)
				`, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert a location with civic address per RFC 4776
				globalLocationID := "location-civic-1"
				civicAddress := `[{"caType": 1, "caValue": "US"}, {"caType": 3, "caValue": "Virginia"}, {"caType": 6, "caValue": "Ashburn"}]`
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO location (global_location_id, name, description, civic_address, data_source_id, generation_id)
					VALUES ($1, 'Civic Location', 'Location with civic address', $2, $3, 1)
				`, globalLocationID, civicAddress, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/locations/" + globalLocationID)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				// Parse response body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var location generated.LocationInfo
				err = json.Unmarshal(body, &location)
				Expect(err).ToNot(HaveOccurred())

				Expect(location.GlobalLocationId).To(Equal(globalLocationID))
				Expect(location.CivicAddress).ToNot(BeNil())
				Expect(*location.CivicAddress).To(HaveLen(3))
			})
		})

		When("database is empty", func() {
			It("returns 200 with empty array", func() {
				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/locations")
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var locations []generated.LocationInfo
				err = json.Unmarshal(body, &locations)
				Expect(err).ToNot(HaveOccurred())

				Expect(locations).To(BeEmpty())
			})
		})
	})

	Describe("GET /locations/{globalLocationId}", func() {
		When("location exists", func() {
			It("returns 200 with the location data", func() {
				// Insert required data_source
				dataSourceID := uuid.New()
				_, err := container.Pool.Exec(ctx, `
					INSERT INTO data_source (data_source_id, name, generation_id)
					VALUES ($1, 'test-source', 1)
				`, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert a location
				globalLocationID := "specific-location-123"
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO location (global_location_id, name, description, address, data_source_id, generation_id)
					VALUES ($1, 'Specific Location', 'A specific location for testing', '456 Test Ave', $2, 1)
				`, globalLocationID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/locations/" + globalLocationID)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				// Parse response body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var location generated.LocationInfo
				err = json.Unmarshal(body, &location)
				Expect(err).ToNot(HaveOccurred())

				Expect(location.GlobalLocationId).To(Equal(globalLocationID))
				Expect(location.Name).To(Equal("Specific Location"))
				Expect(location.Description).To(Equal("A specific location for testing"))
			})
		})

		When("location does not exist", func() {
			It("returns 404 with ProblemDetails", func() {
				nonExistentID := "non-existent-location"

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/locations/" + nonExistentID)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))

				// Verify ProblemDetails body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var problem commonapi.ProblemDetails
				err = json.Unmarshal(body, &problem)
				Expect(err).ToNot(HaveOccurred())

				Expect(problem.Status).To(Equal(http.StatusNotFound))
				Expect(problem.Detail).To(ContainSubstring("not found"))
				Expect(problem.AdditionalAttributes).ToNot(BeNil())
				Expect((*problem.AdditionalAttributes)["globalLocationId"]).To(Equal(nonExistentID))
			})
		})

		When("location has associated O-Cloud sites", func() {
			It("returns 200 with oCloudSiteIds populated", func() {
				// Insert required data_source
				dataSourceID := uuid.New()
				_, err := container.Pool.Exec(ctx, `
					INSERT INTO data_source (data_source_id, name, generation_id)
					VALUES ($1, 'test-source', 1)
				`, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert a location
				globalLocationID := "location-with-sites"
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO location (global_location_id, name, description, address, data_source_id, generation_id)
					VALUES ($1, 'Location With Sites', 'Has associated sites', '789 Site Blvd', $2, 1)
				`, globalLocationID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert O-Cloud sites at this location
				siteID1 := uuid.New()
				siteID2 := uuid.New()
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO o_cloud_site (o_cloud_site_id, global_location_id, name, description, data_source_id, generation_id)
					VALUES ($1, $2, 'Site 1', 'First site', $3, 1),
					       ($4, $2, 'Site 2', 'Second site', $3, 1)
				`, siteID1, globalLocationID, dataSourceID, siteID2)
				Expect(err).ToNot(HaveOccurred())

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/locations/" + globalLocationID)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				// Parse response body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var location generated.LocationInfo
				err = json.Unmarshal(body, &location)
				Expect(err).ToNot(HaveOccurred())

				Expect(location.OCloudSiteIds).ToNot(BeNil())
				Expect(*location.OCloudSiteIds).To(HaveLen(2))
				Expect(*location.OCloudSiteIds).To(ContainElements(siteID1, siteID2))
			})
		})
	})

	Describe("GET /oCloudSites", func() {
		When("O-Cloud sites exist", func() {
			It("returns 200 with the sites", func() {
				// Insert required data_source
				dataSourceID := uuid.New()
				_, err := container.Pool.Exec(ctx, `
					INSERT INTO data_source (data_source_id, name, generation_id)
					VALUES ($1, 'test-source', 1)
				`, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert a location first (foreign key)
				globalLocationID := "site-location-1"
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO location (global_location_id, name, description, address, data_source_id, generation_id)
					VALUES ($1, 'Site Location', 'Location for sites', '123 Site St', $2, 1)
				`, globalLocationID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert an O-Cloud site
				siteID := uuid.New()
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO o_cloud_site (o_cloud_site_id, global_location_id, name, description, data_source_id, generation_id)
					VALUES ($1, $2, 'Test Site', 'A test O-Cloud site', $3, 1)
				`, siteID, globalLocationID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/oCloudSites")
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/json"))

				// Parse response body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var sites []generated.OCloudSiteInfo
				err = json.Unmarshal(body, &sites)
				Expect(err).ToNot(HaveOccurred())

				Expect(sites).To(HaveLen(1))
				Expect(sites[0].OCloudSiteId).To(Equal(siteID))
				Expect(sites[0].GlobalLocationId).To(Equal(globalLocationID))
				Expect(sites[0].Name).To(Equal("Test Site"))
				Expect(sites[0].Description).To(Equal("A test O-Cloud site"))
			})
		})

		When("database is empty", func() {
			It("returns 200 with empty array", func() {
				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/oCloudSites")
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var sites []generated.OCloudSiteInfo
				err = json.Unmarshal(body, &sites)
				Expect(err).ToNot(HaveOccurred())

				Expect(sites).To(BeEmpty())
			})
		})
	})

	Describe("GET /oCloudSites/{oCloudSiteId}", func() {
		When("O-Cloud site exists", func() {
			It("returns 200 with the site data", func() {
				// Insert required data_source
				dataSourceID := uuid.New()
				_, err := container.Pool.Exec(ctx, `
					INSERT INTO data_source (data_source_id, name, generation_id)
					VALUES ($1, 'test-source', 1)
				`, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert a location first
				globalLocationID := "site-location-specific"
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO location (global_location_id, name, description, address, data_source_id, generation_id)
					VALUES ($1, 'Site Location', 'Location for site', '456 Site Ave', $2, 1)
				`, globalLocationID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert an O-Cloud site
				siteID := uuid.New()
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO o_cloud_site (o_cloud_site_id, global_location_id, name, description, data_source_id, generation_id)
					VALUES ($1, $2, 'Specific Site', 'A specific O-Cloud site', $3, 1)
				`, siteID, globalLocationID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/oCloudSites/" + siteID.String())
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				// Parse response body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var site generated.OCloudSiteInfo
				err = json.Unmarshal(body, &site)
				Expect(err).ToNot(HaveOccurred())

				Expect(site.OCloudSiteId).To(Equal(siteID))
				Expect(site.GlobalLocationId).To(Equal(globalLocationID))
				Expect(site.Name).To(Equal("Specific Site"))
			})
		})

		When("O-Cloud site does not exist", func() {
			It("returns 404 with ProblemDetails", func() {
				nonExistentID := uuid.New()

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/oCloudSites/" + nonExistentID.String())
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))

				// Verify ProblemDetails body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var problem commonapi.ProblemDetails
				err = json.Unmarshal(body, &problem)
				Expect(err).ToNot(HaveOccurred())

				Expect(problem.Status).To(Equal(http.StatusNotFound))
				Expect(problem.Detail).To(ContainSubstring("not found"))
				Expect(problem.AdditionalAttributes).ToNot(BeNil())
				Expect((*problem.AdditionalAttributes)["oCloudSiteId"]).To(Equal(nonExistentID.String()))
			})
		})

		When("oCloudSiteId is invalid UUID", func() {
			It("returns 400 with ProblemDetails", func() {
				// Make HTTP request with invalid UUID
				resp, err := http.Get(testServer.URL + basePath + "/oCloudSites/not-a-uuid")
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				Expect(resp.Header.Get("Content-Type")).To(HavePrefix("application/problem+json"))

				// Verify ProblemDetails body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var problem commonapi.ProblemDetails
				err = json.Unmarshal(body, &problem)
				Expect(err).ToNot(HaveOccurred())

				Expect(problem.Status).To(Equal(http.StatusBadRequest))
				Expect(problem.Detail).To(ContainSubstring("oCloudSiteId"))
			})
		})

		When("O-Cloud site has associated resource pools", func() {
			It("returns 200 with resourcePools populated", func() {
				// Insert required data_source
				dataSourceID := uuid.New()
				_, err := container.Pool.Exec(ctx, `
					INSERT INTO data_source (data_source_id, name, generation_id)
					VALUES ($1, 'test-source', 1)
				`, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert a location
				globalLocationID := "site-with-pools-location"
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO location (global_location_id, name, description, address, data_source_id, generation_id)
					VALUES ($1, 'Pools Location', 'Location with pools', '789 Pool Rd', $2, 1)
				`, globalLocationID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert an O-Cloud site
				siteID := uuid.New()
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO o_cloud_site (o_cloud_site_id, global_location_id, name, description, data_source_id, generation_id)
					VALUES ($1, $2, 'Site With Pools', 'Has resource pools', $3, 1)
				`, siteID, globalLocationID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert resource pools linked to this site via o_cloud_site_id
				poolID1 := uuid.New()
				poolID2 := uuid.New()
				oCloudID := uuid.New()
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO resource_pool (resource_pool_id, global_location_id, name, description, o_cloud_id, o_cloud_site_id, data_source_id, generation_id, external_id)
					VALUES ($1, $2, 'Pool 1', 'First pool', $3, $4, $5, 1, 'ext-pool-1'),
					       ($6, $2, 'Pool 2', 'Second pool', $3, $4, $5, 1, 'ext-pool-2')
				`, poolID1, uuid.New(), oCloudID, siteID, dataSourceID, poolID2)
				Expect(err).ToNot(HaveOccurred())

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/oCloudSites/" + siteID.String())
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				// Parse response body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var site generated.OCloudSiteInfo
				err = json.Unmarshal(body, &site)
				Expect(err).ToNot(HaveOccurred())

				Expect(site.ResourcePools).To(HaveLen(2))
				Expect(site.ResourcePools).To(ContainElements(poolID1, poolID2))
			})
		})
	})

	//
	// V11 ResourcePool with OCloudSite relationship tests
	//
	Describe("GET /resourcePools with V11 oCloudSiteId", func() {
		When("resource pool has oCloudSiteId set", func() {
			It("returns 200 with oCloudSiteId in response", func() {
				// Insert required data_source
				dataSourceID := uuid.New()
				_, err := container.Pool.Exec(ctx, `
					INSERT INTO data_source (data_source_id, name, generation_id)
					VALUES ($1, 'test-source', 1)
				`, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert a location
				globalLocationID := "pool-site-location"
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO location (global_location_id, name, description, address, data_source_id, generation_id)
					VALUES ($1, 'Pool Site Location', 'Location for pool site', '101 Pool Lane', $2, 1)
				`, globalLocationID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert an O-Cloud site
				siteID := uuid.New()
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO o_cloud_site (o_cloud_site_id, global_location_id, name, description, data_source_id, generation_id)
					VALUES ($1, $2, 'Pool Site', 'Site for pool', $3, 1)
				`, siteID, globalLocationID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert a resource pool with o_cloud_site_id
				poolID := uuid.New()
				oCloudID := uuid.New()
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO resource_pool (resource_pool_id, global_location_id, name, description, o_cloud_id, o_cloud_site_id, data_source_id, generation_id, external_id)
					VALUES ($1, $2, 'V11 Pool', 'Pool with site reference', $3, $4, $5, 1, 'ext-v11-pool')
				`, poolID, uuid.New(), oCloudID, siteID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/resourcePools/" + poolID.String())
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				// Parse response body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var pool generated.ResourcePool
				err = json.Unmarshal(body, &pool)
				Expect(err).ToNot(HaveOccurred())

				Expect(pool.ResourcePoolId).To(Equal(poolID))
				Expect(pool.OCloudSiteId).To(HaveValue(Equal(siteID)))
			})
		})

		When("resource pool has oCloudSiteId as NULL (legacy data)", func() {
			It("returns 200 with oCloudSiteId as zero UUID", func() {
				// Insert required data_source
				dataSourceID := uuid.New()
				_, err := container.Pool.Exec(ctx, `
					INSERT INTO data_source (data_source_id, name, generation_id)
					VALUES ($1, 'test-source', 1)
				`, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Insert a resource pool WITHOUT o_cloud_site_id (legacy)
				poolID := uuid.New()
				oCloudID := uuid.New()
				_, err = container.Pool.Exec(ctx, `
					INSERT INTO resource_pool (resource_pool_id, global_location_id, name, description, o_cloud_id, data_source_id, generation_id, external_id)
					VALUES ($1, $2, 'Legacy Pool', 'Pool without site reference', $3, $4, 1, 'ext-legacy-pool')
				`, poolID, uuid.New(), oCloudID, dataSourceID)
				Expect(err).ToNot(HaveOccurred())

				// Make HTTP request
				resp, err := http.Get(testServer.URL + basePath + "/resourcePools/" + poolID.String())
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				// Verify HTTP response
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				// Parse response body
				body, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())

				var pool generated.ResourcePool
				err = json.Unmarshal(body, &pool)
				Expect(err).ToNot(HaveOccurred())

				Expect(pool.ResourcePoolId).To(Equal(poolID))
				// oCloudSiteId is required per OpenAPI spec, so it returns zero UUID for legacy data
				// Note: Per O-RAN spec, oCloudSiteId is mandatory; zero UUID indicates migration needed
				Expect(pool.OCloudSiteId).To(Equal(uuid.Nil))
			})
		})
	})
})
