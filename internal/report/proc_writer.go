package report

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

type ProcWriter struct {
	f   *os.File
	bw  *bufio.Writer
	buf []byte
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
		return nil, fmt.Errorf("write header: %w", err)
	}
	return &ProcWriter{f: f, bw: bw, buf: make([]byte, 0, 64)}, nil
}

func (w *ProcWriter) Write(s ProcSample) error {
	buf := w.buf[:0]
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
	w.buf = buf
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
