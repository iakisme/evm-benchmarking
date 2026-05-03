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
