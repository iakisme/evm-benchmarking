package runner

import "testing"

func TestDoublePassConfigDefaults(t *testing.T) {
	c := DoublePassConfig{}
	c.applyDefaults()
	if c.WorkDirRoot == "" {
		t.Errorf("workdir root not defaulted")
	}
}
