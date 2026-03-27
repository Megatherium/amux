// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package dolt

import (
	"context"
	"database/sql"
	"fmt"
)

// verifySchema checks that the database has the expected schema by querying
// the ready_issues view. Returns an actionable error if the schema is missing
// or incompatible.
func verifySchema(ctx context.Context, db *sql.DB) error {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ready_issues LIMIT 1").Scan(&count)
	if err != nil {
		return fmt.Errorf(
			"schema verification failed: unable to query ready_issues view: %w; "+
				"the database may be missing the beads schema or may be corrupted; "+
				"try running 'bd init' to initialize or repair the database schema",
			err,
		)
	}

	return nil
}
