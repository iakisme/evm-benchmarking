package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/triedb"

	"github.com/iakisme/evm-benchmarking/internal/chain"
	"github.com/iakisme/evm-benchmarking/internal/corpus"
	"github.com/iakisme/evm-benchmarking/internal/metrics"
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
	Sampler   *metrics.ProcSampler    // nil for warmup; caller must call Stop() to drain samples
	DBCounter *metrics.CountingDB     // always set; cumulative read/write byte counters
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
	tdb := triedb.NewDatabase(cdb, nil)
	stateDB, err := state.New(parseStateRoot(c.Manifest().ExpectedStateRootAtFrom),
		state.NewDatabase(tdb, nil))
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
		NewTracer: func() chain.EVMTracer {
			if !cfg.Measured {
				return nil
			}
			return metrics.NewStateOpCounter()
		},
	}

	t0 := time.Now()
	var totalGas uint64
	for {
		if err := ctx.Err(); err != nil {
			return PassResult{}, err
		}
		blk, err := it.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return PassResult{}, fmt.Errorf("read block %d: %w", it.Count(), err)
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
				Number: blk.NumberU64(),
				// ApplyBlock invariant: SystemTxSkipped <= len(Transactions()), no underflow.
				TxCount:         uint32(len(blk.Transactions())) - res.SystemTxSkipped,
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
