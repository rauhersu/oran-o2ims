/*
SPDX-FileCopyrightText: Red Hat

SPDX-License-Identifier: Apache-2.0
*/

// Package db provides database utilities for the resources service.
package db

import "embed"

//go:embed migrations/*.sql
var MigrationsFS embed.FS

// MigrationsDir is the directory name within MigrationsFS containing the migration files.
const MigrationsDir = "migrations"
