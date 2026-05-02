package corpus

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifestValid(t *testing.T) {
	m, err := LoadManifest(filepath.Join("testdata", "manifest_valid.json"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.ChainID != 56 {
		t.Errorf("chain_id = %d", m.ChainID)
	}
	if m.FromBlock != 40000000 || m.ToBlock != 40010000 {
		t.Errorf("range %d..%d", m.FromBlock, m.ToBlock)
	}
	if m.BlockCount != 10000 {
		t.Errorf("count = %d", m.BlockCount)
	}
	if m.InputHash != "sha256:abcdef" {
		t.Errorf("hash = %q", m.InputHash)
	}
}

func TestLoadManifestRejectsMissingFields(t *testing.T) {
	_, err := LoadManifest(filepath.Join("testdata", "manifest_missing_field.json"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "from_block") {
		t.Errorf("expected error mentioning from_block, got: %v", err)
	}
}

func TestLoadManifestRejectsCountMismatch(t *testing.T) {
	m := &Manifest{
		SchemaVersion:           "1",
		ChainID:                 56,
		FromBlock:               1,
		ToBlock:                 1000,
		BlockCount:              998, // mismatch: to_block - from_block = 999, not 998
		ExpectedStateRootAtFrom: "0x00",
		Generator:               "x",
		GeneratedAt:             "2026-05-02T00:00:00Z",
		InputHash:               "sha256:x",
	}
	if err := m.Validate(); err == nil {
		t.Fatal("expected error")
	}
}
