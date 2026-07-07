package store

import (
	"context"
	"fmt"
)

var migrations = []string{
	`CREATE TABLE accounts (
		fingerprint TEXT PRIMARY KEY,
		public_key  TEXT NOT NULL,
		created_at  INTEGER NOT NULL,
		last_seen   INTEGER NOT NULL
	);
	CREATE TABLE saves (
		fingerprint TEXT NOT NULL REFERENCES accounts(fingerprint),
		slot        TEXT NOT NULL,
		state       BLOB NOT NULL,
		version     INTEGER NOT NULL,
		created_at  INTEGER NOT NULL,
		updated_at  INTEGER NOT NULL,
		PRIMARY KEY (fingerprint, slot)
	);`,
}

func (st *Store) migrate(ctx context.Context) error {
	var current int
	if err := st.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("store: read schema version: %w", err)
	}
	for v := current; v < len(migrations); v++ {
		tx, err := st.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, migrations[v]); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("store: migration %d: %w", v+1, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", v+1)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
