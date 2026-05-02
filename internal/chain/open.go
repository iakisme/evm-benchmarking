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

	disk, err := rawdb.Open(rawdb.OpenOptions{
		Type:              "pebble",
		Directory:         chaindata,
		AncientsDirectory: ancient,
		Namespace:         "bscbench/",
		Cache:             1024, // MB; small, since we measure VM, not DB cache size effects
		Handles:           512,
		ReadOnly:          false, // we mutate via state Commit; bscbench's runner copies state into a workdir first
	})
	if err != nil {
		return nil, fmt.Errorf("rawdb open: %w", err)
	}

	stateCache := state.NewDatabaseWithConfig(disk, nil)
	return &DB{Path: stateDir, Disk: disk, State: stateCache}, nil
}

func (db *DB) Close() error {
	return db.Disk.Close()
}
