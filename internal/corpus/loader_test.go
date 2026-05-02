package corpus

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/rlp"
)

func TestLoaderRejectsMissingFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(dir); err == nil {
		t.Fatal("expected error for empty dir")
	}
}

func TestLoaderHappyPath(t *testing.T) {
	dir := t.TempDir()

	manifestJSON, err := os.ReadFile(filepath.Join("testdata", "manifest_valid.json"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestFileName), manifestJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	// blocks.rlp: count must equal manifest.BlockCount, but Open does not
	// itself stream-decode (block_iter does); Open only checks file existence
	// and size > 0.
	var buf bytes.Buffer
	for i := 0; i < 1; i++ {
		_ = rlp.Encode(&buf, []byte{0x01})
	}
	if err := os.WriteFile(filepath.Join(dir, BlocksFileName), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(dir, "state"), 0o755); err != nil {
		t.Fatal(err)
	}

	c, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer c.Close()

	if c.Manifest().ChainID != 56 {
		t.Errorf("manifest not loaded")
	}
	if c.StateDir() != filepath.Join(dir, "state") {
		t.Errorf("state dir = %q", c.StateDir())
	}
	if c.BlocksPath() != filepath.Join(dir, BlocksFileName) {
		t.Errorf("blocks path = %q", c.BlocksPath())
	}
}
