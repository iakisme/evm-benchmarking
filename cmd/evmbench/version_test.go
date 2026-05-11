package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	cmd := newVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "evmbench") {
		t.Errorf("expected output to mention evmbench, got %q", out)
	}
	if !strings.Contains(out, "bsc=") {
		t.Errorf("expected output to include bsc dependency version, got %q", out)
	}
	if !strings.Contains(out, "go=") {
		t.Errorf("expected output to include go runtime version, got %q", out)
	}
}
