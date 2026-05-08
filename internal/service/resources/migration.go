/*
SPDX-FileCopyrightText: Red Hat

SPDX-License-Identifier: Apache-2.0
*/

package resources

import (
	"embed"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4/source/iofs"
	ctlrutils "github.com/openshift-kni/oran-o2ims/internal/controllers/utils"
	"github.com/openshift-kni/oran-o2ims/internal/service/common/db"
)

//go:embed db/migrations/*.sql
var MigrationsFS embed.FS

// MigrationsDir is the subdirectory within MigrationsFS containing the SQL files.
const MigrationsDir = "db/migrations"

// StartResourcesMigration initiates the migration process for the resource server database
func StartResourcesMigration() error {
	driver, err := iofs.New(MigrationsFS, MigrationsDir)
	if err != nil {
		return fmt.Errorf("failed to create migrations source: %w", err)
	}

	password, exists := os.LookupEnv(ctlrutils.ResourcesPasswordEnvName)
	if !exists {
		return fmt.Errorf("missing %s environment variable", ctlrutils.ResourcesPasswordEnvName)
	}

	err = db.StartMigration(db.GetPgConfig(username, password, database), driver)
	if err != nil {
		return fmt.Errorf("failed to start migrations: %w", err)
	}

	return nil
}
