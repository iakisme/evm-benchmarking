package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/iakisme/evm-benchmarking/internal/chain"
	"github.com/iakisme/evm-benchmarking/internal/corpus"
)

// DoublePassConfig configures a double-pass benchmark run.
type DoublePassConfig struct {
	WorkDirRoot     string
	Skip            bool          // if true, skip the warmup pass entirely (debug only)
	SamplerInterval time.Duration // /proc sampler period; 0 falls back to PassConfig default (1 s)
}

func (c *DoublePassConfig) applyDefaults() {
	if c.WorkDirRoot == "" {
		c.WorkDirRoot = filepath.Join(os.TempDir(), "evmbench-workdir")
	}
}

// DoublePassResult bundles both passes.
type DoublePassResult struct {
	Warmup   PassResult
	Measured PassResult
}

// RunDoublePass orchestrates: copy state → warmup → reset → copy state → measured.
// On --skip-warmup, only the measured pass runs (records skip_warmup=true upstream).
//
// The measured workdir is intentionally left in place after the run so the caller
// (or the operator) can inspect it. It is NOT cleaned up here.
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
		fmt.Fprintf(os.Stderr, "[evmbench] warmup: copying state to %s ...\n", warmDir)
		t0 := time.Now()
		if err := CopyState(c.StateDir(), warmDir); err != nil {
			return out, fmt.Errorf("copy warmup state: %w", err)
		}
		fmt.Fprintf(os.Stderr, "[evmbench] warmup: copy done in %s\n", time.Since(t0))

		db, err := chain.Open(warmDir)
		if err != nil {
			return out, fmt.Errorf("open warmup db: %w", err)
		}
		warmRes, err := RunPass(ctx, db, c, PassConfig{Measured: false, SamplerInterval: cfg.SamplerInterval})
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
	fmt.Fprintf(os.Stderr, "[evmbench] measured: copying state to %s ...\n", measDir)
	t0 := time.Now()
	if err := CopyState(c.StateDir(), measDir); err != nil {
		return out, fmt.Errorf("copy measured state: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[evmbench] measured: copy done in %s\n", time.Since(t0))

	db, err := chain.Open(measDir)
	if err != nil {
		return out, fmt.Errorf("open measured db: %w", err)
	}
	defer db.Close()
	measRes, err := RunPass(ctx, db, c, PassConfig{Measured: true, SamplerInterval: cfg.SamplerInterval})
	if err != nil {
		return out, fmt.Errorf("measured pass: %w", err)
	}
	if measRes.Sampler != nil {
		// caller stops it during summary collection in cmd/evmbench/replay.go
	}
	out.Measured = measRes
	return out, nil
}
