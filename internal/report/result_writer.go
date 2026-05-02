package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteResult atomically writes r as pretty-printed JSON to path.
func WriteResult(r *Result, path string) error {
	if r.SchemaVersion == "" {
		r.SchemaVersion = SchemaVersion
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".result-*.json")
	if err != nil {
		return fmt.Errorf("tempfile: %w", err)
	}
	defer os.Remove(tmp.Name())

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
