package sysinfo

import "testing"

func TestParseMountinfoFiltersOurPaths(t *testing.T) {
	in := `38 27 259:1 / /data rw,relatime shared:13 - ext4 /dev/nvme0n1 rw
27 0 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw
40 27 0:21 / /proc rw,nosuid,nodev,noexec shared:14 - proc proc rw
`
	mounts := parseMountinfo(in)
	if len(mounts) != 2 {
		t.Fatalf("expected 2 real mounts, got %d", len(mounts))
	}
	if mounts[0].mount != "/data" {
		t.Errorf("[0].mount = %q", mounts[0].mount)
	}
	if mounts[0].source != "/dev/nvme0n1" {
		t.Errorf("[0].source = %q", mounts[0].source)
	}
	if mounts[0].fs != "ext4" {
		t.Errorf("[0].fs = %q", mounts[0].fs)
	}
}

func TestBlockDeviceName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"nvme0n1", "nvme0n1"},
		{"nvme0n1p3", "nvme0n1"},
		{"nvme0n1p15", "nvme0n1"},
		{"sda", "sda"},
		{"sda1", "sda"},
		{"sda12", "sda"},
		{"vda2", "vda"},
		{"mmcblk0", "mmcblk0"},
		{"mmcblk0p1", "mmcblk0"},
	}
	for _, c := range cases {
		if got := blockDeviceName(c.in); got != c.want {
			t.Errorf("blockDeviceName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRelevantMountsForPaths(t *testing.T) {
	mounts := []mountEntry{
		{mount: "/", source: "/dev/sda1", fs: "ext4"},
		{mount: "/data", source: "/dev/nvme0n1", fs: "ext4"},
	}
	got := relevantMountsForPaths(mounts, []string{"/data/state", "/data/out"})
	if len(got) != 1 || got[0].mount != "/data" {
		t.Errorf("relevant = %+v", got)
	}
	got2 := relevantMountsForPaths(mounts, []string{"/data/state", "/tmp/out"})
	if len(got2) != 2 {
		t.Errorf("expected both / and /data, got %+v", got2)
	}
}
