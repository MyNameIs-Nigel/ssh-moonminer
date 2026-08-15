package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	_ "modernc.org/sqlite"
)

var slotPattern = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// ErrInvalidKey is returned when fingerprint or slot fails validation.
var ErrInvalidKey = errors.New("store: invalid fingerprint or slot")

// Store is the open database handle.
type Store struct {
	db *sql.DB
}

// SaveRow is one persisted save.
type SaveRow struct {
	Fingerprint  string
	Slot         string
	CreatedAt    int64
	UpdatedAt    int64
	State        []byte
	StateVersion int
}

// Open opens the database at path.
func Open(ctx context.Context, path string) (*Store, error) {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("store: create db dir: %w", err)
		}
	}
	dsn := "file:" + url.PathEscape(path) +
		"?_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	st := &Store{db: db}
	if err := st.verifyWAL(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := st.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	_ = os.Chmod(path, 0o600)
	return st, nil
}

func (st *Store) Close() error { return st.db.Close() }

func (st *Store) verifyWAL(ctx context.Context) error {
	var mode string
	if err := st.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		return fmt.Errorf("store: read journal mode: %w", err)
	}
	if mode != "wal" {
		return fmt.Errorf("store: WAL mode required, got %q", mode)
	}
	return nil
}

func (st *Store) TouchAccount(ctx context.Context, fingerprint, publicKey string, now int64) error {
	if fingerprint == "" {
		return ErrInvalidKey
	}
	_, err := st.db.ExecContext(ctx, `
		INSERT INTO accounts (fingerprint, public_key, created_at, last_seen)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (fingerprint) DO UPDATE SET last_seen = excluded.last_seen`,
		fingerprint, publicKey, now, now)
	if err != nil {
		return fmt.Errorf("store: touch account: %w", err)
	}
	return nil
}

func (st *Store) LoadOrCreateSave(ctx context.Context, fingerprint, slot string, now int64, create func() ([]byte, int, error)) (SaveRow, bool, error) {
	if err := validateKeys(fingerprint, slot); err != nil {
		return SaveRow{}, false, err
	}
	row, err := st.loadSave(ctx, fingerprint, slot)
	if err == nil {
		return row, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return SaveRow{}, false, err
	}
	state, version, err := create()
	if err != nil {
		return SaveRow{}, false, err
	}
	res, err := st.db.ExecContext(ctx, `
		INSERT INTO saves (fingerprint, slot, state, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (fingerprint, slot) DO NOTHING`,
		fingerprint, slot, state, version, now, now)
	if err != nil {
		return SaveRow{}, false, fmt.Errorf("store: create save: %w", err)
	}
	inserted, _ := res.RowsAffected()
	row, err = st.loadSave(ctx, fingerprint, slot)
	if err != nil {
		return SaveRow{}, false, err
	}
	return row, inserted == 1, nil
}

func (st *Store) loadSave(ctx context.Context, fingerprint, slot string) (SaveRow, error) {
	row := SaveRow{Fingerprint: fingerprint, Slot: slot}
	err := st.db.QueryRowContext(ctx, `
		SELECT created_at, updated_at, state, version FROM saves
		WHERE fingerprint = ? AND slot = ?`, fingerprint, slot).
		Scan(&row.CreatedAt, &row.UpdatedAt, &row.State, &row.StateVersion)
	if err != nil {
		return SaveRow{}, err
	}
	return row, nil
}

func (st *Store) SaveState(ctx context.Context, fingerprint, slot string, state []byte, version int, now int64) error {
	if err := validateKeys(fingerprint, slot); err != nil {
		return err
	}
	res, err := st.db.ExecContext(ctx, `
		UPDATE saves SET state = ?, version = ?, updated_at = ?
		WHERE fingerprint = ? AND slot = ?`,
		state, version, now, fingerprint, slot)
	if err != nil {
		return fmt.Errorf("store: save state: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("store: no row for key/slot")
	}
	return nil
}

// IntegrityCheck runs SQLite's own `PRAGMA integrity_check` against the open
// database. Used by the durability drills (scripts/restore-drill) to prove a
// restored file is structurally sound before anything decides to serve it.
func (st *Store) IntegrityCheck(ctx context.Context) error {
	var result string
	if err := st.db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("store: integrity check: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("store: integrity check failed: %s", result)
	}
	return nil
}

// AllSaves returns every save row, blob included and unfiltered. This exists
// for the durability drills' decode-every-save pass: the store still never
// decodes a state blob itself, it only hands the caller every row so the
// caller can. A structurally valid SQLite file whose blobs no longer decode
// is a failed restore, and only decoding proves otherwise.
func (st *Store) AllSaves(ctx context.Context) ([]SaveRow, error) {
	rows, err := st.db.QueryContext(ctx, `
		SELECT fingerprint, slot, created_at, updated_at, state, version
		FROM saves`)
	if err != nil {
		return nil, fmt.Errorf("store: all saves: %w", err)
	}
	defer rows.Close()
	var out []SaveRow
	for rows.Next() {
		var r SaveRow
		if err := rows.Scan(&r.Fingerprint, &r.Slot, &r.CreatedAt, &r.UpdatedAt, &r.State, &r.StateVersion); err != nil {
			return nil, fmt.Errorf("store: all saves: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: all saves: %w", err)
	}
	return out, nil
}

func validateKeys(fingerprint, slot string) error {
	if fingerprint == "" || !slotPattern.MatchString(slot) {
		return ErrInvalidKey
	}
	return nil
}
