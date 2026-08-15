// Command restore-check is the durability drills' verification step
// (scripts/restore-drill, framework/04's MinIO acceptance rows): given a
// restored ssh-moonminer SQLite file, it runs SQLite's own integrity_check,
// then decodes every save's state blob — proving the restore is not merely a
// well-formed file but actually-readable pilot data — and prints the latest
// updated_at across all saves so the calling drill script can compute the
// observed RPO against its own recorded kill timestamp.
//
// Decoding matters more here than the integrity check does. A torn litestream
// restore can leave a file that passes integrity_check while carrying a
// truncated or stale JSON blob, and sim.DecodeState is the same function the
// game itself boots through — if it fails here, the pilot's save is gone even
// though the file looks fine.
//
// It never writes to the database: a read-only verification tool, safe to run
// against a live restore before deciding whether to serve it.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/store"
)

func main() {
	dbPath := flag.String("db", "", "path to the ssh-moonminer SQLite database to verify")
	flag.Parse()
	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "restore-check: -db is required")
		os.Exit(2)
	}

	if err := run(*dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "restore-check: FAIL: %v\n", err)
		os.Exit(1)
	}
}

func run(dbPath string) error {
	ctx := context.Background()

	// sim.DecodeState needs content to migrate a pre-fleet save forward, so
	// the drill verifies against the same embedded TOML the game ships with.
	c, err := content.Load(os.Getenv("MOONMINER_DATA_DIR"))
	if err != nil {
		return fmt.Errorf("load content: %w", err)
	}

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", dbPath, err)
	}
	defer st.Close()

	if err := st.IntegrityCheck(ctx); err != nil {
		return fmt.Errorf("integrity check: %w", err)
	}
	fmt.Println("integrity: ok")

	rows, err := st.AllSaves(ctx)
	if err != nil {
		return fmt.Errorf("list saves: %w", err)
	}

	var latest int64
	for _, row := range rows {
		if _, err := sim.DecodeState(row.State, c); err != nil {
			return fmt.Errorf("decode save %s/%s: %w", row.Fingerprint, row.Slot, err)
		}
		if row.UpdatedAt > latest {
			latest = row.UpdatedAt
		}
	}

	fmt.Printf("saves: %d decoded ok\n", len(rows))
	fmt.Printf("latest_write: %d\n", latest)
	return nil
}
