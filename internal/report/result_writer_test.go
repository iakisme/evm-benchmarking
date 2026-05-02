package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteResult(t *testing.T) {
	r := makeFixtureResult()

	dir := t.TempDir()
	path := filepath.Join(dir, "result.json")
	if err := WriteResult(r, path); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	want, err := os.ReadFile("testdata/result_golden.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	// Compare as canonical JSON to ignore ordering.
	if !canonicalEqual(t, got, want) {
		t.Errorf("JSON differs from golden\n--- got\n%s\n--- want\n%s\n", got, want)
	}
}

func canonicalEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("unmarshal a: %v", err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("unmarshal b: %v", err)
	}
	ab, _ := json.Marshal(av)
	bb, _ := json.Marshal(bv)
	return bytes.Equal(ab, bb)
}

func makeFixtureResult() *Result {
	return &Result{
		SchemaVersion: "1",
		Run: RunMeta{
			ID:              "2026-05-02T14-30-00Z_bsc10k_host_abc12345",
			StartedAt:       time.Date(2026, 5, 2, 14, 30, 0, 0, time.UTC),
			FinishedAt:      time.Date(2026, 5, 2, 15, 32, 11, 0, time.UTC),
			BscbenchVersion: "v0.1.0",
			BSCVersion:      "v1.4.8",
			InputHash:       "sha256:abc",
			FromBlock:       40000000,
			ToBlock:         40010000,
			BlockCount:      10000,
			WarmupState:     "warm",
			SkipWarmup:      false,
		},
		Sysinfo: Sysinfo{Host: HostInfo{Hostname: "host", OS: "linux"}},
		Metrics: Metrics{Mgasps: 87.4, TotalGasUsed: 318200000000, TotalTxCount: 1240000, TotalWallSec: 3641.2, ExecNsP50: 1800000},
		Passes:  Passes{Warmup: PassMeta{WallSec: 4010.7, GasUsed: 318200000000}, Measured: PassMeta{WallSec: 3641.2, GasUsed: 318200000000}},
	}
}
