package sysinfo

import (
	"os"
	"strconv"
	"strings"

	"github.com/kai-w/bscbench/internal/report"
)

func collectMemory() (report.MemInfo, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return report.MemInfo{}, nil
	}
	return parseMeminfo(string(data)), nil
}

func parseMeminfo(s string) report.MemInfo {
	mi := report.MemInfo{}
	for _, line := range strings.Split(s, "\n") {
		key, value, ok := splitColon(line)
		if !ok {
			continue
		}
		switch key {
		case "MemTotal":
			mi.TotalBytes = parseKB(value)
		case "SwapTotal":
			mi.SwapBytes = parseKB(value)
		case "HugePages_Total":
			if v, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64); err == nil {
				mi.HugepagesTotal = v
			}
		}
	}
	return mi
}

// parseKB parses values like "16384000 kB" → bytes.
func parseKB(v string) uint64 {
	v = strings.TrimSpace(v)
	v = strings.TrimSuffix(v, " kB")
	v = strings.TrimSuffix(v, " KB")
	v = strings.TrimSpace(v)
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0
	}
	return n * 1024
}
