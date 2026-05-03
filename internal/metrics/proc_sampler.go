package metrics

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kai-w/bscbench/internal/report"
)

// ProcSampler periodically reads /proc/self/{stat,status,io} and emits ProcSample.
type ProcSampler struct {
	interval time.Duration

	mu        sync.Mutex
	samples   []report.ProcSample
	stopChan  chan struct{}
	doneChan  chan struct{}
	prevUtime uint64
	prevStime uint64
	prevTs    time.Time

	clkTck float64 // jiffies per second
}

func NewProcSampler(interval time.Duration) *ProcSampler {
	return &ProcSampler{
		interval: interval,
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
		clkTck:   100, // POSIX default; we don't depend on cgo to read SC_CLK_TCK
	}
}

func (p *ProcSampler) Start() {
	go p.loop()
}

// Stop signals the sampler and returns the collected samples.
func (p *ProcSampler) Stop() []report.ProcSample {
	close(p.stopChan)
	<-p.doneChan
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]report.ProcSample(nil), p.samples...)
}

func (p *ProcSampler) loop() {
	defer close(p.doneChan)
	t := time.NewTicker(p.interval)
	defer t.Stop()

	// take an initial reading so the first tick can compute Δ
	p.tick(true)

	for {
		select {
		case <-p.stopChan:
			return
		case <-t.C:
			p.tick(false)
		}
	}
}

func (p *ProcSampler) tick(initial bool) {
	now := time.Now()
	utime, stime := readUtimeStime()
	rss := readRSSBytes()
	rB, wB := readSelfIO()

	if !initial {
		dt := now.Sub(p.prevTs).Seconds()
		jiffyDelta := float64(utime-p.prevUtime) + float64(stime-p.prevStime)
		var cpuPct float64
		if dt > 0 {
			cpuPct = (jiffyDelta / p.clkTck / dt) * 100
		}
		p.mu.Lock()
		p.samples = append(p.samples, report.ProcSample{
			TsMs:              now.UnixMilli(),
			CPUPct:            cpuPct,
			RSSBytes:          rss,
			DiskReadCumBytes:  rB,
			DiskWriteCumBytes: wB,
		})
		p.mu.Unlock()
	}
	p.prevUtime = utime
	p.prevStime = stime
	p.prevTs = now
}

func readUtimeStime() (utime, stime uint64) {
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, 0
	}
	u, s, err := parseSelfStat(string(data))
	if err != nil {
		return 0, 0
	}
	return u, s
}

// parseSelfStat extracts utime (field 14) and stime (field 15) from /proc/self/stat.
// The 'comm' field can contain spaces/parens; we find the last ')' and parse forward.
func parseSelfStat(s string) (utime, stime uint64, err error) {
	idx := strings.LastIndex(s, ")")
	if idx < 0 {
		return 0, 0, errors.New("no ) in /proc/self/stat")
	}
	rest := strings.TrimSpace(s[idx+1:])
	fields := strings.Fields(rest)
	// after ')': state ppid pgrp session tty_nr tpgid flags minflt cminflt majflt cmajflt utime stime ...
	// indexes:  0     1    2    3       4      5     6     7      8       9      10      11    12
	if len(fields) < 13 {
		return 0, 0, fmt.Errorf("stat: only %d fields after )", len(fields))
	}
	u, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	st, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return u, st, nil
}

func readRSSBytes() uint64 {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			return parseKB(strings.TrimPrefix(line, "VmRSS:"))
		}
	}
	return 0
}

// parseKB lives in sysinfo; redefine here to avoid the import cycle (sysinfo
// already imports metrics in some flows).
func parseKB(v string) uint64 {
	v = strings.TrimSpace(v)
	v = strings.TrimSuffix(v, " kB")
	v = strings.TrimSuffix(v, " KB")
	v = strings.TrimSpace(v)
	n, _ := strconv.ParseUint(v, 10, 64)
	return n * 1024
}

func readSelfIO() (read, write uint64) {
	data, err := os.ReadFile("/proc/self/io")
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "read_bytes:"):
			read, _ = strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "read_bytes:")), 10, 64)
		case strings.HasPrefix(line, "write_bytes:"):
			write, _ = strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "write_bytes:")), 10, 64)
		}
	}
	return
}
