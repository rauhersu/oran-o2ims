/*
SPDX-FileCopyrightText: Red Hat

SPDX-License-Identifier: Apache-2.0
*/

package db_test

import (
	"context"
	"embed"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift-kni/oran-o2ims/internal/service/resources/db/testhelpers"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrateConnStr converts a postgres:// connection string to pgx5:// for golang-migrate.
func migrateConnStr(connStr string) string {
	return strings.Replace(connStr, "postgres://", "pgx5://", 1)
}

var _ = Describe("Database Migrations", Label("integration"), func() {
	var (
		ctx       context.Context
		cancel    context.CancelFunc
		container *testhelpers.PostgresContainer
	)

	// start a PostgreSQL container
	BeforeEach(func() {
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)

		var err error
		container, err = testhelpers.NewPostgresContainer(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	// stop PostgreSQL container
	AfterEach(func() {
		if container != nil {
			_ = container.Terminate(ctx)
		}
		cancel()
	})

	Describe("Applying migrations", func() {
		var m *migrate.Migrate

		BeforeEach(func() {
			// Create migration source from embedded files
			source, err := iofs.New(migrationsFS, "migrations")
			Expect(err).ToNot(HaveOccurred())

			// Create migrate instance with pgx5 driver
			m, err = migrate.NewWithSourceInstance("iofs", source, migrateConnStr(container.ConnStr))
			Expect(err).ToNot(HaveOccurred())

			// Run migrations
			err = m.Up()
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			if m != nil {
				m.Close()
			}
		})

		It("should track migration version correctly", func() {
			// Verify migration tracking table exists
			exists, err := container.TableExists(ctx, "schema_migrations")
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue(), "schema_migrations table should exist")

			// Verify migration version
			version, dirty, err := container.GetMigrationVersion(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(dirty).To(BeFalse(), "migration should not be in dirty state")
			Expect(version).To(Equal(2), "migration version should be 2 (latest)")
		})
	})

	Describe("Migration idempotency", func() {
		It("should succeed when running migrations twice", func() {
			// Create migration source
			source, err := iofs.New(migrationsFS, "migrations")
			Expect(err).ToNot(HaveOccurred())

			// Create migrate instance
			m, err := migrate.NewWithSourceInstance("iofs", source, migrateConnStr(container.ConnStr))
			Expect(err).ToNot(HaveOccurred())
			defer m.Close()

			// Run migrations first time
			err = m.Up()
			Expect(err).ToNot(HaveOccurred())

			// Run migrations second time - should return ErrNoChange
			err = m.Up()
			Expect(err).To(Equal(migrate.ErrNoChange), "running migrations twice should return ErrNoChange")

			// Verify final state is still correct
			version, dirty, err := container.GetMigrationVersion(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(dirty).To(BeFalse())
			Expect(version).To(Equal(2))
		})
	})

	Describe("Verifying schema", func() {
		var m *migrate.Migrate

		BeforeEach(func() {
			// Apply migrations before schema tests
			source, err := iofs.New(migrationsFS, "migrations")
			Expect(err).ToNot(HaveOccurred())

			m, err = migrate.NewWithSourceInstance("iofs", source, migrateConnStr(container.ConnStr))
			Expect(err).ToNot(HaveOccurred())

			err = m.Up()
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			if m != nil {
				m.Close()
			}
		})

		It("should have expected tables and columns", func() {
			// 000001_baseline: core inventory tables
			baselineTables := []string{
				"data_source",
				"resource_type",
				"resource_pool",
				"resource",
				"resource_pool_member",
				"deployment_manager",
				"data_change_event",
				"subscription",
				"alarm_dictionary",
				"alarm_definition",
			}
			for _, table := range baselineTables {
				exists, err := container.TableExists(ctx, table)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue(), "table %s should exist after baseline migration", table)
			}

			// 000002_v11_locations_sites: location and site tables
			v11Tables := []string{
				"location",
				"o_cloud_site",
			}
			for _, table := range v11Tables {
				exists, err := container.TableExists(ctx, table)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue(), "table %s should exist after v11 migration", table)
			}

			// 000002_v11_locations_sites: new column in resource_pool for site reference
			exists, err := container.ColumnExists(ctx, "resource_pool", "o_cloud_site_id")
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue(), "o_cloud_site_id column should exist in resource_pool")
		})

		It("should enforce location address constraint", func() {
			// First insert a data_source (required foreign key)
			_, err := container.Pool.Exec(ctx, `
				INSERT INTO data_source (name, generation_id) 
				VALUES ('test-source', 1)
			`)
			Expect(err).ToNot(HaveOccurred())

			// Get the data_source_id
			var dataSourceID string
			err = container.Pool.QueryRow(ctx, `
				SELECT data_source_id FROM data_source WHERE name = 'test-source'
			`).Scan(&dataSourceID)
			Expect(err).ToNot(HaveOccurred())

			// Try to insert a location without any address fields - should fail
			_, err = container.Pool.Exec(ctx, `
				INSERT INTO location (global_location_id, name, description, data_source_id)
				VALUES ('loc-1', 'Test Location', 'A test location', $1)
			`, dataSourceID)
			Expect(err).To(HaveOccurred(), "should fail without coordinate, civic_address, or address")
			Expect(err.Error()).To(ContainSubstring("chk_location_address_required"))

			// Insert with address field - should succeed
			_, err = container.Pool.Exec(ctx, `
				INSERT INTO location (global_location_id, name, description, address, data_source_id)
				VALUES ('loc-2', 'Test Location 2', 'A test location', '123 Main St', $1)
			`, dataSourceID)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
