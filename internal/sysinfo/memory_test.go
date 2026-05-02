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
