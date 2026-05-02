package sysinfo

import (
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestParseCPUInfo(t *testing.T) {
	data, err := os.ReadFile("testdata/proc_cpuinfo")
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	got := parseCPUInfo(string(data))

	if got.Model != "Intel(R) Xeon(R) Platinum 8375C CPU @ 2.90GHz" {
		t.Errorf("model = %q", got.Model)
	}
	if got.CoresLogical != 3 {
		t.Errorf("logical = %d, want 3", got.CoresLogical)
	}
	if got.CoresPhysical != 2 {
		t.Errorf("physical = %d, want 2", got.CoresPhysical)
	}
	if got.MhzBase != 2900.0 {
		t.Errorf("mhz = %v", got.MhzBase)
	}

	wantFlags := []string{"avx2", "avx512f", "sse4_2"}
	gotFlags := append([]string(nil), got.FlagsSubset...)
	sort.Strings(gotFlags)
	sort.Strings(wantFlags)
	if !reflect.DeepEqual(gotFlags, wantFlags) {
		t.Errorf("flags = %v, want %v", gotFlags, wantFlags)
	}
}
