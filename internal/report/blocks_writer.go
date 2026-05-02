package report

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

type BlocksWriter struct {
	f   *os.File
	bw  *bufio.Writer
	buf []byte
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
		return nil, fmt.Errorf("write header: %w", err)
	}
	return &BlocksWriter{f: f, bw: bw, buf: make([]byte, 0, 96)}, nil
}

func (w *BlocksWriter) Write(r BlockRecord) error {
	// hand-rolled to avoid encoding/csv overhead in a 10k-row hot loop
	buf := w.buf[:0]
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
	w.buf = buf
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
