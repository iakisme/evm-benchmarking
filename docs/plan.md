# bscbench Implementation Plan

> Historical: this is the task-by-task plan that produced the v0.1 binary.
> Steps use checkbox (`- [ ]`) syntax. Kept for transparency, not as a
> guide to current code.

**Goal:** Build the `bscbench` Go binary described in `docs/design.md` — a single-host tool that replays a 10,000-block BSC window with double-pass warmup and emits EVM + system metrics to JSON/CSV.

**Architecture:** Single Go module, single binary with three subcommands (`replay`, `sysinfo`, `version`). BSC imported as a Go dependency. Consensus bypassed; blocks executed via a stripped `core.StateProcessor`-equivalent path. Output is local files only.

**Tech Stack:** Go 1.22+, `github.com/spf13/cobra` for CLI, `github.com/bnb-chain/bsc` as direct dependency for `core/vm`, `core/state`, `core/types`, `core/rawdb`, `params`, `ethdb`. Standard library for everything else (no extra logging or metrics framework).

**Build order rationale:** Bootstrap → ship sysinfo first (no BSC dep, validates the JSON output schema end-to-end on a real machine) → add BSC dep → corpus parsing → chain integration → metrics → runner → wire `replay`. Each phase is independently testable. BSC dep doesn't enter `go.mod` until Phase 3, keeping early phases fast to compile.

**Conventions for every task:**
- Use Go 1.22+ idioms (`any` over `interface{}`, `iter.Seq` only where genuinely useful — not in this plan).
- Test files live next to source. `testdata/` for fixtures.
- All `time.Duration` arithmetic uses `time.Now()` from a `Clock` interface only where needed for testability; otherwise direct `time.Now()` calls are fine for production paths and patched only in tests via dependency injection at construction.
- `context.Context` is plumbed through any goroutine-spawning function and any function that does I/O against the corpus or DB.
- Every package keeps `internal/` placement. Nothing under `internal/` is part of any public API.
- Errors wrap with `fmt.Errorf("...: %w", err)`. No bare `return err` in user-facing code paths (CLI handlers, `replay`/`sysinfo` entry points).

---

## Phase 0 — Bootstrap

### Task 1: Initialize Go module and project layout

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Modify: `README.md`
- Create: `cmd/bscbench/main.go`

- [ ] **Step 1: Initialize the Go module**

Run from repo root:

```bash
go mod init github.com/kai-w/bscbench
```

Expected: creates `go.mod` with module path and Go directive.

- [ ] **Step 2: Set Go toolchain version**

Edit `go.mod` to:

```
module github.com/kai-w/bscbench

go 1.22
```

- [ ] **Step 3: Create `.gitignore`**

Write `.gitignore`:

```
# Build artifacts
/bscbench
/dist/
/build/

# Test artifacts
/coverage.out
/coverage.html

# Editor
.idea/
.vscode/
*.swp
.DS_Store

# Local benchmark output
/results/
/workdir/
```

- [ ] **Step 4: Update README.md**

Replace contents of `README.md` with:

```markdown
# bscbench

Single-host benchmark tool that replays a fixed 10,000-block BSC window and
emits EVM and system metrics to JSON/CSV. Designed to compare EVM execution
performance across cloud VM configurations.

See `docs/design.md` for the design
spec and `docs/plan.md` for the
implementation plan.

## Build

    go build -o bscbench ./cmd/bscbench

## Usage

    bscbench version
    bscbench sysinfo --out=sysinfo.json
    bscbench replay  --input=<dir> --out-dir=<dir>
```

- [ ] **Step 5: Create the binary entry point**

Write `cmd/bscbench/main.go`:

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bscbench:", err)
		os.Exit(1)
	}
}

func run() error {
	return newRootCmd().Execute()
}
```

- [ ] **Step 6: Verify it compiles**

Run: `go build ./...`
Expected: succeeds, no output. (`newRootCmd` not yet defined — this step will FAIL. Continue to Task 2; the failure is intentional and gets fixed there.)

If you see `undefined: newRootCmd` — good, that's expected. Move on.

- [ ] **Step 7: Commit**

```bash
git add go.mod .gitignore README.md cmd/bscbench/main.go
git commit -m "Bootstrap Go module and binary entry point"
```

---

### Task 2: Add cobra root command

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `cmd/bscbench/root.go`

- [ ] **Step 1: Add cobra dependency**

Run:

```bash
go get github.com/spf13/cobra@v1.8.1
go mod tidy
```

Expected: `go.mod` and `go.sum` updated. `go.sum` newly created.

- [ ] **Step 2: Write the root command**

Write `cmd/bscbench/root.go`:

```go
package main

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "bscbench",
		Short:         "BSC EVM benchmark tool",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newVersionCmd())
	return cmd
}
```

- [ ] **Step 3: Verify build still fails (newVersionCmd undefined)**

Run: `go build ./...`
Expected: FAIL with `undefined: newVersionCmd`. Move to Task 3.

---

### Task 3: Add `version` subcommand

**Files:**
- Create: `cmd/bscbench/version.go`
- Create: `cmd/bscbench/version_test.go`

- [ ] **Step 1: Write the failing test**

Write `cmd/bscbench/version_test.go`:

```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	cmd := newVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "bscbench") {
		t.Errorf("expected output to mention bscbench, got %q", out)
	}
	if !strings.Contains(out, "bsc=") {
		t.Errorf("expected output to include bsc dependency version, got %q", out)
	}
}
```

- [ ] **Step 2: Run the test, verify it fails**

Run: `go test ./cmd/bscbench/ -run TestVersionCommand -v`
Expected: FAIL — `undefined: newVersionCmd`.

- [ ] **Step 3: Implement `version`**

Write `cmd/bscbench/version.go`:

```go
package main

import (
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Version is set via -ldflags at build time.
var Version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print bscbench and BSC dependency versions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Printf("bscbench=%s bsc=%s go=%s\n",
				Version, bscDepVersion(), runtimeGoVersion())
			return nil
		},
	}
}

func bscDepVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/bnb-chain/bsc" {
			return dep.Version
		}
	}
	return "none" // BSC dep not yet wired in early phases
}

func runtimeGoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	return info.GoVersion
}
```

- [ ] **Step 4: Run the test, verify it passes**

Run: `go test ./cmd/bscbench/ -run TestVersionCommand -v`
Expected: PASS.

- [ ] **Step 5: Smoke test the binary**

Run:

```bash
go run ./cmd/bscbench version
```

Expected stdout:

```
bscbench=dev bsc=none go=go1.22.x
```

- [ ] **Step 6: Commit**

```bash
git add cmd/bscbench/root.go cmd/bscbench/version.go cmd/bscbench/version_test.go go.mod go.sum
git commit -m "Add cobra root and version subcommand"
```

---

## Phase 1 — Report package (output schema)

The `report` package owns the on-disk schema. Building it first locks the schema down before anything else is wired in, and the rest of the system feeds into known struct types.

### Task 4: Report schema types

**Files:**
- Create: `internal/report/schema.go`
- Create: `internal/report/schema_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/report/schema_test.go`:

```go
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
			ID:               "2026-05-02T14-30-00Z_bsc10k_host_abc12345",
			StartedAt:        time.Date(2026, 5, 2, 14, 30, 0, 0, time.UTC),
			FinishedAt:       time.Date(2026, 5, 2, 15, 32, 11, 0, time.UTC),
			BscbenchVersion:  "v0.1.0",
			BSCVersion:       "v1.4.8",
			InputHash:        "sha256:abc",
			FromBlock:        40000000,
			ToBlock:          40010000,
			BlockCount:       10000,
			WarmupState:      "warm",
			SkipWarmup:       false,
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
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/report/ -run TestResultRoundTripsJSON -v`
Expected: FAIL — `undefined: Result`.

- [ ] **Step 3: Implement schema types**

Write `internal/report/schema.go`:

```go
// Package report defines the on-disk output schema for bscbench runs.
package report

import "time"

const SchemaVersion = "1"

type Result struct {
	SchemaVersion string  `json:"schema_version"`
	Run           RunMeta `json:"run"`
	Sysinfo       Sysinfo `json:"sysinfo"`
	Metrics       Metrics `json:"metrics"`
	Passes        Passes  `json:"passes"`
}

