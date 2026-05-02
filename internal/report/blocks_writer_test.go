package report

import (
	"os"
	"path/filepath"
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
}
