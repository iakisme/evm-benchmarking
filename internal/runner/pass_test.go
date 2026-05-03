package runner

import "testing"

func TestPassConfigDefaults(t *testing.T) {
	c := PassConfig{}
	c.applyDefaults()
	if c.SamplerInterval == 0 {
		t.Errorf("sampler interval not defaulted")
	}
}
