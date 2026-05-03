package metrics

import (
	"testing"

	"github.com/ethereum/go-ethereum/core/vm"
)

func TestStateOpCounterCountsSloadSstore(t *testing.T) {
	tr := NewStateOpCounter()
	h := tr.Hooks()

	// emulate a few SLOADs and SSTOREs via the OnOpcode hook
	h.OnOpcode(0, byte(vm.SLOAD), 0, 0, nil, nil, 0, nil)
	h.OnOpcode(0, byte(vm.SLOAD), 0, 0, nil, nil, 0, nil)
	h.OnOpcode(0, byte(vm.SSTORE), 0, 0, nil, nil, 0, nil)
	h.OnOpcode(0, byte(vm.ADD), 0, 0, nil, nil, 0, nil)

	r, w := tr.Counts()
	if r != 2 {
		t.Errorf("reads = %d, want 2", r)
	}
	if w != 1 {
		t.Errorf("writes = %d, want 1", w)
	}
}
