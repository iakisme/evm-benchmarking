package sysinfo

import (
	"os"
	"runtime"
	"runtime/debug"
	"strconv"

	"github.com/kai-w/bscbench/internal/report"
)

func collectGo() report.GoInfo {
	gi := report.GoInfo{
		Version:    runtime.Version(),
		GOMAXPROCS: runtime.GOMAXPROCS(0),
		GOGC:       100,
	}
	if v := os.Getenv("GOGC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			gi.GOGC = n
		}
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.GoVersion != "" {
		gi.Version = info.GoVersion
	}
	return gi
}
