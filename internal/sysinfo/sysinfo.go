// Package sysinfo collects host inventory: OS, CPU, memory, disks, Go runtime, cloud.
package sysinfo

import (
	"context"

	"github.com/iakisme/evm-benchmarking/internal/report"
)

// Collect returns a Sysinfo populated by inspecting the host. interestingPaths
// determines which mount points are reported in the disk array.
func Collect(ctx context.Context, interestingPaths []string) (report.Sysinfo, error) {
	si := report.Sysinfo{}

	if h, err := collectHost(); err == nil {
		si.Host = h
	}
	if c, err := collectCPU(); err == nil {
		si.CPU = c
	}
	if m, err := collectMemory(); err == nil {
		si.Memory = m
	}
	if d, err := collectDisks(interestingPaths); err == nil {
		si.Disk = d
	}
	si.Go = collectGo()
	si.Cloud = collectCloud(ctx) // returns nil on no detection (Task 13)

	return si, nil
}
