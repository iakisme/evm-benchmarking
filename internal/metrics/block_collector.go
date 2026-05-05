package metrics

import (
	"sync"

	"github.com/iakisme/evm-benchmarking/internal/report"
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

// NewBlockCollector allocates a BlockCollector with capacity for 10000 blocks.
func NewBlockCollector() *BlockCollector {
	return &BlockCollector{
		records:      make([]report.BlockRecord, 0, 10000),
		execNs:       make([]uint64, 0, 10000),
		trieCommitNs: make([]uint64, 0, 10000),
		gasPerBlock:  make([]uint64, 0, 10000),
	}
}

// Record appends a BlockEvent to the collector under lock.
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

// BlockRecords returns a copy of all collected per-block records (used to write blocks.csv).
func (c *BlockCollector) BlockRecords() []report.BlockRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]report.BlockRecord(nil), c.records...)
}

// Aggregate is a snapshot of totals and percentiles computed from collected events.
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

// Aggregate computes percentiles and totals from all recorded events.
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
