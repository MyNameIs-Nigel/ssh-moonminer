package store_test

import (
	"context"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/store"
)

func TestRoundTrip(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/test.db"
	st, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	now := int64(1000)
	if err := st.TouchAccount(ctx, "SHA256:abc", "ssh-rsa AAA", now); err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"version":1}`)
	row, created, err := st.LoadOrCreateSave(ctx, "SHA256:abc", "default", now, func() ([]byte, int, error) {
		return payload, 1, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created || string(row.State) != string(payload) {
		t.Fatalf("create: created=%v state=%q", created, row.State)
	}
	payload2 := []byte(`{"version":1,"credits":999}`)
	if err := st.SaveState(ctx, "SHA256:abc", "default", payload2, 1, now+10); err != nil {
		t.Fatal(err)
	}
	row2, created2, err := st.LoadOrCreateSave(ctx, "SHA256:abc", "default", now, func() ([]byte, int, error) {
		t.Fatal("should not create")
		return nil, 0, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if created2 {
		t.Fatal("should not recreate")
	}
	if string(row2.State) != string(payload2) {
		t.Fatalf("load: %q", row2.State)
	}
}
