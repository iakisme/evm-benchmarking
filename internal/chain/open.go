// Package chain wraps BSC core packages to provide a stripped, consensus-bypassed
// execution environment for benchmark replay.
package chain

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	pebbledb "github.com/ethereum/go-ethereum/ethdb/pebble"
)

// DB is the tuple of low-level handles evmbench needs.
//
// Note: we do NOT construct a triedb here. Under path-based state scheme
// the triedb opens (and locks via FLOCK) the state-history freezer, and the
// runner needs to construct its own triedb on top of a CountingDB wrapper
// for byte-accounting. Two triedbs sharing the same on-disk freezer would
// race for the lock; so we keep this layer thin and let the runner own the
// triedb.
type DB struct {
	Path string
	Disk ethdb.Database // raw key-value + ancient store
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
	// evmbench measures EVM throughput, not DB cache sizing.
	const (
		cacheMB     = 1024
		fileHandles = 512
	)
	kv, err := pebbledb.New(chaindata, cacheMB, fileHandles, "evmbench/", false /*readonly*/)
	if err != nil {
		return nil, fmt.Errorf("pebble open: %w", err)
	}
	disk, err := rawdb.Open(kv, rawdb.OpenOptions{
		Ancient:          ancient,
		MetricsNamespace: "evmbench/",
		ReadOnly:         false,
	})
	if err != nil {
		return nil, fmt.Errorf("rawdb open: %w", err)
	}

	return &DB{Path: stateDir, Disk: disk}, nil
}

func (db *DB) Close() error {
	return db.Disk.Close()
}
