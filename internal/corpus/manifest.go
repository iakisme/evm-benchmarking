// Package corpus loads and validates the evmbench input directory.
package corpus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const ManifestFileName = "manifest.json"

// Manifest describes the contents of a evmbench corpus directory.
type Manifest struct {
	SchemaVersion           string            `json:"schema_version"`
	ChainID                 uint64            `json:"chain_id"`
	FromBlock               uint64            `json:"from_block"`
	ToBlock                 uint64            `json:"to_block"`
	BlockCount              uint64            `json:"block_count"`
	ExpectedStateRootAtFrom string            `json:"expected_state_root_at_from"`
	ForkSchedule            map[string]uint64 `json:"fork_schedule"`
	Generator               string            `json:"generator"`
	GeneratedAt             string            `json:"generated_at"`
	InputHash               string            `json:"input_hash"`
	BSCVersionRecommended   string            `json:"bsc_version_recommended,omitempty"`
}

// LoadManifest reads and validates a manifest.json file at the given path.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("manifest invalid: %w", err)
	}
	return &m, nil
}

// Validate checks the manifest for required fields and consistency.
func (m *Manifest) Validate() error {
	if m.SchemaVersion != "1" {
		return fmt.Errorf("schema_version: want \"1\", got %q", m.SchemaVersion)
	}
	if m.ChainID == 0 {
		return errors.New("chain_id is required and non-zero")
	}
	if m.FromBlock == 0 && m.ToBlock == 0 {
		return errors.New("from_block / to_block required")
	}
	if m.ToBlock <= m.FromBlock {
		return fmt.Errorf("to_block (%d) must be > from_block (%d)", m.ToBlock, m.FromBlock)
	}
	if m.BlockCount != m.ToBlock-m.FromBlock {
		return fmt.Errorf("block_count (%d) != to_block - from_block (%d)",
			m.BlockCount, m.ToBlock-m.FromBlock)
	}
	if m.ExpectedStateRootAtFrom == "" {
		return errors.New("expected_state_root_at_from required")
	}
	if m.Generator == "" {
		return errors.New("generator required")
	}
	if m.GeneratedAt == "" {
		return errors.New("generated_at required")
	}
	if m.InputHash == "" {
		return errors.New("input_hash required")
	}
	return nil
}
