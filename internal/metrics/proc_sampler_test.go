//go:build linux

package metrics

import (
	"testing"
	"time"
)

func TestProcSamplerStartsAndStops(t *testing.T) {
	s := NewProcSampler(50 * time.Millisecond)
	s.Start()

	time.Sleep(200 * time.Millisecond)
	samples := s.Stop()

	if len(samples) < 2 {
		t.Errorf("expected at least 2 samples in 200ms at 50ms interval, got %d", len(samples))
	}
	for i, sm := range samples {
		if sm.TsMs == 0 {
			t.Errorf("sample[%d] has zero ts", i)
		}
	}
}

func TestParseSelfStat(t *testing.T) {
	// fields: pid, comm, state, ppid, ..., utime(14), stime(15), ...
	// Synthetic line covering enough fields to extract utime/stime.
	line := "1234 (bscbench) S 1 1234 1234 0 -1 4194304 100 0 0 0 1500 700 0 0 20 0 1 0 1 0 0\n"
	utime, stime, err := parseSelfStat(line)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if utime != 1500 || stime != 700 {
		t.Errorf("got %d/%d, want 1500/700", utime, stime)
	}
}