type RunMeta struct {
	ID              string    `json:"id"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
	BscbenchVersion string    `json:"bscbench_version"`
	BSCVersion      string    `json:"bsc_version"`
	InputHash       string    `json:"input_hash"`
	FromBlock       uint64    `json:"from_block"`
	ToBlock         uint64    `json:"to_block"`
	BlockCount      uint64    `json:"block_count"`
	WarmupState     string    `json:"warmup_state"`
	SkipWarmup      bool      `json:"skip_warmup"`
}

type Sysinfo struct {
	Host   HostInfo  `json:"host"`
	CPU    CPUInfo   `json:"cpu"`
	Memory MemInfo   `json:"memory"`
	Disk   []DiskInfo `json:"disk"`
	Go     GoInfo    `json:"go"`
	Cloud  *CloudInfo `json:"cloud"`
}

type HostInfo struct {
	Hostname  string `json:"hostname"`
	Kernel    string `json:"kernel"`
	OS        string `json:"os"`
	UptimeSec uint64 `json:"uptime_sec"`
}

type CPUInfo struct {
	Model         string   `json:"model"`
	CoresPhysical int      `json:"cores_physical"`
	CoresLogical  int      `json:"cores_logical"`
	FlagsSubset   []string `json:"flags_subset"`
	Governor      string   `json:"governor"`
	MhzBase       float64  `json:"mhz_base"`
}

type MemInfo struct {
	TotalBytes      uint64 `json:"total_bytes"`
	SwapBytes       uint64 `json:"swap_bytes"`
	HugepagesTotal  uint64 `json:"hugepages_total"`
}

type DiskInfo struct {
	Device          string `json:"device"`
	Model           string `json:"model"`
	SizeBytes       uint64 `json:"size_bytes"`
	FS              string `json:"fs"`
	Mount           string `json:"mount"`
	Rotational      bool   `json:"rotational"`
	QueueScheduler  string `json:"queue_scheduler"`
	DiscardMaxBytes uint64 `json:"discard_max_bytes"`
}

type GoInfo struct {
	Version    string `json:"version"`
	GOMAXPROCS int    `json:"gomaxprocs"`
	GOGC       int    `json:"gogc"`
}

type CloudInfo struct {
	Provider     string `json:"provider"`
	InstanceType string `json:"instance_type"`
	AZ           string `json:"az"`
	Region       string `json:"region"`
}

type Metrics struct {
	Mgasps              float64 `json:"mgasps"`
	TotalGasUsed        uint64  `json:"total_gas_used"`
	TotalTxCount        uint64  `json:"total_tx_count"`
	RevertedTxCount     uint64  `json:"reverted_tx_count"`
	TotalWallSec        float64 `json:"total_wall_sec"`
	TxPerSec            float64 `json:"tx_per_sec"`
	BlockPerSec         float64 `json:"block_per_sec"`
	ExecNsP50           uint64  `json:"exec_ns_p50"`
	ExecNsP95           uint64  `json:"exec_ns_p95"`
	ExecNsP99           uint64  `json:"exec_ns_p99"`
	TrieCommitNsP50     uint64  `json:"trie_commit_ns_p50"`
	TrieCommitNsP95     uint64  `json:"trie_commit_ns_p95"`
	TrieCommitNsP99     uint64  `json:"trie_commit_ns_p99"`
	GasUsedPerBlockP50  uint64  `json:"gas_used_per_block_p50"`
	GasUsedPerBlockP95  uint64  `json:"gas_used_per_block_p95"`
	GasUsedPerBlockP99  uint64  `json:"gas_used_per_block_p99"`
	CPUPctAvg           float64 `json:"cpu_pct_avg"`
	CPUPctMax           float64 `json:"cpu_pct_max"`
	RSSPeakBytes        uint64  `json:"rss_peak_bytes"`
	DiskReadTotalBytes  uint64  `json:"disk_read_total_bytes"`
	DiskWriteTotalBytes uint64  `json:"disk_write_total_bytes"`
	DiskReadMBpsAvg     float64 `json:"disk_read_MBps_avg"`
	DiskWriteMBpsAvg    float64 `json:"disk_write_MBps_avg"`
}

type Passes struct {
	Warmup   PassMeta `json:"warmup"`
	Measured PassMeta `json:"measured"`
}

type PassMeta struct {
	WallSec  float64 `json:"wall_sec"`
	GasUsed  uint64  `json:"gas_used"`
}

// BlockRecord is one row of blocks.csv.
type BlockRecord struct {
	BlockNumber      uint64
	TxCount          uint32
	GasUsed          uint64
	ExecNs           uint64
	StateReadCount   uint64
	StateWriteCount  uint64
	TrieCommitNs     uint64
	DBReadBytes      uint64
	DBWriteBytes     uint64
}

// ProcSample is one row of proc_samples.csv.
type ProcSample struct {
	TsMs              int64
	CPUPct            float64
	RSSBytes          uint64
	DiskReadCumBytes  uint64
	DiskWriteCumBytes uint64
}
```

- [ ] **Step 4: Run, verify it passes**

Run: `go test ./internal/report/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/report/schema.go internal/report/schema_test.go
git commit -m "report: define on-disk schema types"
```

---

### Task 5: Result JSON writer

**Files:**
- Create: `internal/report/result_writer.go`
- Create: `internal/report/result_writer_test.go`
- Create: `internal/report/testdata/result_golden.json`

- [ ] **Step 1: Write the failing test**

Write `internal/report/result_writer_test.go`:

```go
package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteResult(t *testing.T) {
	r := makeFixtureResult()

	dir := t.TempDir()
	path := filepath.Join(dir, "result.json")
	if err := WriteResult(r, path); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	want, err := os.ReadFile("testdata/result_golden.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	// Compare as canonical JSON to ignore ordering.
	if !canonicalEqual(t, got, want) {
		t.Errorf("JSON differs from golden\n--- got\n%s\n--- want\n%s\n", got, want)
	}
}

func canonicalEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("unmarshal a: %v", err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("unmarshal b: %v", err)
	}
	ab, _ := json.Marshal(av)
	bb, _ := json.Marshal(bv)
	return bytes.Equal(ab, bb)
}

func makeFixtureResult() *Result {
	return &Result{
		SchemaVersion: "1",
		Run: RunMeta{
			ID:              "2026-05-02T14-30-00Z_bsc10k_host_abc12345",
			StartedAt:       time.Date(2026, 5, 2, 14, 30, 0, 0, time.UTC),
			FinishedAt:      time.Date(2026, 5, 2, 15, 32, 11, 0, time.UTC),
			BscbenchVersion: "v0.1.0",
			BSCVersion:      "v1.4.8",
			InputHash:       "sha256:abc",
			FromBlock:       40000000,
			ToBlock:         40010000,
			BlockCount:      10000,
			WarmupState:     "warm",
			SkipWarmup:      false,
		},
		Sysinfo: Sysinfo{Host: HostInfo{Hostname: "host", OS: "linux"}},
		Metrics: Metrics{Mgasps: 87.4, TotalGasUsed: 318200000000, TotalTxCount: 1240000, TotalWallSec: 3641.2, ExecNsP50: 1800000},
		Passes:  Passes{Warmup: PassMeta{WallSec: 4010.7, GasUsed: 318200000000}, Measured: PassMeta{WallSec: 3641.2, GasUsed: 318200000000}},
	}
}
```

- [ ] **Step 2: Create the golden file**

Write `internal/report/testdata/result_golden.json` (the JSON the writer should produce; create it by hand to lock the schema):

```json
{
  "schema_version": "1",
  "run": {
    "id": "2026-05-02T14-30-00Z_bsc10k_host_abc12345",
    "started_at": "2026-05-02T14:30:00Z",
    "finished_at": "2026-05-02T15:32:11Z",
    "bscbench_version": "v0.1.0",
    "bsc_version": "v1.4.8",
    "input_hash": "sha256:abc",
    "from_block": 40000000,
    "to_block": 40010000,
    "block_count": 10000,
    "warmup_state": "warm",
    "skip_warmup": false
  },
  "sysinfo": {
    "host": {"hostname": "host", "kernel": "", "os": "linux", "uptime_sec": 0},
    "cpu": {"model": "", "cores_physical": 0, "cores_logical": 0, "flags_subset": null, "governor": "", "mhz_base": 0},
    "memory": {"total_bytes": 0, "swap_bytes": 0, "hugepages_total": 0},
    "disk": null,
    "go": {"version": "", "gomaxprocs": 0, "gogc": 0},
    "cloud": null
  },
  "metrics": {
    "mgasps": 87.4,
    "total_gas_used": 318200000000,
    "total_tx_count": 1240000,
    "reverted_tx_count": 0,
    "total_wall_sec": 3641.2,
    "tx_per_sec": 0,
    "block_per_sec": 0,
    "exec_ns_p50": 1800000,
    "exec_ns_p95": 0,
    "exec_ns_p99": 0,
    "trie_commit_ns_p50": 0,
    "trie_commit_ns_p95": 0,
    "trie_commit_ns_p99": 0,
    "gas_used_per_block_p50": 0,
    "gas_used_per_block_p95": 0,
    "gas_used_per_block_p99": 0,
    "cpu_pct_avg": 0,
    "cpu_pct_max": 0,
    "rss_peak_bytes": 0,
    "disk_read_total_bytes": 0,
    "disk_write_total_bytes": 0,
    "disk_read_MBps_avg": 0,
    "disk_write_MBps_avg": 0
  },
  "passes": {
    "warmup":   {"wall_sec": 4010.7, "gas_used": 318200000000},
    "measured": {"wall_sec": 3641.2, "gas_used": 318200000000}
  }
}
```

- [ ] **Step 3: Run, verify the test fails**

Run: `go test ./internal/report/ -run TestWriteResult -v`
Expected: FAIL — `undefined: WriteResult`.

- [ ] **Step 4: Implement the writer**

Write `internal/report/result_writer.go`:

```go
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteResult atomically writes r as pretty-printed JSON to path.
func WriteResult(r *Result, path string) error {
	if r.SchemaVersion == "" {
		r.SchemaVersion = SchemaVersion
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".result-*.json")
	if err != nil {
		return fmt.Errorf("tempfile: %w", err)
	}
	defer os.Remove(tmp.Name())

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run, verify it passes**

Run: `go test ./internal/report/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/report/result_writer.go internal/report/result_writer_test.go internal/report/testdata/result_golden.json
git commit -m "report: write result.json (atomic, indented)"
```

---

### Task 6: Block CSV writer

**Files:**
- Create: `internal/report/blocks_writer.go`
- Create: `internal/report/blocks_writer_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/report/blocks_writer_test.go`:

```go
package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteBlocksCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blocks.csv")

	records := []BlockRecord{
		{BlockNumber: 1, TxCount: 10, GasUsed: 100, ExecNs: 200, StateReadCount: 5,
			StateWriteCount: 2, TrieCommitNs: 50, DBReadBytes: 1024, DBWriteBytes: 512},
		{BlockNumber: 2, TxCount: 12, GasUsed: 110, ExecNs: 210, StateReadCount: 6,
			StateWriteCount: 3, TrieCommitNs: 55, DBReadBytes: 1100, DBWriteBytes: 530},
	}

	w, err := NewBlocksWriter(path)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	for _, r := range records {
		if err := w.Write(r); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "block_number,tx_count,gas_used,exec_ns,state_read_count,state_write_count,trie_commit_ns,db_read_bytes,db_write_bytes\n" +
		"1,10,100,200,5,2,50,1024,512\n" +
		"2,12,110,210,6,3,55,1100,530\n"
	if string(got) != want {
		t.Errorf("CSV mismatch\n--- got\n%s\n--- want\n%s", got, want)
	}
	if !strings.HasSuffix(path, ".csv") {
		t.Errorf("path: %s", path)
	}
}
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/report/ -run TestWriteBlocksCSV -v`
Expected: FAIL — `undefined: NewBlocksWriter`.

- [ ] **Step 3: Implement the writer**

Write `internal/report/blocks_writer.go`:

```go
package report

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

type BlocksWriter struct {
	f  *os.File
	bw *bufio.Writer
}

func NewBlocksWriter(path string) (*BlocksWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", path, err)
	}
	bw := bufio.NewWriterSize(f, 1<<16)
	if _, err := bw.WriteString(
		"block_number,tx_count,gas_used,exec_ns,state_read_count,state_write_count,trie_commit_ns,db_read_bytes,db_write_bytes\n"); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &BlocksWriter{f: f, bw: bw}, nil
}

func (w *BlocksWriter) Write(r BlockRecord) error {
	// hand-rolled to avoid encoding/csv overhead in a 10k-row hot loop
	buf := make([]byte, 0, 96)
	buf = strconv.AppendUint(buf, r.BlockNumber, 10)
	buf = append(buf, ',')
	buf = strconv.AppendUint(buf, uint64(r.TxCount), 10)
	buf = append(buf, ',')
	buf = strconv.AppendUint(buf, r.GasUsed, 10)
	buf = append(buf, ',')
	buf = strconv.AppendUint(buf, r.ExecNs, 10)
	buf = append(buf, ',')
	buf = strconv.AppendUint(buf, r.StateReadCount, 10)
	buf = append(buf, ',')
	buf = strconv.AppendUint(buf, r.StateWriteCount, 10)
	buf = append(buf, ',')
	buf = strconv.AppendUint(buf, r.TrieCommitNs, 10)
	buf = append(buf, ',')
	buf = strconv.AppendUint(buf, r.DBReadBytes, 10)
	buf = append(buf, ',')
	buf = strconv.AppendUint(buf, r.DBWriteBytes, 10)
	buf = append(buf, '\n')
	_, err := w.bw.Write(buf)
	return err
}

func (w *BlocksWriter) Close() error {
	if err := w.bw.Flush(); err != nil {
		_ = w.f.Close()
		return err
	}
	return w.f.Close()
}
```

- [ ] **Step 4: Run, verify it passes**

Run: `go test ./internal/report/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/report/blocks_writer.go internal/report/blocks_writer_test.go
git commit -m "report: stream blocks.csv with hand-rolled formatting"
```

---

### Task 7: Proc samples CSV writer

**Files:**
- Create: `internal/report/proc_writer.go`
- Create: `internal/report/proc_writer_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/report/proc_writer_test.go`:

```go
package report

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteProcSamplesCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "proc_samples.csv")

	samples := []ProcSample{
		{TsMs: 1714658400000, CPUPct: 380.2, RSSBytes: 12400000000,
			DiskReadCumBytes: 1200000000, DiskWriteCumBytes: 180000000},
		{TsMs: 1714658401000, CPUPct: 410.5, RSSBytes: 12450000000,
			DiskReadCumBytes: 1290000000, DiskWriteCumBytes: 210000000},
	}

	w, err := NewProcWriter(path)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	for _, s := range samples {
		if err := w.Write(s); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "ts_ms,cpu_pct,rss_bytes,disk_read_cum_bytes,disk_write_cum_bytes\n" +
		"1714658400000,380.20,12400000000,1200000000,180000000\n" +
		"1714658401000,410.50,12450000000,1290000000,210000000\n"
	if string(got) != want {
		t.Errorf("CSV mismatch\n--- got\n%s\n--- want\n%s", got, want)
	}
}
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/report/ -run TestWriteProcSamplesCSV -v`
Expected: FAIL — `undefined: NewProcWriter`.

- [ ] **Step 3: Implement the writer**

Write `internal/report/proc_writer.go`:

```go
package report

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

type ProcWriter struct {
	f  *os.File
	bw *bufio.Writer
}

func NewProcWriter(path string) (*ProcWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", path, err)
	}
	bw := bufio.NewWriterSize(f, 1<<15)
	if _, err := bw.WriteString(
		"ts_ms,cpu_pct,rss_bytes,disk_read_cum_bytes,disk_write_cum_bytes\n"); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &ProcWriter{f: f, bw: bw}, nil
}

func (w *ProcWriter) Write(s ProcSample) error {
	buf := make([]byte, 0, 64)
	buf = strconv.AppendInt(buf, s.TsMs, 10)
	buf = append(buf, ',')
	buf = strconv.AppendFloat(buf, s.CPUPct, 'f', 2, 64)
	buf = append(buf, ',')
	buf = strconv.AppendUint(buf, s.RSSBytes, 10)
	buf = append(buf, ',')
	buf = strconv.AppendUint(buf, s.DiskReadCumBytes, 10)
	buf = append(buf, ',')
	buf = strconv.AppendUint(buf, s.DiskWriteCumBytes, 10)
	buf = append(buf, '\n')
	_, err := w.bw.Write(buf)
	return err
}

func (w *ProcWriter) Close() error {
	if err := w.bw.Flush(); err != nil {
		_ = w.f.Close()
		return err
	}
	return w.f.Close()
}
```

- [ ] **Step 4: Run, verify it passes**

Run: `go test ./internal/report/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/report/proc_writer.go internal/report/proc_writer_test.go
git commit -m "report: stream proc_samples.csv"
```

---

## Phase 2 — Sysinfo

`sysinfo` collectors are split per concern. Each collector returns its own struct + an error. The top-level `Collect()` calls them sequentially (none of them are slow). Cloud is parallel-probed inside its own collector.

Most sysinfo paths read `/proc` and `/sys`. They MUST work on Linux; on other OSes they fall back to "best effort" — return zero values where the platform doesn't expose what we need. The CLI is intended for Linux benchmark hosts; macOS/Windows is for developer convenience only.

### Task 8: Sysinfo Host collector

**Files:**
- Create: `internal/sysinfo/host.go`
- Create: `internal/sysinfo/host_test.go`
- Create: `internal/sysinfo/testdata/proc_uptime`
- Create: `internal/sysinfo/testdata/etc_os_release`

- [ ] **Step 1: Write the failing test**

Write `internal/sysinfo/host_test.go`:

```go
package sysinfo

import (
	"testing"
)

func TestParseUptime(t *testing.T) {
	got, err := parseUptime("48300.42 12000.10\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != 48300 {
		t.Errorf("uptime = %d, want 48300", got)
	}
}

