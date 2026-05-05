package sysinfo

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/iakisme/evm-benchmarking/internal/report"
)

type mountEntry struct {
	mount  string
	source string
	fs     string
}

// collectDisks inspects mounts that contain interesting paths (state dir, out dir).
func collectDisks(interestingPaths []string) ([]report.DiskInfo, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, nil
	}
	mounts := parseMountinfo(string(data))
	relevant := relevantMountsForPaths(mounts, interestingPaths)

	out := make([]report.DiskInfo, 0, len(relevant))
	for _, m := range relevant {
		di := report.DiskInfo{
			Device: m.source,
			Mount:  m.mount,
			FS:     m.fs,
		}
		fillBlockDevAttrs(&di)
		out = append(out, di)
	}
	return out, nil
}

func parseMountinfo(s string) []mountEntry {
	var out []mountEntry
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// format: ID parent major:minor root mount opts shared - fs source super-opts
		dashIdx := strings.Index(line, " - ")
		if dashIdx < 0 {
			continue
		}
		left := strings.Fields(line[:dashIdx])
		right := strings.Fields(line[dashIdx+3:])
		if len(left) < 5 || len(right) < 2 {
			continue
		}
		fs := right[0]
		source := right[1]
		mount := left[4]

		// skip pseudo-fs
		if !strings.HasPrefix(source, "/dev/") {
			continue
		}
		out = append(out, mountEntry{mount: mount, source: source, fs: fs})
	}
	return out
}

func relevantMountsForPaths(mounts []mountEntry, paths []string) []mountEntry {
	// for each path, find the longest mount prefix
	chosen := map[string]mountEntry{}
	for _, p := range paths {
		best := ""
		var bestE mountEntry
		for _, m := range mounts {
			if hasPathPrefix(p, m.mount) && len(m.mount) > len(best) {
				best = m.mount
				bestE = m
			}
		}
		if best != "" {
			chosen[best] = bestE
		}
	}
	out := make([]mountEntry, 0, len(chosen))
	for _, e := range chosen {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].mount < out[j].mount })
	return out
}

func hasPathPrefix(p, prefix string) bool {
	p = filepath.Clean(p)
	prefix = filepath.Clean(prefix)
	if prefix == "/" {
		return true
	}
	return p == prefix || strings.HasPrefix(p, prefix+string(filepath.Separator))
}

// blockDeviceName strips the partition suffix from a device name.
// Examples: "nvme0n1p3" → "nvme0n1", "sda1" → "sda", "mmcblk0p2" → "mmcblk0".
func blockDeviceName(dev string) string {
	if strings.HasPrefix(dev, "nvme") || strings.HasPrefix(dev, "mmcblk") {
		// strip trailing "p<digits>" if present
		if idx := strings.LastIndex(dev, "p"); idx > 0 {
			suffix := dev[idx+1:]
			if len(suffix) > 0 && allDigits(suffix) {
				return dev[:idx]
			}
		}
		return dev
	}
	// SCSI / virtio: strip trailing digits
	return strings.TrimRight(dev, "0123456789")
}

func allDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func fillBlockDevAttrs(di *report.DiskInfo) {
	dev := blockDeviceName(strings.TrimPrefix(di.Device, "/dev/"))
	base := "/sys/block/" + dev
	if _, err := os.Stat(base); err != nil {
		return
	}
	if data, err := os.ReadFile(base + "/device/model"); err == nil {
		di.Model = strings.TrimSpace(string(data))
	}
	if data, err := os.ReadFile(base + "/size"); err == nil {
		if sectors, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err == nil {
			di.SizeBytes = sectors * 512
		}
	}
	if data, err := os.ReadFile(base + "/queue/rotational"); err == nil {
		di.Rotational = strings.TrimSpace(string(data)) == "1"
	}
	if data, err := os.ReadFile(base + "/queue/scheduler"); err == nil {
		// "[mq-deadline] kyber none" → mq-deadline
		s := strings.TrimSpace(string(data))
		if i := strings.Index(s, "["); i >= 0 {
			if j := strings.Index(s[i:], "]"); j > 0 {
				di.QueueScheduler = s[i+1 : i+j]
			}
		}
	}
	if data, err := os.ReadFile(base + "/queue/discard_max_bytes"); err == nil {
		if v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err == nil {
			di.DiscardMaxBytes = v
		}
	}
}

