package chain

import (
	"path/filepath"
	"testing"
)

func TestOpenRejectsMissingDir(t *testing.T) {
	_, err := Open(filepath.Join("testdata", "does-not-exist"))
	if err == nil {
		t.Fatal("expected error")
	}
}
