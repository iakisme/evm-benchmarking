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
