package sysinfo

import (
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/iakisme/evm-benchmarking/internal/report"
)

// flagsOfInterest is the small subset that matters for EVM perf comparison.
var flagsOfInterest = map[string]bool{
	"avx2":    true,
	"avx512f": true,
	"sse4_2":  true,
	"bmi1":    true,
	"bmi2":    true,
	"adx":     true,
}

func collectCPU() (report.CPUInfo, error) {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return report.CPUInfo{
			CoresLogical: runtime.NumCPU(),
		}, nil // not fatal off-Linux
	}
	ci := parseCPUInfo(string(data))

	if g, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor"); err == nil {
		ci.Governor = strings.TrimSpace(string(g))
	}
	return ci, nil
}

func parseCPUInfo(s string) report.CPUInfo {
	ci := report.CPUInfo{}

	type proc struct {
		physID, coreID string
	}
	var procs []proc
	var current proc
	have := false

	flush := func() {
		if have {
			procs = append(procs, current)
		}
		current = proc{}
		have = false
	}

	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}
		key, value, ok := splitColon(line)
		if !ok {
			continue
		}
		switch key {
		case "processor":
			have = true
		case "model name":
			if ci.Model == "" {
				ci.Model = value
			}
		case "cpu MHz":
			if ci.MhzBase == 0 {
				if v, err := strconv.ParseFloat(value, 64); err == nil {
					ci.MhzBase = v
				}
			}
		case "physical id":
			current.physID = value
		case "core id":
			current.coreID = value
		case "flags":
			if len(ci.FlagsSubset) == 0 {
				for _, f := range strings.Fields(value) {
					if flagsOfInterest[f] {
						ci.FlagsSubset = append(ci.FlagsSubset, f)
					}
				}
			}
		}
	}
	flush()

	ci.CoresLogical = len(procs)
	seen := map[string]bool{}
	for _, p := range procs {
		key := p.physID + ":" + p.coreID
		if !seen[key] {
			seen[key] = true
			ci.CoresPhysical++
		}
	}
	if ci.CoresPhysical == 0 {
		ci.CoresPhysical = ci.CoresLogical
	}
	return ci
}

// splitColon splits a line at the first colon, trimming whitespace from both sides.
// It is used by multiple parsers (CPU, memory) in this package.
func splitColon(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}
