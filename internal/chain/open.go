// Package chain wraps BSC core packages to provide a stripped, consensus-bypassed
// execution environment for benchmark replay.
package chain

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
	pebbledb "github.com/ethereum/go-ethereum/ethdb/pebble"
	"github.com/ethereum/go-ethereum/triedb"
)

// DB is the tuple of low-level handles bscbench needs.
type DB struct {
	Path  string
	Disk  ethdb.Database // raw key-value + ancient store
	State state.Database // wraps Disk for state.New()
}

// Open opens the state database at <stateDir>. The directory layout follows
// BSC/geth conventions: chaindata/ for kv, ancient/ for the freezer.
func Open(stateDir string) (*DB, error) {
	if st, err := os.Stat(stateDir); err != nil {
		return nil, fmt.Errorf("state dir: %w", err)
	} else if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", stateDir)
	}

	chaindata := filepath.Join(stateDir, "chaindata")
	ancient := filepath.Join(stateDir, "ancient")

	// Open the underlying pebble KV first; rawdb.Open then wraps it together
	// with the on-disk freezer. Cache/handles are intentionally small —
	// bscbench measures EVM throughput, not DB cache sizing.
	const (
		cacheMB     = 1024
		fileHandles = 512
	)
	kv, err := pebbledb.New(chaindata, cacheMB, fileHandles, "bscbench/", false /*readonly*/)
	if err != nil {
		return nil, fmt.Errorf("pebble open: %w", err)
	}
	disk, err := rawdb.Open(kv, rawdb.OpenOptions{
		Ancient:          ancient,
		MetricsNamespace: "bscbench/",
		ReadOnly:         false,
	})
	if err != nil {
		return nil, fmt.Errorf("rawdb open: %w", err)
	}

	tdb := triedb.NewDatabase(disk, nil)
	stateCache := state.NewDatabase(tdb, nil)
	return &DB{Path: stateDir, Disk: disk, State: stateCache}, nil
}

func (db *DB) Close() error {
	return db.Disk.Close()
}
