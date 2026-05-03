package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/kai-w/bscbench/internal/corpus"
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

	cpuAvg, cpuMax, rssPeak := summarizeSamples(samples)
	dRead, dWrite := diskDelta(samples)

	bi, _ := debug.ReadBuildInfo()
	r := &report.Result{
		SchemaVersion: report.SchemaVersion,
		Run: report.RunMeta{
			ID:              buildRunID(startedAt, c.Manifest().InputHash, si.Host.Hostname),
			StartedAt:       startedAt,
			FinishedAt:      finishedAt,
			BscbenchVersion: Version,
			BSCVersion:      bscDepVersion(bi),
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
