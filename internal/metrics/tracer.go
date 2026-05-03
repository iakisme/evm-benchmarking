// Package metrics implements EVM-layer and process-layer instrumentation.
package metrics

import (
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

// StateOpCounter counts SLOAD and SSTORE opcodes via a tracing.Hooks bag.
// Allocate one per block; counts accumulate across all transactions in the block.
type StateOpCounter struct {
	reads, writes uint64
	hooks         *tracing.Hooks
}

// NewStateOpCounter returns a counter wired to a fresh tracing.Hooks.
func NewStateOpCounter() *StateOpCounter {
	c := &StateOpCounter{}
	c.hooks = &tracing.Hooks{
		OnOpcode: func(_ uint64, op byte, _, _ uint64, _ tracing.OpContext, _ []byte, _ int, _ error) {
			switch vm.OpCode(op) {
			case vm.SLOAD:
				c.reads++
			case vm.SSTORE:
				c.writes++
			}
		},
	}
	return c
}

// Hooks returns the underlying tracing.Hooks for installation in vm.Config.Tracer.
func (s *StateOpCounter) Hooks() *tracing.Hooks { return s.hooks }

// Counts returns the accumulated SLOAD count (reads) and SSTORE count (writes).
func (s *StateOpCounter) Counts() (reads, writes uint64) {
	return s.reads, s.writes
}