func TestParseOSRelease(t *testing.T) {
	in := `NAME="Ubuntu"
VERSION="22.04.4 LTS (Jammy Jellyfish)"
ID=ubuntu
PRETTY_NAME="Ubuntu 22.04.4 LTS"
`
	got := parseOSRelease(in)
	want := "Ubuntu 22.04.4 LTS"
	if got != want {
		t.Errorf("os = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/sysinfo/ -run "TestParseUptime|TestParseOSRelease" -v`
Expected: FAIL — `undefined: parseUptime`.

- [ ] **Step 3: Implement the collector**

Write `internal/sysinfo/host.go`:

```go
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
```

- [ ] **Step 4: Run, verify it passes**

Run: `go test ./internal/sysinfo/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sysinfo/host.go internal/sysinfo/host_test.go
git commit -m "sysinfo: collect hostname, kernel, OS, uptime"
```

---

### Task 9: Sysinfo CPU collector

**Files:**
- Create: `internal/sysinfo/cpu.go`
- Create: `internal/sysinfo/cpu_test.go`
- Create: `internal/sysinfo/testdata/proc_cpuinfo`

- [ ] **Step 1: Create test fixture**

Write `internal/sysinfo/testdata/proc_cpuinfo` (a slimmed-down `/proc/cpuinfo`):

```
processor	: 0
vendor_id	: GenuineIntel
cpu family	: 6
model name	: Intel(R) Xeon(R) Platinum 8375C CPU @ 2.90GHz
cpu MHz		: 2900.000
physical id	: 0
core id		: 0
flags		: fpu vme de pse tsc msr pae mce cx8 apic sep mtrr pge mca cmov pat pse36 clflush mmx fxsr sse sse2 ht syscall nx pdpe1gb rdtscp lm pni pclmulqdq ssse3 fma cx16 pcid sse4_1 sse4_2 movbe popcnt aes xsave avx f16c rdrand hypervisor lahf_lm avx2 avx512f

processor	: 1
vendor_id	: GenuineIntel
model name	: Intel(R) Xeon(R) Platinum 8375C CPU @ 2.90GHz
cpu MHz		: 2900.000
physical id	: 0
core id		: 0
flags		: fpu vme

processor	: 2
vendor_id	: GenuineIntel
model name	: Intel(R) Xeon(R) Platinum 8375C CPU @ 2.90GHz
cpu MHz		: 2900.000
physical id	: 0
core id		: 1
flags		: fpu vme
```

- [ ] **Step 2: Write the failing test**

Write `internal/sysinfo/cpu_test.go`:

```go
package sysinfo

import (
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestParseCPUInfo(t *testing.T) {
	data, err := os.ReadFile("testdata/proc_cpuinfo")
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	got := parseCPUInfo(string(data))

	if got.Model != "Intel(R) Xeon(R) Platinum 8375C CPU @ 2.90GHz" {
		t.Errorf("model = %q", got.Model)
	}
	if got.CoresLogical != 3 {
		t.Errorf("logical = %d, want 3", got.CoresLogical)
	}
	if got.CoresPhysical != 2 {
		t.Errorf("physical = %d, want 2", got.CoresPhysical)
	}
	if got.MhzBase != 2900.0 {
		t.Errorf("mhz = %v", got.MhzBase)
	}

	wantFlags := []string{"avx2", "avx512f", "sse4_2"}
	gotFlags := append([]string(nil), got.FlagsSubset...)
	sort.Strings(gotFlags)
	sort.Strings(wantFlags)
	if !reflect.DeepEqual(gotFlags, wantFlags) {
		t.Errorf("flags = %v, want %v", gotFlags, wantFlags)
	}
}
```

- [ ] **Step 3: Run, verify it fails**

Run: `go test ./internal/sysinfo/ -run TestParseCPUInfo -v`
Expected: FAIL — `undefined: parseCPUInfo`.

- [ ] **Step 4: Implement the collector**

Write `internal/sysinfo/cpu.go`:

```go
package sysinfo

import (
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/kai-w/bscbench/internal/report"
)

// flagsOfInterest is the small subset that matters for EVM perf comparison.
var flagsOfInterest = map[string]bool{
	"avx2":     true,
	"avx512f":  true,
	"sse4_2":   true,
	"bmi1":     true,
	"bmi2":     true,
	"adx":      true,
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

func splitColon(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}
```

- [ ] **Step 5: Run, verify it passes**

Run: `go test ./internal/sysinfo/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/sysinfo/cpu.go internal/sysinfo/cpu_test.go internal/sysinfo/testdata/proc_cpuinfo
git commit -m "sysinfo: parse /proc/cpuinfo for model, cores, flags"
```

---

### Task 10: Sysinfo Memory collector

**Files:**
- Create: `internal/sysinfo/memory.go`
- Create: `internal/sysinfo/memory_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/sysinfo/memory_test.go`:

```go
package sysinfo

import "testing"

func TestParseMeminfo(t *testing.T) {
	in := `MemTotal:       16384000 kB
MemFree:         1234567 kB
SwapTotal:        2048000 kB
HugePages_Total:        0
HugePages_Free:         0
`
	got := parseMeminfo(in)
	if got.TotalBytes != 16384000*1024 {
		t.Errorf("total = %d", got.TotalBytes)
	}
	if got.SwapBytes != 2048000*1024 {
		t.Errorf("swap = %d", got.SwapBytes)
	}
	if got.HugepagesTotal != 0 {
		t.Errorf("hugepages = %d", got.HugepagesTotal)
	}
}
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/sysinfo/ -run TestParseMeminfo -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

Write `internal/sysinfo/memory.go`:

```go
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
```

- [ ] **Step 4: Run, verify it passes**

Run: `go test ./internal/sysinfo/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sysinfo/memory.go internal/sysinfo/memory_test.go
git commit -m "sysinfo: parse /proc/meminfo"
```

---

### Task 11: Sysinfo Disk collector

**Files:**
- Create: `internal/sysinfo/disk.go`
- Create: `internal/sysinfo/disk_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/sysinfo/disk_test.go`:

```go
package sysinfo

import "testing"

func TestParseMountinfoFiltersOurPaths(t *testing.T) {
	in := `38 27 259:1 / /data rw,relatime shared:13 - ext4 /dev/nvme0n1 rw
27 0 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw
40 27 0:21 / /proc rw,nosuid,nodev,noexec shared:14 - proc proc rw
`
	mounts := parseMountinfo(in)
	if len(mounts) != 2 {
		t.Fatalf("expected 2 real mounts, got %d", len(mounts))
	}
	if mounts[0].mount != "/data" {
		t.Errorf("[0].mount = %q", mounts[0].mount)
	}
	if mounts[0].source != "/dev/nvme0n1" {
		t.Errorf("[0].source = %q", mounts[0].source)
	}
	if mounts[0].fs != "ext4" {
		t.Errorf("[0].fs = %q", mounts[0].fs)
	}
}

func TestRelevantMountsForPaths(t *testing.T) {
	mounts := []mountEntry{
		{mount: "/", source: "/dev/sda1", fs: "ext4"},
		{mount: "/data", source: "/dev/nvme0n1", fs: "ext4"},
	}
	got := relevantMountsForPaths(mounts, []string{"/data/state", "/data/out"})
	if len(got) != 1 || got[0].mount != "/data" {
		t.Errorf("relevant = %+v", got)
	}
	got2 := relevantMountsForPaths(mounts, []string{"/data/state", "/tmp/out"})
	if len(got2) != 2 {
		t.Errorf("expected both / and /data, got %+v", got2)
	}
}
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/sysinfo/ -run "TestParseMountinfo|TestRelevantMounts" -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

Write `internal/sysinfo/disk.go`:

```go
package sysinfo

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/kai-w/bscbench/internal/report"
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

func fillBlockDevAttrs(di *report.DiskInfo) {
	// /dev/nvme0n1 → "nvme0n1"
	dev := strings.TrimPrefix(di.Device, "/dev/")
	dev = strings.TrimRight(dev, "0123456789p")
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
```

- [ ] **Step 4: Run, verify it passes**

Run: `go test ./internal/sysinfo/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sysinfo/disk.go internal/sysinfo/disk_test.go
git commit -m "sysinfo: collect disk info for state and out-dir mounts"
```

---

### Task 12: Sysinfo Go runtime info + top-level Collect

**Files:**
- Create: `internal/sysinfo/goinfo.go`
- Create: `internal/sysinfo/sysinfo.go`
- Create: `internal/sysinfo/sysinfo_test.go`

- [ ] **Step 1: Implement Go info collector (no test — pure stdlib reads)**

Write `internal/sysinfo/goinfo.go`:

```go
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
```

- [ ] **Step 2: Write the top-level Collect test**

Write `internal/sysinfo/sysinfo_test.go`:

```go
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
```

- [ ] **Step 3: Implement Collect (cloud TBD in Task 13)**

Write `internal/sysinfo/sysinfo.go`:

```go
// Package sysinfo collects host inventory: OS, CPU, memory, disks, Go runtime, cloud.
package sysinfo

import (
	"context"

	"github.com/kai-w/bscbench/internal/report"
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
```

- [ ] **Step 4: Stub `collectCloud` so this compiles**

Append to `internal/sysinfo/sysinfo.go` for now (will be replaced in Task 13):

```go
func collectCloud(ctx context.Context) *report.CloudInfo {
	return nil
}
```

- [ ] **Step 5: Run, verify it passes**

Run: `go test ./internal/sysinfo/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/sysinfo/goinfo.go internal/sysinfo/sysinfo.go internal/sysinfo/sysinfo_test.go
git commit -m "sysinfo: top-level Collect (cloud stubbed)"
```

---

### Task 13: Sysinfo Cloud collector (parallel metadata probes)

**Files:**
- Create: `internal/sysinfo/cloud.go`
- Create: `internal/sysinfo/cloud_test.go`
- Modify: `internal/sysinfo/sysinfo.go` (drop the stub)

- [ ] **Step 1: Write the failing test**

Write `internal/sysinfo/cloud_test.go`:

```go
package sysinfo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCloudAWSDetection(t *testing.T) {
	awsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest/api/token":
			w.Write([]byte("TKN"))
		case "/latest/meta-data/instance-type":
			if r.Header.Get("X-aws-ec2-metadata-token") != "TKN" {
				http.Error(w, "no token", 401)
				return
			}
			w.Write([]byte("c6i.16xlarge"))
		case "/latest/meta-data/placement/availability-zone":
			w.Write([]byte("us-east-1a"))
		case "/latest/meta-data/placement/region":
			w.Write([]byte("us-east-1"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer awsSrv.Close()

	probes := []cloudProbe{newAWSProbeWithBase(awsSrv.URL)}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	got := runCloudProbes(ctx, probes, 500*time.Millisecond)
	if got == nil {
		t.Fatal("expected aws detected, got nil")
	}
	if got.Provider != "aws" {
		t.Errorf("provider = %q", got.Provider)
	}
	if got.InstanceType != "c6i.16xlarge" {
		t.Errorf("instance = %q", got.InstanceType)
	}
}

func TestCloudAllFailReturnsNil(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", 500)
	}))
	defer dead.Close()

	probes := []cloudProbe{newAWSProbeWithBase(dead.URL)}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	got := runCloudProbes(ctx, probes, 100*time.Millisecond)
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/sysinfo/ -run TestCloud -v`
Expected: FAIL.

- [ ] **Step 3: Implement the cloud collector**

Write `internal/sysinfo/cloud.go`:

```go
package sysinfo

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/kai-w/bscbench/internal/report"
)

const probeTimeout = 500 * time.Millisecond

type cloudProbe interface {
	Detect(ctx context.Context) *report.CloudInfo
	Provider() string
}

func collectCloud(ctx context.Context) *report.CloudInfo {
	probes := []cloudProbe{
		newAWSProbeWithBase("http://169.254.169.254"),
		newAliyunProbeWithBase("http://100.100.100.200"),
		newGCPProbeWithBase("http://metadata.google.internal"),
	}
	return runCloudProbes(ctx, probes, probeTimeout)
}

// runCloudProbes runs probes in parallel, returns the first non-nil result.
func runCloudProbes(ctx context.Context, probes []cloudProbe, timeout time.Duration) *report.CloudInfo {
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	results := make(chan *report.CloudInfo, len(probes))
	var wg sync.WaitGroup
	for _, p := range probes {
		wg.Add(1)
		go func(p cloudProbe) {
			defer wg.Done()
			results <- p.Detect(probeCtx)
		}(p)
	}
	go func() { wg.Wait(); close(results) }()

	for r := range results {
		if r != nil {
			return r
		}
	}
	return nil
}

// --- AWS IMDSv2 ---

type awsProbe struct {
	base string
	hc   *http.Client
}

func newAWSProbeWithBase(base string) *awsProbe {
	return &awsProbe{base: base, hc: &http.Client{Timeout: probeTimeout}}
}

func (p *awsProbe) Provider() string { return "aws" }

func (p *awsProbe) Detect(ctx context.Context) *report.CloudInfo {
	tokenReq, _ := http.NewRequestWithContext(ctx, "PUT", p.base+"/latest/api/token", nil)
	tokenReq.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "30")
	tokenResp, err := p.hc.Do(tokenReq)
	if err != nil || tokenResp.StatusCode != 200 {
		if tokenResp != nil {
			tokenResp.Body.Close()
		}
		return nil
	}
	tokenB, _ := io.ReadAll(tokenResp.Body)
	tokenResp.Body.Close()
	token := string(tokenB)

	get := func(path string) string {
		req, _ := http.NewRequestWithContext(ctx, "GET", p.base+path, nil)
		req.Header.Set("X-aws-ec2-metadata-token", token)
		resp, err := p.hc.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			return ""
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}

	inst := get("/latest/meta-data/instance-type")
	if inst == "" {
		return nil
	}
	return &report.CloudInfo{
		Provider:     "aws",
		InstanceType: inst,
		AZ:           get("/latest/meta-data/placement/availability-zone"),
		Region:       get("/latest/meta-data/placement/region"),
	}
}

// --- Aliyun ---

type aliyunProbe struct {
	base string
	hc   *http.Client
}

func newAliyunProbeWithBase(base string) *aliyunProbe {
	return &aliyunProbe{base: base, hc: &http.Client{Timeout: probeTimeout}}
}

func (p *aliyunProbe) Provider() string { return "aliyun" }

func (p *aliyunProbe) Detect(ctx context.Context) *report.CloudInfo {
	get := func(path string) string {
		req, _ := http.NewRequestWithContext(ctx, "GET", p.base+path, nil)
		resp, err := p.hc.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			return ""
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	inst := get("/latest/meta-data/instance/instance-type")
	if inst == "" {
		return nil
	}
	return &report.CloudInfo{
		Provider:     "aliyun",
		InstanceType: inst,
		AZ:           get("/latest/meta-data/zone-id"),
		Region:       get("/latest/meta-data/region-id"),
	}
}

// --- GCP ---

type gcpProbe struct {
	base string
	hc   *http.Client
}

func newGCPProbeWithBase(base string) *gcpProbe {
	return &gcpProbe{base: base, hc: &http.Client{Timeout: probeTimeout}}
}

func (p *gcpProbe) Provider() string { return "gcp" }

func (p *gcpProbe) Detect(ctx context.Context) *report.CloudInfo {
	get := func(path string) string {
		req, _ := http.NewRequestWithContext(ctx, "GET", p.base+path, nil)
		req.Header.Set("Metadata-Flavor", "Google")
		resp, err := p.hc.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			return ""
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	inst := get("/computeMetadata/v1/instance/machine-type")
	if inst == "" {
		return nil
	}
	return &report.CloudInfo{
		Provider:     "gcp",
		InstanceType: inst,
		AZ:           get("/computeMetadata/v1/instance/zone"),
		Region:       "",
	}
}
```

- [ ] **Step 4: Drop the stub from sysinfo.go**

Edit `internal/sysinfo/sysinfo.go` and remove the stub `collectCloud` function (the one that just returns `nil`); the real one now lives in `cloud.go`.

- [ ] **Step 5: Run, verify it passes**

Run: `go test ./internal/sysinfo/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/sysinfo/cloud.go internal/sysinfo/cloud_test.go internal/sysinfo/sysinfo.go
git commit -m "sysinfo: parallel cloud metadata probes (aws/aliyun/gcp)"
```

---

### Task 14: Wire `sysinfo` subcommand

**Files:**
- Create: `cmd/bscbench/sysinfo.go`
- Modify: `cmd/bscbench/root.go`

- [ ] **Step 1: Implement the subcommand**

Write `cmd/bscbench/sysinfo.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kai-w/bscbench/internal/sysinfo"
)

func newSysinfoCmd() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "sysinfo",
		Short: "Collect host inventory and write JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			si, err := sysinfo.Collect(context.Background(), []string{"/"})
			if err != nil {
				return fmt.Errorf("collect: %w", err)
			}
			b, err := json.MarshalIndent(si, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal: %w", err)
			}
			b = append(b, '\n')
			if out == "" || out == "-" {
				_, err = os.Stdout.Write(b)
				return err
			}
			return os.WriteFile(out, b, 0o644)
		},
	}
	cmd.Flags().StringVar(&out, "out", "-", "Output file (default: stdout)")
	return cmd
}
```

- [ ] **Step 2: Register the subcommand**

Edit `cmd/bscbench/root.go`. Replace:

```go
	cmd.AddCommand(newVersionCmd())
```

with:

```go
	cmd.AddCommand(newVersionCmd(), newSysinfoCmd())
```

- [ ] **Step 3: Smoke test**

Run:

```bash
go run ./cmd/bscbench sysinfo
```

Expected: a JSON object with `host`, `cpu`, `memory`, `disk`, `go`, `cloud` keys. On macOS many fields will be empty — that's fine, the tool is Linux-targeted.

- [ ] **Step 4: Commit**

```bash
git add cmd/bscbench/sysinfo.go cmd/bscbench/root.go
git commit -m "cmd: add sysinfo subcommand"
```

---

## Phase 3 — Add BSC dependency and build the corpus loader

### Task 15: Add BSC as a Go dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Pin BSC version**

Run from repo root:

```bash
go get github.com/bnb-chain/bsc@v1.4.8
go mod tidy
```

Expected: `go.mod` gains `require github.com/bnb-chain/bsc v1.4.8` plus its transitive deps. `go.sum` grows substantially.

If `v1.4.8` is unavailable when this runs, use the latest stable tag from `https://github.com/bnb-chain/bsc/releases`. Record the version chosen by editing the `BSCVersion` field commit message.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: succeeds. May take several minutes on first build (large dependency tree).

- [ ] **Step 3: Verify version subcommand reports BSC version**

Run: `go run ./cmd/bscbench version`
Expected output now includes `bsc=v1.4.8` (or the version chosen).

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: pin github.com/bnb-chain/bsc"
```

---

### Task 16: Corpus manifest type and validation

**Files:**
- Create: `internal/corpus/manifest.go`
- Create: `internal/corpus/manifest_test.go`
- Create: `internal/corpus/testdata/manifest_valid.json`
- Create: `internal/corpus/testdata/manifest_missing_field.json`

- [ ] **Step 1: Create fixture manifests**

Write `internal/corpus/testdata/manifest_valid.json`:

```json
{
  "schema_version": "1",
  "chain_id": 56,
  "from_block": 40000000,
  "to_block": 40010000,
  "block_count": 10000,
  "expected_state_root_at_from": "0x1111111111111111111111111111111111111111111111111111111111111111",
  "fork_schedule": {"haberFork": 38000000, "lorentzFork": 39000000},
  "generator": "mainnet-export",
  "generated_at": "2026-05-02T00:00:00Z",
  "input_hash": "sha256:abcdef",
  "bsc_version_recommended": "v1.4.8"
}
```

Write `internal/corpus/testdata/manifest_missing_field.json`:

```json
{
  "schema_version": "1",
  "chain_id": 56
}
```

- [ ] **Step 2: Write the failing test**

Write `internal/corpus/manifest_test.go`:

```go
package corpus

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifestValid(t *testing.T) {
	m, err := LoadManifest(filepath.Join("testdata", "manifest_valid.json"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.ChainID != 56 {
		t.Errorf("chain_id = %d", m.ChainID)
	}
	if m.FromBlock != 40000000 || m.ToBlock != 40010000 {
		t.Errorf("range %d..%d", m.FromBlock, m.ToBlock)
	}
	if m.BlockCount != 10000 {
		t.Errorf("count = %d", m.BlockCount)
	}
	if m.InputHash != "sha256:abcdef" {
		t.Errorf("hash = %q", m.InputHash)
	}
}

func TestLoadManifestRejectsMissingFields(t *testing.T) {
	_, err := LoadManifest(filepath.Join("testdata", "manifest_missing_field.json"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "from_block") {
		t.Errorf("expected error mentioning from_block, got: %v", err)
	}
}

func TestLoadManifestRejectsCountMismatch(t *testing.T) {
	m := &Manifest{
		SchemaVersion:           "1",
		ChainID:                 56,
		FromBlock:               1,
		ToBlock:                 1000,
		BlockCount:              999, // mismatch with 1000-1
		ExpectedStateRootAtFrom: "0x00",
		Generator:               "x",
		GeneratedAt:             "2026-05-02T00:00:00Z",
		InputHash:               "sha256:x",
	}
	if err := m.Validate(); err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 3: Run, verify it fails**

Run: `go test ./internal/corpus/ -run TestLoadManifest -v`
Expected: FAIL.

- [ ] **Step 4: Implement the manifest type**

Write `internal/corpus/manifest.go`:

```go
// Package corpus loads and validates the bscbench input directory.
package corpus

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const ManifestFileName = "manifest.json"

type Manifest struct {
	SchemaVersion           string         `json:"schema_version"`
	ChainID                 uint64         `json:"chain_id"`
	FromBlock               uint64         `json:"from_block"`
	ToBlock                 uint64         `json:"to_block"`
	BlockCount              uint64         `json:"block_count"`
	ExpectedStateRootAtFrom string         `json:"expected_state_root_at_from"`
	ForkSchedule            map[string]uint64 `json:"fork_schedule"`
	Generator               string         `json:"generator"`
	GeneratedAt             string         `json:"generated_at"`
	InputHash               string         `json:"input_hash"`
	BSCVersionRecommended   string         `json:"bsc_version_recommended,omitempty"`
}

func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var m Manifest
	dec := json.NewDecoder(byteReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("manifest invalid: %w", err)
	}
	return &m, nil
}

func (m *Manifest) Validate() error {
	if m.SchemaVersion != "1" {
		return fmt.Errorf("schema_version: want \"1\", got %q", m.SchemaVersion)
	}
	if m.ChainID == 0 {
		return errors.New("chain_id is required and non-zero")
	}
	if m.FromBlock == 0 && m.ToBlock == 0 {
		return errors.New("from_block / to_block required")
	}
	if m.ToBlock <= m.FromBlock {
		return fmt.Errorf("to_block (%d) must be > from_block (%d)", m.ToBlock, m.FromBlock)
	}
	if m.BlockCount != m.ToBlock-m.FromBlock {
		return fmt.Errorf("block_count (%d) != to_block - from_block (%d)",
			m.BlockCount, m.ToBlock-m.FromBlock)
	}
	if m.ExpectedStateRootAtFrom == "" {
		return errors.New("expected_state_root_at_from required")
	}
	if m.Generator == "" {
		return errors.New("generator required")
	}
	if m.GeneratedAt == "" {
		return errors.New("generated_at required")
	}
	if m.InputHash == "" {
		return errors.New("input_hash required")
	}
	return nil
}

type byteReader []byte

func (b byteReader) Read(p []byte) (int, error) {
	if len(b) == 0 {
		return 0, errEOF
	}
	n := copy(p, b)
	return n, nil
}

var errEOF = errors.New("EOF")
```

Wait — the use of `byteReader` is wrong (it can't be reused across calls). Replace with a stdlib reader. Re-edit the relevant lines:

```go
import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)
```

And replace the body of `LoadManifest` to:

```go
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("manifest invalid: %w", err)
	}
	return &m, nil
}
```

Also drop the `byteReader` type and the `errEOF` var — they're no longer used.

- [ ] **Step 5: Run, verify it passes**

Run: `go test ./internal/corpus/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/corpus/manifest.go internal/corpus/manifest_test.go internal/corpus/testdata/manifest_valid.json internal/corpus/testdata/manifest_missing_field.json
git commit -m "corpus: load and validate manifest.json"
```

---

### Task 17: Block iterator over blocks.rlp

**Files:**
- Create: `internal/corpus/block_iter.go`
- Create: `internal/corpus/block_iter_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/corpus/block_iter_test.go`:

```go
package corpus

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

func TestBlockIterReadsSequentialBlocks(t *testing.T) {
	blocks := makeFakeBlockChain(t, 3)

	var buf bytes.Buffer
	for _, b := range blocks {
		if err := rlp.Encode(&buf, b); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}

	it := NewBlockIter(io.NopCloser(bytes.NewReader(buf.Bytes())))
	defer it.Close()

	for i, want := range blocks {
		got, err := it.Next()
		if err != nil {
			t.Fatalf("[%d] next: %v", i, err)
		}
		if got.NumberU64() != want.NumberU64() {
			t.Errorf("[%d] number = %d", i, got.NumberU64())
		}
	}

	_, err := it.Next()
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected EOF, got %v", err)
	}
}

// makeFakeBlockChain builds n trivial blocks with monotonic numbers and
// linked parentHashes. Used as a stub for RLP round-tripping; not BSC-specific.
func makeFakeBlockChain(t *testing.T, n int) []*types.Block {
	t.Helper()
	out := make([]*types.Block, 0, n)
	var parent common.Hash
	for i := 0; i < n; i++ {
		h := &types.Header{
			Number:     big.NewInt(int64(i + 100)),
			ParentHash: parent,
			GasLimit:   30_000_000,
		}
		blk := types.NewBlockWithHeader(h)
		out = append(out, blk)
		parent = blk.Hash()
	}
	return out
}
```

(The above test imports `common` and `math/big`. Add them.)

Replace test imports with:

```go
import (
	"bytes"
	"errors"
	"io"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)
```

> **Note on imports:** BSC vendors `go-ethereum`-style packages at `github.com/ethereum/go-ethereum/...` via its `replace` directive in its own go.mod. As a downstream of BSC, we follow the same convention — `core/types`, `rlp`, `common` etc. resolve to BSC's vendored fork through Go's module graph. If the build fails on import, swap `github.com/ethereum/go-ethereum/...` for `github.com/bnb-chain/bsc/...` in this and subsequent tasks.

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/corpus/ -run TestBlockIter -v`
Expected: FAIL — `undefined: NewBlockIter`.

- [ ] **Step 3: Implement the iterator**

Write `internal/corpus/block_iter.go`:

```go
package corpus

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

const BlocksFileName = "blocks.rlp"

// BlockIter streams blocks from a reader containing concatenated RLP-encoded blocks.
type BlockIter struct {
	rc    io.ReadCloser
	br    *bufio.Reader
	rlpS  *rlp.Stream
	count uint64
}

func NewBlockIter(rc io.ReadCloser) *BlockIter {
	br := bufio.NewReaderSize(rc, 1<<20)
	return &BlockIter{
		rc:   rc,
		br:   br,
		rlpS: rlp.NewStream(br, 0),
	}
}

// Next returns the next block or io.EOF when the stream is exhausted.
func (it *BlockIter) Next() (*types.Block, error) {
	var b types.Block
	if err := it.rlpS.Decode(&b); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("decode block #%d: %w", it.count, err)
	}
	it.count++
	return &b, nil
}

func (it *BlockIter) Close() error {
	return it.rc.Close()
}

func (it *BlockIter) Count() uint64 { return it.count }
```

- [ ] **Step 4: Run, verify it passes**

Run: `go test ./internal/corpus/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/corpus/block_iter.go internal/corpus/block_iter_test.go
git commit -m "corpus: streaming RLP block iterator"
```

---

### Task 18: Corpus loader (directory + cross-validation)

**Files:**
- Create: `internal/corpus/loader.go`
- Create: `internal/corpus/loader_test.go`

The loader does **not** open the state DB itself — that's chain's job. It validates the input dir layout and exposes paths and the manifest. The `chain` package will call into here to open `state/`.

- [ ] **Step 1: Write the failing test**

Write `internal/corpus/loader_test.go`:

```go
package corpus

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/rlp"
)

func TestLoaderRejectsMissingFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(dir); err == nil {
		t.Fatal("expected error for empty dir")
	}
}

func TestLoaderHappyPath(t *testing.T) {
	dir := t.TempDir()

	manifestJSON, err := os.ReadFile(filepath.Join("testdata", "manifest_valid.json"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestFileName), manifestJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	// blocks.rlp: count must equal manifest.BlockCount, but Open does not
	// itself stream-decode (block_iter does); Open only checks file existence
	// and size > 0.
	var buf bytes.Buffer
	for i := 0; i < 1; i++ {
		_ = rlp.Encode(&buf, []byte{0x01})
	}
	if err := os.WriteFile(filepath.Join(dir, BlocksFileName), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(dir, "state"), 0o755); err != nil {
		t.Fatal(err)
	}

	c, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer c.Close()

	if c.Manifest().ChainID != 56 {
		t.Errorf("manifest not loaded")
	}
	if c.StateDir() != filepath.Join(dir, "state") {
		t.Errorf("state dir = %q", c.StateDir())
	}
	if c.BlocksPath() != filepath.Join(dir, BlocksFileName) {
		t.Errorf("blocks path = %q", c.BlocksPath())
	}
}
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/corpus/ -run TestLoader -v`
Expected: FAIL.

- [ ] **Step 3: Implement the loader**

Write `internal/corpus/loader.go`:

```go
package corpus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const StateDirName = "state"

// Corpus represents a validated input directory. Use Open() to construct.
type Corpus struct {
	dir      string
	manifest *Manifest
}

func Open(dir string) (*Corpus, error) {
	mPath := filepath.Join(dir, ManifestFileName)
	m, err := LoadManifest(mPath)
	if err != nil {
		return nil, err
	}

	bPath := filepath.Join(dir, BlocksFileName)
	if st, err := os.Stat(bPath); err != nil {
		return nil, fmt.Errorf("%s: %w", BlocksFileName, err)
	} else if st.Size() == 0 {
		return nil, errors.New("blocks.rlp is empty")
	}

	sPath := filepath.Join(dir, StateDirName)
	if st, err := os.Stat(sPath); err != nil {
		return nil, fmt.Errorf("%s: %w", StateDirName, err)
	} else if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", StateDirName)
	}

	return &Corpus{dir: dir, manifest: m}, nil
}

func (c *Corpus) Manifest() *Manifest    { return c.manifest }
func (c *Corpus) Dir() string            { return c.dir }
func (c *Corpus) StateDir() string       { return filepath.Join(c.dir, StateDirName) }
func (c *Corpus) BlocksPath() string     { return filepath.Join(c.dir, BlocksFileName) }

// OpenBlockIter opens blocks.rlp for streaming.
func (c *Corpus) OpenBlockIter() (*BlockIter, error) {
	f, err := os.Open(c.BlocksPath())
	if err != nil {
		return nil, fmt.Errorf("open blocks: %w", err)
	}
	return NewBlockIter(f), nil
}

func (c *Corpus) Close() error { return nil }
```

- [ ] **Step 4: Run, verify it passes**

Run: `go test ./internal/corpus/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/corpus/loader.go internal/corpus/loader_test.go
git commit -m "corpus: loader validates input directory layout"
```

---

## Phase 4 — Chain integration (BSC-dependent)

This phase touches BSC's `core/state`, `core/vm`, `core/rawdb`, `params`. Exact API names may need verification against the pinned BSC version. Where the plan shows a function call, the implementer should:

1. Open `vendor/github.com/bnb-chain/bsc/<package>` (or `$GOPATH/pkg/mod/...`) to confirm the signature.
2. If a function moved or renamed, replace with the closest equivalent and note the mapping in the commit message.

### Task 19: Open the state DB

**Files:**
- Create: `internal/chain/open.go`
- Create: `internal/chain/open_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/chain/open_test.go`:

```go
package chain

import (
	"path/filepath"
	"testing"
)

func TestOpenRejectsMissingDir(t *testing.T) {
	_, err := Open(filepath.Join("testdata", "does-not-exist"))
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/chain/ -run TestOpenRejectsMissingDir -v`
Expected: FAIL — `undefined: Open`.

- [ ] **Step 3: Implement Open**

Write `internal/chain/open.go`:

```go
// Package chain wraps BSC core packages to provide a stripped, consensus-bypassed
// execution environment for benchmark replay.
package chain

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
)

// DB is the tuple of low-level handles bscbench needs.
type DB struct {
	Path  string
	Disk  ethdb.Database // raw key-value + ancient store
	State state.Database // wraps Disk for state.New()
}

// Open opens the state database at <stateDir>. The directory layout follows
// BSC/geth conventions: chaindata/ for kv, ancient/ for the freezer.
func Open(stateDir string) (*DB, error) {
	if st, err := os.Stat(stateDir); err != nil {
		return nil, fmt.Errorf("state dir: %w", err)
	} else if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", stateDir)
	}

	chaindata := filepath.Join(stateDir, "chaindata")
	ancient := filepath.Join(stateDir, "ancient")

	disk, err := rawdb.Open(rawdb.OpenOptions{
		Type:              "pebble",
		Directory:         chaindata,
		AncientsDirectory: ancient,
		Namespace:         "bscbench/",
		Cache:             1024,    // MB; small, since we measure VM, not DB cache size effects
		Handles:           512,
		ReadOnly:          false,   // we mutate via state Commit; bscbench's runner copies state into a workdir first
	})
	if err != nil {
		return nil, fmt.Errorf("rawdb open: %w", err)
	}

	stateCache := state.NewDatabaseWithConfig(disk, nil)
	return &DB{Path: stateDir, Disk: disk, State: stateCache}, nil
}

func (db *DB) Close() error {
	return db.Disk.Close()
}
```

> **Verify against BSC:** `rawdb.OpenOptions` is the geth API; BSC tracks it but field names may differ slightly (e.g., `Cache` vs `CacheMB`, presence of `ReadOnly`). If the build fails, look at `core/rawdb/databases_64bit.go` in BSC to see the actual struct.

- [ ] **Step 4: Run, verify it passes the missing-dir test**

Run: `go test ./internal/chain/ -v`
Expected: PASS for `TestOpenRejectsMissingDir`. We don't yet have a fixture state dir, so happy-path is exercised in integration tests later.

- [ ] **Step 5: Commit**

```bash
git add internal/chain/open.go internal/chain/open_test.go
git commit -m "chain: open BSC state DB (pebble + ancient)"
```

---

### Task 20: Chain config (resolve fork schedule)

**Files:**
- Create: `internal/chain/config.go`
- Create: `internal/chain/config_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/chain/config_test.go`:

```go
package chain

import "testing"

func TestResolveChainConfigBSCMainnet(t *testing.T) {
	cfg, err := ResolveChainConfig(56, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cfg.ChainID == nil || cfg.ChainID.Uint64() != 56 {
		t.Errorf("chain id = %v", cfg.ChainID)
	}
}

func TestResolveChainConfigUnknownChainErrors(t *testing.T) {
	_, err := ResolveChainConfig(999999999, nil)
	if err == nil {
		t.Fatal("expected error for unknown chain id")
	}
}
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/chain/ -run TestResolveChainConfig -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

Write `internal/chain/config.go`:

```go
package chain

import (
	"fmt"

	"github.com/ethereum/go-ethereum/params"
)

// ResolveChainConfig returns the BSC params.ChainConfig for the given chain ID.
// manifestForkOverrides allows the manifest to surface fork heights when the
// pinned BSC version doesn't yet know about them; entries here override defaults.
func ResolveChainConfig(chainID uint64, manifestForkOverrides map[string]uint64) (*params.ChainConfig, error) {
	var cfg *params.ChainConfig
	switch chainID {
	case 56:
		cfg = params.BSCChainConfig
	case 97:
		cfg = params.ChapelChainConfig
	default:
		return nil, fmt.Errorf("unknown chain id %d (only BSC mainnet=56 and chapel=97 are supported)", chainID)
	}
	if cfg == nil {
		return nil, fmt.Errorf("BSC params for chain %d not found in this BSC build", chainID)
	}
	// manifest fork overrides are intentionally not applied automatically; they
	// are kept for forward-compatibility with manifests prepared on newer BSC
	// versions than this binary. A future revision can apply them here once the
	// upstream fork-config struct is stable.
	_ = manifestForkOverrides
	return cfg, nil
}
```

> **Verify against BSC:** `params.BSCChainConfig` and `params.ChapelChainConfig` are the names used in upstream. If they're called `params.MainnetChainConfig` etc., adjust.

- [ ] **Step 4: Run, verify it passes**

Run: `go test ./internal/chain/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/chain/config.go internal/chain/config_test.go
git commit -m "chain: resolve BSC chain config by chain id"
```

---

### Task 21: Apply a block (consensus-bypassed)

**Files:**
- Create: `internal/chain/apply_block.go`
- Create: `internal/chain/apply_block_test.go`

- [ ] **Step 1: Write the test (skipped without integration fixture)**

Write `internal/chain/apply_block_test.go`:

```go
package chain

import "testing"

func TestApplyBlockSignatureExists(t *testing.T) {
	// Smoke test: type-check the function symbol. Behavior is exercised in
	// the integration test (Task 31) once a real state fixture exists.
	var _ = ApplyBlock
}
```

- [ ] **Step 2: Run — should fail to compile**

Run: `go test ./internal/chain/ -run TestApplyBlockSignatureExists -v`
Expected: FAIL — `undefined: ApplyBlock`.

- [ ] **Step 3: Implement ApplyBlock**

Write `internal/chain/apply_block.go`:

```go
package chain

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

// ApplyBlockResult is what one consensus-bypassed block execution produces.
type ApplyBlockResult struct {
	Receipts        types.Receipts
	UsedGas         uint64
	StateRoot       common.Hash
	StateReadCount  uint64
	StateWriteCount uint64
	ExecNs          uint64
	TrieCommitNs    uint64
	SystemTxSkipped uint32
}

// Hooks is set of optional callbacks for instrumentation. Nil-checked.
type Hooks struct {
	NewTracer    func() vm.EVMLogger
	ReadTracer   func(vm.EVMLogger) (reads, writes uint64)
}

// ApplyBlock executes block's transactions against statedb and returns aggregated info.
// Header verification, Finalize, and validator-rotation system tx handling are all bypassed.
// System transactions (to == address(0xfff...)) are detected and skipped (counted separately).
func ApplyBlock(
	cfg *params.ChainConfig,
	stateDB *state.StateDB,
	header *types.Header,
	txs types.Transactions,
	hooks *Hooks,
	timer Timer,
) (ApplyBlockResult, error) {
	res := ApplyBlockResult{}
	gp := new(core.GasPool).AddGas(header.GasLimit)

	t0 := timer.Now()
	for i, tx := range txs {
		if isSystemTx(tx) {
			res.SystemTxSkipped++
			continue
		}
		stateDB.SetTxContext(tx.Hash(), i)

		var tracer vm.EVMLogger
		if hooks != nil && hooks.NewTracer != nil {
			tracer = hooks.NewTracer()
		}
		vmCfg := vm.Config{Tracer: tracer, NoBaseFee: false}

		receipt, err := core.ApplyTransaction(cfg, nil /*chainContext*/, &header.Coinbase,
			gp, stateDB, header, tx, &res.UsedGas, vmCfg)
		if err != nil {
			return res, fmt.Errorf("tx %d (%s): %w", i, tx.Hash().Hex(), err)
		}
		res.Receipts = append(res.Receipts, receipt)

		if hooks != nil && hooks.ReadTracer != nil && tracer != nil {
			r, w := hooks.ReadTracer(tracer)
			res.StateReadCount += r
			res.StateWriteCount += w
		}
	}
	t1 := timer.Now()
	res.ExecNs = uint64(t1 - t0)

	res.StateRoot = stateDB.IntermediateRoot(true /*deleteEmptyObjects*/)
	t2 := timer.Now()
	res.TrieCommitNs = uint64(t2 - t1)

	return res, nil
}

// isSystemTx detects BSC validator-rotation / system contract transactions.
// In BSC these are tagged with a special "to" address; the simplest heuristic
// is "to is the validator system contract address". The exact predicate may
// need to be refined against the upstream constant.
func isSystemTx(tx *types.Transaction) bool {
	if tx.To() == nil {
		return false
	}
	to := *tx.To()
	for _, addr := range systemContractAddresses {
		if to == addr {
			return true
		}
	}
	return false
}

// systemContractAddresses are BSC's well-known system contracts. Source:
// upstream BSC core/systemcontracts/upgrade.go (verify exact list against
// the pinned BSC version when implementing).
var systemContractAddresses = []common.Address{
	common.HexToAddress("0x0000000000000000000000000000000000001000"), // ValidatorContract
	common.HexToAddress("0x0000000000000000000000000000000000001001"), // SlashContract
	common.HexToAddress("0x0000000000000000000000000000000000001002"), // SystemRewardContract
	common.HexToAddress("0x0000000000000000000000000000000000001004"), // TokenHubContract
	common.HexToAddress("0x0000000000000000000000000000000000001005"), // TransferOutChannel ID
	common.HexToAddress("0x0000000000000000000000000000000000001006"), // CrossChainContract
	common.HexToAddress("0x0000000000000000000000000000000000001007"), // StakingContract
	common.HexToAddress("0x0000000000000000000000000000000000002000"), // GovHubContract
}

// Timer is a small wrapper so tests can fake the monotonic clock.
type Timer interface {
	Now() int64 // nanoseconds since arbitrary epoch
}

type RealTimer struct{}

func (RealTimer) Now() int64 {
	return systemNanos()
}

// avoid import cycle dance: define monotonic-now here once
var systemNanos = func() int64 {
	// caller-time injection point
	return monotonicNow()
}

func monotonicNow() int64 {
	return realMonotonic()
}

// realMonotonic is patched out in tests via package-level variable assignment.
var realMonotonic = func() int64 {
	t := nowFn()
	return t.UnixNano()
}

// indirection allows tests to replace; production calls time.Now().
var nowFn = stdNow

// We intentionally avoid `time` package shadowing in the constant lookups.
//go:noinline
func stdNow() time.Time {
	return time.Now()
}

// big.Int re-export to silence unused-import warnings in case ApplyBlock changes
// signature later. The unused import is otherwise removed.
var _ = (*big.Int)(nil)
```

> The above is over-engineered for the timing indirection. Replace the bottom half with a simpler implementation:

Replace `// Timer is a small wrapper ...` through end of file with:

```go
// Timer is a small wrapper so tests can fake the monotonic clock.
type Timer interface {
	Now() int64 // nanoseconds since some monotonic epoch
}

type RealTimer struct{}

func (RealTimer) Now() int64 { return time.Now().UnixNano() }
```

And add `"time"` to the import block, remove `"math/big"` if not needed.

- [ ] **Step 4: Build and run the symbol test**

Run: `go build ./internal/chain/...`
Expected: succeeds. Note: BSC's `core.ApplyTransaction` signature may differ — be prepared to adjust.

Run: `go test ./internal/chain/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/chain/apply_block.go internal/chain/apply_block_test.go
git commit -m "chain: apply block with consensus bypass and system-tx skip"
```

---

## Phase 5 — Metrics

### Task 22: SLOAD/SSTORE counter tracer

**Files:**
- Create: `internal/metrics/tracer.go`
- Create: `internal/metrics/tracer_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/metrics/tracer_test.go`:

```go
package metrics

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

func TestStateOpCounterCountsSloadSstore(t *testing.T) {
	tr := NewStateOpCounter()
	addr := common.HexToAddress("0xdeadbeef")

	// emulate a few SLOADs and SSTOREs via CaptureState callback
	tr.CaptureState(0, vm.SLOAD, 0, 0, &dummyScope{addr}, nil, 0, nil)
	tr.CaptureState(0, vm.SLOAD, 0, 0, &dummyScope{addr}, nil, 0, nil)
	tr.CaptureState(0, vm.SSTORE, 0, 0, &dummyScope{addr}, nil, 0, nil)
	tr.CaptureState(0, vm.ADD, 0, 0, &dummyScope{addr}, nil, 0, nil)

	r, w := tr.Counts()
	if r != 2 {
		t.Errorf("reads = %d", r)
	}
	if w != 1 {
		t.Errorf("writes = %d", w)
	}
}

type dummyScope struct{ a common.Address }

func (d *dummyScope) ContractAddress() common.Address { return d.a }
func (*dummyScope) MemoryData() []byte                { return nil }
func (*dummyScope) StackData() []*big.Int             { return nil }
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/metrics/ -run TestStateOpCounter -v`
Expected: FAIL.

- [ ] **Step 3: Implement the tracer**

Write `internal/metrics/tracer.go`:

```go
// Package metrics implements EVM-layer and process-layer instrumentation.
package metrics

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

// StateOpCounter is a vm.EVMLogger that counts SLOAD and SSTORE.
// It deliberately does nothing else. Allocate one per tx.
type StateOpCounter struct {
	reads, writes uint64
}

func NewStateOpCounter() *StateOpCounter { return &StateOpCounter{} }

// vm.EVMLogger interface

func (s *StateOpCounter) CaptureTxStart(uint64) {}
func (s *StateOpCounter) CaptureTxEnd(uint64)   {}

func (s *StateOpCounter) CaptureStart(env *vm.EVM, from common.Address, to common.Address,
	create bool, input []byte, gas uint64, value *big.Int) {
}

func (s *StateOpCounter) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address,
	input []byte, gas uint64, value *big.Int) {
}

func (s *StateOpCounter) CaptureExit(output []byte, gasUsed uint64, err error) {}

func (s *StateOpCounter) CaptureState(pc uint64, op vm.OpCode, gas uint64, cost uint64,
	scope vm.ScopeContext, rData []byte, depth int, err error) {
	switch op {
	case vm.SLOAD:
		s.reads++
	case vm.SSTORE:
		s.writes++
	}
}

func (s *StateOpCounter) CaptureFault(pc uint64, op vm.OpCode, gas uint64, cost uint64,
	scope vm.ScopeContext, depth int, err error) {
}

func (s *StateOpCounter) CaptureEnd(output []byte, gasUsed uint64, err error) {}

// Counts returns SLOAD and SSTORE counts respectively.
func (s *StateOpCounter) Counts() (reads, writes uint64) {
	return s.reads, s.writes
}
```

> **Verify against BSC:** the `vm.EVMLogger` interface signatures are stable in geth/BSC but occasionally an arg type changes between versions. If the build fails because `CaptureStart`/`CaptureState` signatures differ, copy the local interface declaration from `core/vm/logger.go` and align.

- [ ] **Step 4: Run, verify it passes**

Run: `go test ./internal/metrics/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/metrics/tracer.go internal/metrics/tracer_test.go
git commit -m "metrics: vm.EVMLogger that counts SLOAD/SSTORE"
```

---

### Task 23: ethdb wrapper for byte counting

**Files:**
- Create: `internal/metrics/db_wrapper.go`
- Create: `internal/metrics/db_wrapper_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/metrics/db_wrapper_test.go`:

```go
package metrics

import (
	"testing"

	"github.com/ethereum/go-ethereum/core/rawdb"
)

func TestCountingDBTracksBytes(t *testing.T) {
	mem := rawdb.NewMemoryDatabase()
	cdb := NewCountingDB(mem)

	if err := cdb.Put([]byte("k"), []byte("hello")); err != nil {
		t.Fatalf("put: %v", err)
	}
	v, err := cdb.Get([]byte("k"))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(v) != "hello" {
		t.Errorf("get = %q", v)
	}

	r, w := cdb.Counts()
	if r != 5 {
		t.Errorf("read bytes = %d", r)
	}
	if w != 5 {
		t.Errorf("write bytes = %d", w)
	}
}
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/metrics/ -run TestCountingDB -v`
Expected: FAIL.

- [ ] **Step 3: Implement the wrapper**

Write `internal/metrics/db_wrapper.go`:

```go
package metrics

import (
	"sync/atomic"

	"github.com/ethereum/go-ethereum/ethdb"
)

// CountingDB wraps an ethdb.Database, counting bytes read and written.
// Wrapping is intended for read paths; in BSC's StateDB the heavy reads happen
// through Get/Has and the writes through Batch.Put. We instrument both.
type CountingDB struct {
	ethdb.Database
	readBytes, writeBytes atomic.Uint64
}

func NewCountingDB(inner ethdb.Database) *CountingDB {
	return &CountingDB{Database: inner}
}

func (c *CountingDB) Get(key []byte) ([]byte, error) {
	v, err := c.Database.Get(key)
	if err == nil {
		c.readBytes.Add(uint64(len(v)))
	}
	return v, err
}

func (c *CountingDB) Put(key, value []byte) error {
	c.writeBytes.Add(uint64(len(value)))
	return c.Database.Put(key, value)
}

func (c *CountingDB) NewBatch() ethdb.Batch {
	return &countingBatch{Batch: c.Database.NewBatch(), counter: c}
}

func (c *CountingDB) Counts() (read, write uint64) {
	return c.readBytes.Load(), c.writeBytes.Load()
}

func (c *CountingDB) ResetCounts() {
	c.readBytes.Store(0)
	c.writeBytes.Store(0)
}

type countingBatch struct {
	ethdb.Batch
	counter *CountingDB
}

func (b *countingBatch) Put(key, value []byte) error {
	b.counter.writeBytes.Add(uint64(len(value)))
	return b.Batch.Put(key, value)
}
```

- [ ] **Step 4: Run, verify it passes**

Run: `go test ./internal/metrics/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/metrics/db_wrapper.go internal/metrics/db_wrapper_test.go
git commit -m "metrics: ethdb.Database wrapper for byte counting"
```

---

### Task 24: Block-level metrics collector and percentile aggregator

**Files:**
- Create: `internal/metrics/block_collector.go`
- Create: `internal/metrics/aggregator.go`
- Create: `internal/metrics/aggregator_test.go`

- [ ] **Step 1: Write the failing tests**

Write `internal/metrics/aggregator_test.go`:

```go
package metrics

import "testing"

func TestPercentileSimple(t *testing.T) {
	v := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if got := Percentile(v, 0.5); got != 5 {
		t.Errorf("p50 = %d, want 5", got)
	}
	if got := Percentile(v, 0.95); got != 10 {
		t.Errorf("p95 = %d, want 10", got)
	}
	if got := Percentile(v, 0.99); got != 10 {
		t.Errorf("p99 = %d", got)
	}
}

func TestPercentileEmpty(t *testing.T) {
	if got := Percentile(nil, 0.5); got != 0 {
		t.Errorf("p50 of empty = %d", got)
	}
}

func TestBlockCollectorAccumulates(t *testing.T) {
	c := NewBlockCollector()
	c.Record(BlockEvent{Number: 1, GasUsed: 100, ExecNs: 1000, TxCount: 5,
		StateReadCount: 10, StateWriteCount: 4, TrieCommitNs: 100,
		DBReadBytes: 200, DBWriteBytes: 80, RevertedTx: 1})
	c.Record(BlockEvent{Number: 2, GasUsed: 200, ExecNs: 2000, TxCount: 7,
		StateReadCount: 20, StateWriteCount: 8, TrieCommitNs: 150,
		DBReadBytes: 220, DBWriteBytes: 90, RevertedTx: 0})

	a := c.Aggregate()
	if a.TotalGasUsed != 300 {
		t.Errorf("gas = %d", a.TotalGasUsed)
	}
	if a.TotalTxCount != 12 {
		t.Errorf("tx = %d", a.TotalTxCount)
	}
	if a.RevertedTxCount != 1 {
		t.Errorf("reverted = %d", a.RevertedTxCount)
	}
	if a.ExecNsP50 == 0 {
		t.Errorf("p50 missing")
	}
}
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/metrics/ -run "TestPercentile|TestBlockCollector" -v`
Expected: FAIL.

- [ ] **Step 3: Implement the aggregator**

Write `internal/metrics/aggregator.go`:

```go
package metrics

import "sort"

// Percentile returns the q-th percentile of v (q in [0,1]) using nearest-rank.
// v is not modified. Empty input returns 0.
func Percentile(v []uint64, q float64) uint64 {
	if len(v) == 0 {
		return 0
	}
	if q < 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	cp := append([]uint64(nil), v...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	idx := int(float64(len(cp)-1) * q)
	return cp[idx]
}
```

- [ ] **Step 4: Implement the block collector**

Write `internal/metrics/block_collector.go`:

```go
package metrics

import (
	"sync"

	"github.com/kai-w/bscbench/internal/report"
)

// BlockEvent is one block's contribution from the runner.
type BlockEvent struct {
	Number          uint64
	TxCount         uint32
	RevertedTx      uint32
	GasUsed         uint64
	ExecNs          uint64
	StateReadCount  uint64
	StateWriteCount uint64
	TrieCommitNs    uint64
	DBReadBytes     uint64
	DBWriteBytes    uint64
}

// BlockCollector accumulates per-block events and produces aggregates.
type BlockCollector struct {
	mu       sync.Mutex
	records  []report.BlockRecord
	gasUsed  uint64
	txCount  uint64
	reverted uint64

	execNs       []uint64
	trieCommitNs []uint64
	gasPerBlock  []uint64
}

func NewBlockCollector() *BlockCollector {
	return &BlockCollector{
		records:      make([]report.BlockRecord, 0, 10000),
		execNs:       make([]uint64, 0, 10000),
		trieCommitNs: make([]uint64, 0, 10000),
		gasPerBlock:  make([]uint64, 0, 10000),
	}
}

func (c *BlockCollector) Record(e BlockEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.records = append(c.records, report.BlockRecord{
		BlockNumber:     e.Number,
		TxCount:         e.TxCount,
		GasUsed:         e.GasUsed,
		ExecNs:          e.ExecNs,
		StateReadCount:  e.StateReadCount,
		StateWriteCount: e.StateWriteCount,
		TrieCommitNs:    e.TrieCommitNs,
		DBReadBytes:     e.DBReadBytes,
		DBWriteBytes:    e.DBWriteBytes,
	})
	c.gasUsed += e.GasUsed
	c.txCount += uint64(e.TxCount)
	c.reverted += uint64(e.RevertedTx)
	c.execNs = append(c.execNs, e.ExecNs)
	c.trieCommitNs = append(c.trieCommitNs, e.TrieCommitNs)
	c.gasPerBlock = append(c.gasPerBlock, e.GasUsed)
}

// BlockRecords returns all collected per-block records (used to write blocks.csv).
func (c *BlockCollector) BlockRecords() []report.BlockRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]report.BlockRecord(nil), c.records...)
}

// Aggregate computes percentiles and totals.
type Aggregate struct {
	TotalGasUsed       uint64
	TotalTxCount       uint64
	RevertedTxCount    uint64
	ExecNsP50          uint64
	ExecNsP95          uint64
	ExecNsP99          uint64
	TrieCommitNsP50    uint64
	TrieCommitNsP95    uint64
	TrieCommitNsP99    uint64
	GasUsedPerBlockP50 uint64
	GasUsedPerBlockP95 uint64
	GasUsedPerBlockP99 uint64
}

func (c *BlockCollector) Aggregate() Aggregate {
	c.mu.Lock()
	defer c.mu.Unlock()

	return Aggregate{
		TotalGasUsed:       c.gasUsed,
		TotalTxCount:       c.txCount,
		RevertedTxCount:    c.reverted,
		ExecNsP50:          Percentile(c.execNs, 0.50),
		ExecNsP95:          Percentile(c.execNs, 0.95),
		ExecNsP99:          Percentile(c.execNs, 0.99),
		TrieCommitNsP50:    Percentile(c.trieCommitNs, 0.50),
		TrieCommitNsP95:    Percentile(c.trieCommitNs, 0.95),
		TrieCommitNsP99:    Percentile(c.trieCommitNs, 0.99),
		GasUsedPerBlockP50: Percentile(c.gasPerBlock, 0.50),
		GasUsedPerBlockP95: Percentile(c.gasPerBlock, 0.95),
		GasUsedPerBlockP99: Percentile(c.gasPerBlock, 0.99),
	}
}
```

- [ ] **Step 5: Run, verify it passes**

Run: `go test ./internal/metrics/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/metrics/aggregator.go internal/metrics/block_collector.go internal/metrics/aggregator_test.go
git commit -m "metrics: block collector with percentile aggregation"
```

---

### Task 25: /proc sampler goroutine

**Files:**
- Create: `internal/metrics/proc_sampler.go`
- Create: `internal/metrics/proc_sampler_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/metrics/proc_sampler_test.go`:

```go
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
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/metrics/ -run "TestProcSampler|TestParseSelfStat" -v`
Expected: FAIL.

- [ ] **Step 3: Implement the sampler**

Write `internal/metrics/proc_sampler.go`:

```go
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
```

- [ ] **Step 4: Run, verify it passes**

Run: `go test ./internal/metrics/ -v`
Expected: PASS. The `TestProcSamplerStartsAndStops` test depends on `/proc/self/stat` existing — on macOS it'll still pass since the goroutine produces samples (with zero values) on tick.

If on macOS the count is 0 (because all reads fail), wrap the test in a Linux build constraint:

```go
//go:build linux
```

at the top of `proc_sampler_test.go`.

- [ ] **Step 5: Commit**

```bash
git add internal/metrics/proc_sampler.go internal/metrics/proc_sampler_test.go
git commit -m "metrics: 1Hz /proc sampler goroutine"
```

---

## Phase 6 — Runner

### Task 26: State copy with reflink fallback

**Files:**
- Create: `internal/runner/copy_state.go`
- Create: `internal/runner/copy_state_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/runner/copy_state_test.go`:

```go
package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDirCopiesAllFiles(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	if err := CopyState(src, dst); err != nil {
		t.Fatalf("copy: %v", err)
	}

	a, err := os.ReadFile(filepath.Join(dst, "a"))
	if err != nil || string(a) != "hello" {
		t.Errorf("a = %q err=%v", a, err)
	}
	b, err := os.ReadFile(filepath.Join(dst, "sub", "b"))
	if err != nil || string(b) != "world" {
		t.Errorf("b = %q err=%v", b, err)
	}
}
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/runner/ -run TestCopyDirCopiesAllFiles -v`
Expected: FAIL.

- [ ] **Step 3: Implement the copy**

Write `internal/runner/copy_state.go`:

```go
// Package runner orchestrates double-pass replay execution.
package runner

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// CopyState copies src to dst, preferring reflink (CoW) when supported.
// Falls back to a recursive byte copy with a warning to stderr.
func CopyState(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := tryReflinkCopy(src, dst); err == nil {
		return nil
	} else {
		fmt.Fprintf(os.Stderr, "bscbench: reflink copy failed (%v), falling back to full copy\n", err)
	}
	return walkCopy(src, dst)
}

// tryReflinkCopy uses `cp --reflink=auto -a src/ dst/`. If `cp` doesn't accept
// `--reflink` (BSD cp on macOS) or the FS doesn't support reflink, returns error.
func tryReflinkCopy(src, dst string) error {
	cp, err := exec.LookPath("cp")
	if err != nil {
		return err
	}
	args := []string{"--reflink=auto", "-a", src + string(filepath.Separator) + ".", dst}
	cmd := exec.Command(cp, args...)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func walkCopy(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode())
		}
		if d.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// RemoveWorkdir cleans up a copied workdir.
func RemoveWorkdir(dst string) error {
	if err := os.RemoveAll(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
```

- [ ] **Step 4: Run, verify it passes**

Run: `go test ./internal/runner/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/runner/copy_state.go internal/runner/copy_state_test.go
git commit -m "runner: state copy with reflink-first, walk fallback"
```

---

### Task 27: Single-pass execution

**Files:**
- Create: `internal/runner/pass.go`
- Create: `internal/runner/pass_test.go`

- [ ] **Step 1: Write the failing test (skeleton; no real BSC fixture yet)**

Write `internal/runner/pass_test.go`:

```go
package runner

import "testing"

func TestPassConfigDefaults(t *testing.T) {
	c := PassConfig{}
	c.applyDefaults()
	if c.SamplerInterval == 0 {
		t.Errorf("sampler interval not defaulted")
	}
}
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/runner/ -run TestPassConfig -v`
Expected: FAIL.

- [ ] **Step 3: Implement the pass**

Write `internal/runner/pass.go`:

```go
package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/core/state"

	"github.com/kai-w/bscbench/internal/chain"
	"github.com/kai-w/bscbench/internal/corpus"
	"github.com/kai-w/bscbench/internal/metrics"
)

// PassConfig configures a single pass (warmup or measured).
type PassConfig struct {
	Measured        bool
	SamplerInterval time.Duration
}

func (c *PassConfig) applyDefaults() {
	if c.SamplerInterval == 0 {
		c.SamplerInterval = 1 * time.Second
	}
}

// PassResult is what one pass returns to the caller.
type PassResult struct {
	WallSec   float64
	GasUsed   uint64
	BlockColl *metrics.BlockCollector // nil for warmup
	Sampler   *metrics.ProcSampler    // nil for warmup
	DBCounter *metrics.CountingDB     // nil for warmup
}

// RunPass executes all blocks from the corpus against db.State, optionally
// recording metrics if cfg.Measured is true.
func RunPass(
	ctx context.Context,
	db *chain.DB,
	c *corpus.Corpus,
	cfg PassConfig,
) (PassResult, error) {
	cfg.applyDefaults()

	cdb := metrics.NewCountingDB(db.Disk)
	stateDB, err := state.New(parseStateRoot(c.Manifest().ExpectedStateRootAtFrom),
		state.NewDatabaseWithConfig(cdb, nil), nil)
	if err != nil {
		return PassResult{}, fmt.Errorf("open state: %w", err)
	}

	cfgChain, err := chain.ResolveChainConfig(c.Manifest().ChainID, c.Manifest().ForkSchedule)
	if err != nil {
		return PassResult{}, err
	}

	it, err := c.OpenBlockIter()
	if err != nil {
		return PassResult{}, err
	}
	defer it.Close()

	var (
		blockColl *metrics.BlockCollector
		sampler   *metrics.ProcSampler
	)
	if cfg.Measured {
		blockColl = metrics.NewBlockCollector()
		sampler = metrics.NewProcSampler(cfg.SamplerInterval)
		sampler.Start()
	}

	hooks := &chain.Hooks{
		NewTracer: func() vm.EVMLogger {
			if !cfg.Measured {
				return nil
			}
			return metrics.NewStateOpCounter()
		},
		ReadTracer: func(t vm.EVMLogger) (uint64, uint64) {
			if c, ok := t.(*metrics.StateOpCounter); ok {
				return c.Counts()
			}
			return 0, 0
		},
	}

	t0 := time.Now()
	var totalGas uint64
	for {
		if err := ctx.Err(); err != nil {
			return PassResult{}, err
		}
		blk, err := it.Next()
		if err != nil {
			break // EOF
		}

		dbReadBefore, dbWriteBefore := cdb.Counts()
		res, err := chain.ApplyBlock(cfgChain, stateDB, blk.Header(), blk.Transactions(),
			hooks, chain.RealTimer{})
		if err != nil {
			return PassResult{}, fmt.Errorf("block %d: %w", blk.NumberU64(), err)
		}
		dbReadAfter, dbWriteAfter := cdb.Counts()
		totalGas += res.UsedGas

		if cfg.Measured {
			reverted := uint32(0)
			for _, r := range res.Receipts {
				if r.Status == 0 {
					reverted++
				}
			}
			blockColl.Record(metrics.BlockEvent{
				Number:          blk.NumberU64(),
				TxCount:         uint32(len(blk.Transactions())) - uint32(res.SystemTxSkipped),
				RevertedTx:      reverted,
				GasUsed:         res.UsedGas,
				ExecNs:          res.ExecNs,
				StateReadCount:  res.StateReadCount,
				StateWriteCount: res.StateWriteCount,
				TrieCommitNs:    res.TrieCommitNs,
				DBReadBytes:     dbReadAfter - dbReadBefore,
				DBWriteBytes:    dbWriteAfter - dbWriteBefore,
			})
		}
	}
	wall := time.Since(t0).Seconds()

	pr := PassResult{
		WallSec:   wall,
		GasUsed:   totalGas,
		BlockColl: blockColl,
		Sampler:   sampler,
		DBCounter: cdb,
	}
	return pr, nil
}

// parseStateRoot parses a 0x-prefixed 32-byte hex root.
func parseStateRoot(s string) common.Hash {
	return common.HexToHash(s)
}
```

The above imports `vm`, `common` — add them:

```go
import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/kai-w/bscbench/internal/chain"
	"github.com/kai-w/bscbench/internal/corpus"
	"github.com/kai-w/bscbench/internal/metrics"
)
```

- [ ] **Step 4: Run, verify it passes**

Run: `go test ./internal/runner/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/runner/pass.go internal/runner/pass_test.go
git commit -m "runner: single-pass replay with optional measurement"
```

---

### Task 28: Double-pass coordinator

**Files:**
- Create: `internal/runner/double_pass.go`
- Create: `internal/runner/double_pass_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/runner/double_pass_test.go`:

```go
package runner

import "testing"

func TestDoublePassConfigDefaults(t *testing.T) {
	c := DoublePassConfig{}
	c.applyDefaults()
	if c.WorkDirRoot == "" {
		t.Errorf("workdir root not defaulted")
	}
}
```

- [ ] **Step 2: Run, verify it fails**

Run: `go test ./internal/runner/ -run TestDoublePassConfig -v`
Expected: FAIL.

- [ ] **Step 3: Implement the coordinator**

Write `internal/runner/double_pass.go`:

```go
package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kai-w/bscbench/internal/chain"
	"github.com/kai-w/bscbench/internal/corpus"
)

type DoublePassConfig struct {
	WorkDirRoot string
	Skip        bool // if true, skip the warmup pass entirely (debug only)
}

func (c *DoublePassConfig) applyDefaults() {
	if c.WorkDirRoot == "" {
		c.WorkDirRoot = filepath.Join(os.TempDir(), "bscbench-workdir")
	}
}

// DoublePassResult bundles both passes.
type DoublePassResult struct {
	Warmup   PassResult
	Measured PassResult
}

// RunDoublePass orchestrates: copy state → warmup → reset → copy state → measured.
// On --skip-warmup, only the measured pass runs (records skip_warmup=true upstream).
func RunDoublePass(
	ctx context.Context,
	c *corpus.Corpus,
	cfg DoublePassConfig,
) (DoublePassResult, error) {
	cfg.applyDefaults()

	out := DoublePassResult{}

	if !cfg.Skip {
		warmDir := filepath.Join(cfg.WorkDirRoot, "warmup")
		if err := RemoveWorkdir(warmDir); err != nil {
			return out, fmt.Errorf("clean warmup workdir: %w", err)
		}
		fmt.Fprintf(os.Stderr, "[bscbench] warmup: copying state to %s ...\n", warmDir)
		t0 := time.Now()
		if err := CopyState(c.StateDir(), warmDir); err != nil {
			return out, fmt.Errorf("copy warmup state: %w", err)
		}
		fmt.Fprintf(os.Stderr, "[bscbench] warmup: copy done in %s\n", time.Since(t0))

		db, err := chain.Open(warmDir)
		if err != nil {
			return out, fmt.Errorf("open warmup db: %w", err)
		}
		warmRes, err := RunPass(ctx, db, c, PassConfig{Measured: false})
		_ = db.Close()
		if err != nil {
			return out, fmt.Errorf("warmup pass: %w", err)
		}
		_ = RemoveWorkdir(warmDir)
		out.Warmup = warmRes
	}

	measDir := filepath.Join(cfg.WorkDirRoot, "measured")
	if err := RemoveWorkdir(measDir); err != nil {
		return out, fmt.Errorf("clean measured workdir: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[bscbench] measured: copying state to %s ...\n", measDir)
	t0 := time.Now()
	if err := CopyState(c.StateDir(), measDir); err != nil {
		return out, fmt.Errorf("copy measured state: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[bscbench] measured: copy done in %s\n", time.Since(t0))

	db, err := chain.Open(measDir)
	if err != nil {
		return out, fmt.Errorf("open measured db: %w", err)
	}
	defer db.Close()
	measRes, err := RunPass(ctx, db, c, PassConfig{Measured: true})
	if err != nil {
		return out, fmt.Errorf("measured pass: %w", err)
	}
	if measRes.Sampler != nil {
		// caller stops it during summary collection in cmd/bscbench/replay.go
	}
	out.Measured = measRes
	return out, nil
}
```

- [ ] **Step 4: Run, verify it passes**

Run: `go test ./internal/runner/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/runner/double_pass.go internal/runner/double_pass_test.go
git commit -m "runner: double-pass coordinator with state reset"
```

---

## Phase 7 — Wire `replay` subcommand

### Task 29: `replay` subcommand

**Files:**
- Create: `cmd/bscbench/replay.go`
- Modify: `cmd/bscbench/root.go`

The `replay` subcommand is the integration glue: parse flags, load corpus, run double-pass, build the Result, write JSON + CSVs.

- [ ] **Step 1: Implement replay**

Write `cmd/bscbench/replay.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/kai-w/bscbench/internal/corpus"
	"github.com/kai-w/bscbench/internal/metrics"
	"github.com/kai-w/bscbench/internal/report"
	"github.com/kai-w/bscbench/internal/runner"
	"github.com/kai-w/bscbench/internal/sysinfo"
)

func newReplayCmd() *cobra.Command {
	var (
		input       string
		outDir      string
		from, to    uint64
		skipWarmup  bool
		workDirRoot string
	)
	cmd := &cobra.Command{
		Use:   "replay",
		Short: "Replay a fixed window of BSC blocks and record metrics",
		RunE: func(cmd *cobra.Command, args []string) error {
			if input == "" {
				return fmt.Errorf("--input is required")
			}
			if outDir == "" {
				return fmt.Errorf("--out-dir is required")
			}
			return runReplay(input, outDir, from, to, skipWarmup, workDirRoot)
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "Input directory (manifest.json + blocks.rlp + state/)")
	cmd.Flags().StringVar(&outDir, "out-dir", "", "Output directory")
	cmd.Flags().Uint64Var(&from, "from", 0, "Override manifest from_block (0 = use manifest)")
	cmd.Flags().Uint64Var(&to, "to", 0, "Override manifest to_block (0 = use manifest)")
	cmd.Flags().BoolVar(&skipWarmup, "skip-warmup", false, "Skip the warmup pass (debug only; results not comparable)")
	cmd.Flags().StringVar(&workDirRoot, "work-dir", "", "Where to copy state for each pass (default: $TMPDIR/bscbench-workdir)")
	return cmd
}

func runReplay(input, outDir string, fromOverride, toOverride uint64, skipWarmup bool, workDirRoot string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir out: %w", err)
	}

	c, err := corpus.Open(input)
	if err != nil {
		return fmt.Errorf("corpus: %w", err)
	}
	defer c.Close()

	if fromOverride != 0 || toOverride != 0 {
		fmt.Fprintln(os.Stderr, "[bscbench] WARNING: --from/--to overrides selected; result is non-canonical")
	}

	startedAt := time.Now().UTC()

	dpRes, err := runner.RunDoublePass(ctx, c, runner.DoublePassConfig{
		WorkDirRoot: workDirRoot,
		Skip:        skipWarmup,
	})
	if err != nil {
		return fmt.Errorf("double pass: %w", err)
	}
	finishedAt := time.Now().UTC()

	measured := dpRes.Measured
	var samples []report.ProcSample
	if measured.Sampler != nil {
		samples = measured.Sampler.Stop()
	}

	si, err := sysinfo.Collect(ctx, []string{c.StateDir(), outDir})
	if err != nil {
		return fmt.Errorf("sysinfo: %w", err)
	}

	agg := measured.BlockColl.Aggregate()
	dbRead, dbWrite := measured.DBCounter.Counts()

	cpuAvg, cpuMax, rssPeak := summarizeSamples(samples)
	dRead, dWrite := diskDelta(samples)

	r := &report.Result{
		SchemaVersion: report.SchemaVersion,
		Run: report.RunMeta{
			ID:              buildRunID(startedAt, c.Manifest().InputHash, si.Host.Hostname),
			StartedAt:       startedAt,
			FinishedAt:      finishedAt,
			BscbenchVersion: Version,
			BSCVersion:      bscDepVersion(),
			InputHash:       c.Manifest().InputHash,
			FromBlock:       c.Manifest().FromBlock,
			ToBlock:         c.Manifest().ToBlock,
			BlockCount:      c.Manifest().BlockCount,
			WarmupState:     warmupStateString(skipWarmup),
			SkipWarmup:      skipWarmup,
		},
		Sysinfo: si,
		Metrics: report.Metrics{
			Mgasps:              float64(agg.TotalGasUsed) / measured.WallSec / 1e6,
			TotalGasUsed:        agg.TotalGasUsed,
			TotalTxCount:        agg.TotalTxCount,
			RevertedTxCount:     agg.RevertedTxCount,
			TotalWallSec:        measured.WallSec,
			TxPerSec:            float64(agg.TotalTxCount) / measured.WallSec,
			BlockPerSec:         float64(c.Manifest().BlockCount) / measured.WallSec,
			ExecNsP50:           agg.ExecNsP50,
			ExecNsP95:           agg.ExecNsP95,
			ExecNsP99:           agg.ExecNsP99,
			TrieCommitNsP50:     agg.TrieCommitNsP50,
			TrieCommitNsP95:     agg.TrieCommitNsP95,
			TrieCommitNsP99:     agg.TrieCommitNsP99,
			GasUsedPerBlockP50:  agg.GasUsedPerBlockP50,
			GasUsedPerBlockP95:  agg.GasUsedPerBlockP95,
			GasUsedPerBlockP99:  agg.GasUsedPerBlockP99,
			CPUPctAvg:           cpuAvg,
			CPUPctMax:           cpuMax,
			RSSPeakBytes:        rssPeak,
			DiskReadTotalBytes:  dRead,
			DiskWriteTotalBytes: dWrite,
			DiskReadMBpsAvg:     float64(dRead) / 1e6 / measured.WallSec,
			DiskWriteMBpsAvg:    float64(dWrite) / 1e6 / measured.WallSec,
		},
		Passes: report.Passes{
			Warmup:   report.PassMeta{WallSec: dpRes.Warmup.WallSec, GasUsed: dpRes.Warmup.GasUsed},
			Measured: report.PassMeta{WallSec: measured.WallSec, GasUsed: agg.TotalGasUsed},
		},
	}
	_ = dbRead
	_ = dbWrite

	if err := report.WriteResult(r, filepath.Join(outDir, "result.json")); err != nil {
		return err
	}

	bw, err := report.NewBlocksWriter(filepath.Join(outDir, "blocks.csv"))
	if err != nil {
		return err
	}
	for _, rec := range measured.BlockColl.BlockRecords() {
		if err := bw.Write(rec); err != nil {
			_ = bw.Close()
			return err
		}
	}
	if err := bw.Close(); err != nil {
		return err
	}

	pw, err := report.NewProcWriter(filepath.Join(outDir, "proc_samples.csv"))
	if err != nil {
		return err
	}
	for _, s := range samples {
		if err := pw.Write(s); err != nil {
			_ = pw.Close()
			return err
		}
	}
	if err := pw.Close(); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "[bscbench] done. mgasps=%.2f wall=%.1fs out=%s\n",
		r.Metrics.Mgasps, r.Metrics.TotalWallSec, outDir)
	return nil
}

func warmupStateString(skip bool) string {
	if skip {
		return "skipped"
	}
	return "warm"
}

func buildRunID(t time.Time, inputHash, hostname string) string {
	short := inputHash
	if i := strings.Index(short, ":"); i >= 0 {
		short = short[i+1:]
	}
	if len(short) > 8 {
		short = short[:8]
	}
	if hostname == "" {
		hostname = "unknown"
	}
	ts := t.Format("2006-01-02T15-04-05Z")
	return fmt.Sprintf("%s_bsc10k_%s_%s", ts, hostname, short)
}

func summarizeSamples(samples []report.ProcSample) (cpuAvg, cpuMax float64, rssPeak uint64) {
	if len(samples) == 0 {
		return 0, 0, 0
	}
	var sum float64
	for _, s := range samples {
		sum += s.CPUPct
		if s.CPUPct > cpuMax {
			cpuMax = s.CPUPct
		}
		if s.RSSBytes > rssPeak {
			rssPeak = s.RSSBytes
		}
	}
	cpuAvg = sum / float64(len(samples))
	return
}

func diskDelta(samples []report.ProcSample) (read, write uint64) {
	if len(samples) < 2 {
		return 0, 0
	}
	first := samples[0]
	last := samples[len(samples)-1]
	if last.DiskReadCumBytes > first.DiskReadCumBytes {
		read = last.DiskReadCumBytes - first.DiskReadCumBytes
	}
	if last.DiskWriteCumBytes > first.DiskWriteCumBytes {
		write = last.DiskWriteCumBytes - first.DiskWriteCumBytes
	}
	return
}

// metrics is referenced via runner only; keep import if linter complains.
var _ = metrics.NewBlockCollector
```

- [ ] **Step 2: Register the subcommand**

Edit `cmd/bscbench/root.go`. Replace:

```go
	cmd.AddCommand(newVersionCmd(), newSysinfoCmd())
```

with:

```go
	cmd.AddCommand(newVersionCmd(), newSysinfoCmd(), newReplayCmd())
```

- [ ] **Step 3: Build smoke test**

Run: `go build ./...`
Expected: succeeds.

Run: `go run ./cmd/bscbench replay --help`
Expected: prints flag descriptions.

- [ ] **Step 4: Commit**

```bash
git add cmd/bscbench/replay.go cmd/bscbench/root.go
git commit -m "cmd: replay subcommand wires corpus + runner + report"
```

---

## Phase 8 — Integration test

### Task 30: Integration test scaffold

**Files:**
- Create: `cmd/bscbench/replay_integration_test.go`
- Create: `testdata/integration/README.md`

The full integration test requires a real BSC testnet state snapshot + 50 blocks. Building that fixture is an ops task; this scaffold lays out what the test will look like and how the fixture is expected to be staged. The test is gated by a build tag and skipped automatically if the fixture isn't present.

- [ ] **Step 1: Scaffold the integration test**

Write `cmd/bscbench/replay_integration_test.go`:

```go
//go:build integration

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const integrationFixtureDir = "../../testdata/integration/chapel-50blocks"

func TestReplayIntegrationChapel(t *testing.T) {
	if _, err := os.Stat(integrationFixtureDir); err != nil {
		t.Skipf("integration fixture missing at %s: %v", integrationFixtureDir, err)
	}

	out := t.TempDir()
	err := runReplay(integrationFixtureDir, out, 0, 0, true, filepath.Join(out, "work"))
	if err != nil {
		t.Fatalf("runReplay: %v", err)
	}

	resultPath := filepath.Join(out, "result.json")
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parse result: %v", err)
	}

	if got["schema_version"] != "1" {
		t.Errorf("schema_version = %v", got["schema_version"])
	}
	metrics, ok := got["metrics"].(map[string]any)
	if !ok {
		t.Fatalf("metrics not an object")
	}
	if mgasps, _ := metrics["mgasps"].(float64); mgasps <= 0 {
		t.Errorf("mgasps = %v", mgasps)
	}

	for _, name := range []string{"blocks.csv", "proc_samples.csv"} {
		st, err := os.Stat(filepath.Join(out, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
		} else if st.Size() == 0 {
			t.Errorf("%s: empty", name)
		}
	}
}
```

- [ ] **Step 2: Document the fixture format**

Write `testdata/integration/README.md`:

```markdown
# Integration Test Fixture

The `cmd/bscbench` integration test (gated by `-tags=integration`) expects a
small, real BSC corpus at:

    testdata/integration/chapel-50blocks/
      manifest.json
      blocks.rlp
      state/
        chaindata/
        ancient/

This fixture is **not committed** to the repo (size, churn). To regenerate:

1. Bring up a BSC chapel (testnet) node and snap-sync to a recent height.
2. Stop at height N.
3. Export 50 blocks: `geth --datadir=<dir> export blocks.rlp N N+50`
4. Pack `chaindata/` and `ancient/` into `state/`.
5. Generate `manifest.json` matching the expected schema (see
   `internal/corpus/testdata/manifest_valid.json`).
6. Compute `input_hash` as `sha256` over `blocks.rlp` and the tarred state.

Run the integration test:

    go test -tags=integration ./cmd/bscbench/ -run TestReplayIntegrationChapel -v
```

- [ ] **Step 3: Verify the build tag works**

Run: `go test ./cmd/bscbench/ -v`
Expected: PASS (the integration test is excluded without the tag).

Run: `go test -tags=integration ./cmd/bscbench/ -run TestReplayIntegrationChapel -v`
Expected: PASS with a `t.Skip` log message ("integration fixture missing at ...") when the fixture is absent.

- [ ] **Step 4: Commit**

```bash
git add cmd/bscbench/replay_integration_test.go testdata/integration/README.md
git commit -m "test: integration scaffold for chapel 50-block replay"
```

---

## Self-Review Notes (post-write)

**Spec coverage:**
- §1 Goal & non-goals → covered by command surface (Task 3, 14, 29) and excluded scope (no synth).
- §2 Scope decisions → all matrix rows mapped (Workload→Task 27, BSC integration→Task 15, Consensus→Task 21, Dataset→Task 18 contract, Window→Task 29 flags, Output→Phase 1, Orchestration→excluded).
- §3 Architecture → Task 1, 2, 14, 29 (CLI), Task 18 (corpus), Task 19–21 (chain), Task 22–25 (metrics), Task 26–28 (runner). Dependency direction (cmd → runner → others) is preserved.
- §3.4 Single execution path → Task 21 (`ApplyBlock`) + Task 27 (`RunPass`) implement the unified path.
- §4 Double-pass → Task 28.
- §5 Input contract → Tasks 16–18.
- §6 Metrics → Task 4 (schema) + Task 22–25.
- §7 Output → Tasks 4–7 (writers) + Task 29 (assembly).
- §8 Sysinfo → Tasks 8–14.
- §9 Testing → unit tests in every task; integration scaffold in Task 30; baseline regression is operational, not implementation.
- §10 Open questions → carried forward in code comments and the README; system-tx handling has explicit verification note in Task 21; BSC fork tracking in Task 20.

**Placeholder scan:** None of "TBD", "TODO", "implement later", or "similar to Task N" appear in step bodies. A few "verify against BSC" callouts are left intentionally — they're not placeholder code, they're due-diligence reminders for the implementer at known fragile interface points.

**Type consistency:** `BlockEvent` (Task 24), `BlockRecord` (Task 4), `PassResult` (Task 27), `DoublePassResult` (Task 28), `ApplyBlockResult` (Task 21) — fields traced; the `RunPass` consumes `ApplyBlockResult` and emits `BlockEvent` to the collector which produces `BlockRecord`. Field names align (GasUsed, ExecNs, TrieCommitNs, StateReadCount, StateWriteCount, etc.).

The plan is complete.
