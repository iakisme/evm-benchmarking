package report

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResultRoundTripsJSON(t *testing.T) {
	r := &Result{
		SchemaVersion: "1",
		Run: RunMeta{
			ID:              "2026-05-02T14-30-00Z_bsc10k_host_abc12345",
			StartedAt:       time.Date(2026, 5, 2, 14, 30, 0, 0, time.UTC),
			FinishedAt:      time.Date(2026, 5, 2, 15, 32, 11, 0, time.UTC),
			BscbenchVersion: "v0.1.0",
			BSCVersion:      "v1.7.3",
			InputHash:       "sha256:abc",
			FromBlock:       40000000,
			ToBlock:         40010000,
			BlockCount:      10000,
			WarmupState:     "warm",
			SkipWarmup:      false,
		},
		Sysinfo: Sysinfo{Host: HostInfo{Hostname: "h"}},
		Metrics: Metrics{Mgasps: 87.4, TotalGasUsed: 318200000000},
		Passes: Passes{
			Warmup:   PassMeta{WallSec: 4010.7, GasUsed: 318200000000},
			Measured: PassMeta{WallSec: 3641.2, GasUsed: 318200000000},
		},
	}

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Result
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.SchemaVersion != "1" {
		t.Errorf("schema_version = %q", got.SchemaVersion)
	}
	if got.Run.BlockCount != 10000 {
		t.Errorf("block_count = %d", got.Run.BlockCount)
	}
	if got.Metrics.Mgasps != 87.4 {
		t.Errorf("mgasps = %v", got.Metrics.Mgasps)
	}
}
