package sysinfo

import (
	"testing"
)

func TestParseUptime(t *testing.T) {
	got, err := parseUptime("48300.42 12000.10\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != 48300 {
		t.Errorf("uptime = %d, want 48300", got)
	}
}

func TestParseOSRelease(t *testing.T) {
	in := `NAME="Ubuntu"
VERSION="22.04.4 LTS (Jammy Jellyfish)"
ID=ubuntu
PRETTY_NAME="Ubuntu 22.04.4 LTS"
`
	got := parseOSRelease(in)
	want := "Ubuntu 22.04.4 LTS"
	if got != want {
		t.Errorf("os = %q, want %q", got, want)
	}
}
