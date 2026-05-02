package sysinfo

import (
	"context"
	"testing"
	"time"
)

func TestCollect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	si, err := Collect(ctx, []string{"/tmp"})
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if si.Host.Hostname == "" {
		t.Errorf("hostname empty")
	}
	if si.Go.Version == "" {
		t.Errorf("go version empty")
	}
	// si.CPU.CoresLogical may be 0 on platforms with no /proc/cpuinfo, but
	// the fallback uses runtime.NumCPU() which should always be ≥ 1.
	if si.CPU.CoresLogical < 1 {
		t.Errorf("cores_logical = %d", si.CPU.CoresLogical)
	}
}
