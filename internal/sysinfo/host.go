package sysinfo

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/kai-w/bscbench/internal/report"
)

func collectHost() (report.HostInfo, error) {
	hi := report.HostInfo{OS: runtime.GOOS}

	if h, err := os.Hostname(); err == nil {
		hi.Hostname = h
	}

	if data, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		hi.Kernel = strings.TrimSpace(string(data))
	}

	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		if pretty := parseOSRelease(string(data)); pretty != "" {
			hi.OS = pretty
		}
	}

	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		if up, err := parseUptime(string(data)); err == nil {
			hi.UptimeSec = up
		}
	}

	return hi, nil
}

func parseUptime(s string) (uint64, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return 0, errors.New("empty /proc/uptime")
	}
	f, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parse: %w", err)
	}
	if f < 0 {
		return 0, errors.New("negative uptime")
	}
	return uint64(f), nil
}

func parseOSRelease(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			val := strings.TrimPrefix(line, "PRETTY_NAME=")
			val = strings.Trim(val, `"`)
			return val
		}
	}
	return ""
}
