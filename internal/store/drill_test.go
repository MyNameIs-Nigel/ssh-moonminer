package store_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/store"
)

// The store methods the durability drills depend on (scripts/restore-drill,
// via cmd/restore-check). A drill that can't distinguish a good restore from a
// bad one is worse than no drill, so these pin both directions.

func TestIntegrityCheckPassesOnAHealthyDatabase(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, t.TempDir()+"/ok.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.IntegrityCheck(ctx); err != nil {
		t.Fatalf("IntegrityCheck on a fresh database: %v", err)
	}
}

func TestAllSavesReturnsEveryRowWithItsBlob(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, t.TempDir()+"/saves.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if rows, err := st.AllSaves(ctx); err != nil || len(rows) != 0 {
		t.Fatalf("empty database: rows=%d err=%v", len(rows), err)
	}

	type want struct {
		fingerprint, slot string
		payload           string
		updatedAt         int64
	}
	wants := []want{
		{"SHA256:aaa", "default", `{"version":7,"credits":1}`, 1000},
		{"SHA256:aaa", "second", `{"version":7,"credits":2}`, 1200},
		{"SHA256:bbb", "default", `{"version":7,"credits":3}`, 1100},
	}
	for _, w := range wants {
		if err := st.TouchAccount(ctx, w.fingerprint, "ssh-ed25519 AAA", 900); err != nil {
			t.Fatal(err)
		}
		payload := []byte(w.payload)
		if _, _, err := st.LoadOrCreateSave(ctx, w.fingerprint, w.slot, 900, func() ([]byte, int, error) {
			return payload, 7, nil
		}); err != nil {
			t.Fatal(err)
		}
		if err := st.SaveState(ctx, w.fingerprint, w.slot, payload, 7, w.updatedAt); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := st.AllSaves(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(wants) {
		t.Fatalf("AllSaves returned %d rows, want %d", len(rows), len(wants))
	}

	// The drill's RPO number is the newest updated_at across every row, so a
	// row whose blob or timestamp went missing must show up here.
	var latest int64
	seen := map[string]string{}
	for _, r := range rows {
		if len(r.State) == 0 {
			t.Fatalf("row %s/%s came back with an empty blob", r.Fingerprint, r.Slot)
		}
		if r.StateVersion != 7 {
			t.Fatalf("row %s/%s lost its state version: %d", r.Fingerprint, r.Slot, r.StateVersion)
		}
		seen[r.Fingerprint+"/"+r.Slot] = string(r.State)
		if r.UpdatedAt > latest {
			latest = r.UpdatedAt
		}
	}
	for _, w := range wants {
		got, ok := seen[w.fingerprint+"/"+w.slot]
		if !ok {
			t.Fatalf("AllSaves omitted %s/%s", w.fingerprint, w.slot)
		}
		if got != w.payload {
			t.Fatalf("blob for %s/%s: got %q want %q", w.fingerprint, w.slot, got, w.payload)
		}
	}
	if latest != 1200 {
		t.Fatalf("newest updated_at across rows: got %d want 1200", latest)
	}
}

// TestIntegrityCheckFailsOnACorruptDatabase is the half that matters: a drill
// whose verification step cannot fail proves nothing. This corrupts a single
// b-tree page deep in the file, behind SQLite's back — the shape of a torn or
// truncated restore — leaving page 1 (the header and schema) intact so Open
// still succeeds and IntegrityCheck itself has to be the thing that catches it.
func TestIntegrityCheckFailsOnACorruptDatabase(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/corrupt.db"

	st, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.TouchAccount(ctx, "SHA256:aaa", "ssh-ed25519 AAA", 900); err != nil {
		t.Fatal(err)
	}
	// Enough rows to span many pages, so a single scribbled page in the middle
	// damages data without touching the schema on page 1.
	blob := make([]byte, 4096)
	for i := range blob {
		blob[i] = 'x'
	}
	for i := 0; i < 200; i++ {
		slot := fmt.Sprintf("slot%03d", i)
		if _, _, err := st.LoadOrCreateSave(ctx, "SHA256:aaa", slot, 900, func() ([]byte, int, error) {
			return blob, 7, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	f, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	const pageSize = 4096
	if info.Size() < 8*pageSize {
		t.Skipf("database too small (%d bytes) to corrupt a mid-file page", info.Size())
	}
	// Land on a page boundary well past page 1 and well before EOF.
	offset := (info.Size() / 2 / pageSize) * pageSize
	junk := make([]byte, pageSize)
	for i := range junk {
		junk[i] = 0xA5
	}
	if _, err := f.WriteAt(junk, offset); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	st2, err := store.Open(ctx, path)
	if err != nil {
		// Open refusing outright is an equally good failure signal for the
		// drill, but it means IntegrityCheck went unexercised — say so.
		t.Skipf("corruption caught at Open rather than IntegrityCheck: %v", err)
	}
	defer st2.Close()
	err = st2.IntegrityCheck(ctx)
	if err == nil {
		t.Fatal("IntegrityCheck passed on a corrupted database — the drill's verification step cannot fail")
	}
	t.Logf("corruption caught at IntegrityCheck: %v", err)
}
