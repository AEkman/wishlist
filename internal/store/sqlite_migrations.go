package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/pressly/goose/v3"
)

const schema = `
	CREATE TABLE IF NOT EXISTS device_sessions (
        token_hash BLOB PRIMARY KEY,
        expires_at INTEGER NOT NULL
    );
	CREATE TABLE IF NOT EXISTS children (
		id TEXT NOT NULL PRIMARY KEY,
		name TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS wishlists (
		id TEXT NOT NULL PRIMARY KEY,
		child_id TEXT NOT NULL REFERENCES children(id),
		title TEXT NOT NULL,
		share_id TEXT NOT NULL UNIQUE
	);
	CREATE INDEX IF NOT EXISTS wishlists_child_id ON wishlists(child_id);
	CREATE TABLE IF NOT EXISTS wishes (
		id TEXT NOT NULL PRIMARY KEY,
		wishlist_id TEXT NOT NULL REFERENCES wishlists(id),
		title TEXT NOT NULL,
		description TEXT NOT NULL,
		media_type TEXT NOT NULL,
		image BLOB NOT NULL,
		position INTEGER NOT NULL,
		buyer TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS wishes_rank ON wishes(wishlist_id, position);
`

func initialize(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	baseline := goose.NewGoMigration(1, &goose.GoFunc{RunTx: migrateSQLiteBaseline}, nil)
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, nil, goose.WithGoMigrations(baseline, goose.NewGoMigration(2, &goose.GoFunc{RunTx: migrateGuidedRankings}, nil)))
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}

// The baseline upgrades pre-Goose SQLite files using their legacy version.
// Goose records completion and owns all subsequent migration history.
func migrateSQLiteBaseline(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, schema); err != nil {
		return err
	}
	var version int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version == 0 {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE wishes ADD COLUMN image_revision INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 1"); err != nil {
			return err
		}
	}
	if version < 2 {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE children ADD COLUMN number INTEGER"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE children SET number = rowid"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "CREATE UNIQUE INDEX children_number ON children(number)"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 2"); err != nil {
			return err
		}
	}
	if version < 3 {
		if err := migrateChildIDs(tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 3"); err != nil {
			return err
		}
	}
	if version < 4 {
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE wishlists ADD COLUMN family_visible INTEGER NOT NULL DEFAULT 0;
			CREATE TABLE family_sharing (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				share_id TEXT NOT NULL UNIQUE
			);
		`); err != nil {
			return err
		}
		shareID, err := newID()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO family_sharing (id, share_id) VALUES (1, ?)", shareID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 4"); err != nil {
			return err
		}
	}
	return nil
}

func migrateChildIDs(tx *sql.Tx) error {
	rows, err := tx.Query("SELECT id, name FROM children")
	if err != nil {
		return err
	}
	type mapping struct{ old, name, id string }
	var mappings []mapping
	seen := make(map[string]bool)
	for rows.Next() {
		var m mapping
		if err := rows.Scan(&m.old, &m.name); err != nil {
			rows.Close()
			return err
		}
		m.id = ChildID(m.name)
		mappings = append(mappings, m)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	reserved := make(map[string]bool, len(mappings))
	for _, m := range mappings {
		reserved[m.id] = true
	}
	for i := range mappings {
		id := mappings[i].id
		if seen[id] {
			id += "-extra"
			for reserved[id] || seen[id] {
				id += "-extra"
			}
		}
		seen[id] = true
		mappings[i].id = id
	}
	if _, err := tx.Exec("PRAGMA defer_foreign_keys = ON"); err != nil {
		return err
	}
	for _, m := range mappings {
		if _, err := tx.Exec("UPDATE wishlists SET child_id = ? WHERE child_id = ?", "migration:"+m.old, m.old); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE children SET id = ? WHERE id = ?", "migration:"+m.old, m.old); err != nil {
			return err
		}
	}
	for _, m := range mappings {
		if _, err := tx.Exec("UPDATE wishlists SET child_id = ? WHERE child_id = ?", m.id, "migration:"+m.old); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE children SET id = ? WHERE id = ?", m.id, "migration:"+m.old); err != nil {
			return err
		}
	}
	return nil
}

func migrateGuidedRankings(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE guided_rankings (wishlist_id TEXT PRIMARY KEY REFERENCES wishlists(id), state TEXT NOT NULL)`)
	return err
}
