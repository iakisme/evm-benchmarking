//go:build integration

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const integrationFixtureDir = "../../testdata/integration/chapel-50blocks"

func TestReplayIntegrationChapel(t *testing.T) {
	if _, err := os.Stat(integrationFixtureDir); err != nil {
		t.Skipf("integration fixture missing at %s: %v", integrationFixtureDir, err)
	}

	out := t.TempDir()
	err := runReplay(integrationFixtureDir, out, 0, 0, true, filepath.Join(out, "work"))
	if err != nil {
		t.Fatalf("runReplay: %v", err)
	}

	resultPath := filepath.Join(out, "result.json")
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parse result: %v", err)
	}

	if got["schema_version"] != "1" {
		t.Errorf("schema_version = %v", got["schema_version"])
	}
	metrics, ok := got["metrics"].(map[string]any)
	if !ok {
		t.Fatalf("metrics not an object")
	}
	if mgasps, _ := metrics["mgasps"].(float64); mgasps <= 0 {
		t.Errorf("mgasps = %v", mgasps)
	}

	for _, name := range []string{"blocks.csv", "proc_samples.csv"} {
		st, err := os.Stat(filepath.Join(out, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
		} else if st.Size() == 0 {
			t.Errorf("%s: empty", name)
		}
	}
}
