package metrics

import (
	"testing"

	"github.com/ethereum/go-ethereum/core/vm"
)

func TestStateOpCounterCountsSloadSstore(t *testing.T) {
	tr := NewStateOpCounter()

	// emulate a few SLOADs and SSTOREs via CaptureState callback
	// scope is *vm.ScopeContext (concrete pointer); nil is fine because the counter ignores it
	tr.CaptureState(0, vm.SLOAD, 0, 0, nil, nil, 0, nil)
	tr.CaptureState(0, vm.SLOAD, 0, 0, nil, nil, 0, nil)
	tr.CaptureState(0, vm.SSTORE, 0, 0, nil, nil, 0, nil)
	tr.CaptureState(0, vm.ADD, 0, 0, nil, nil, 0, nil)

	r, w := tr.Counts()
	if r != 2 {
		t.Errorf("reads = %d, want 2", r)
	}
	if w != 1 {
		t.Errorf("writes = %d, want 1", w)
	}
}
