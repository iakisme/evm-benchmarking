// Package metrics implements EVM-layer and process-layer instrumentation.
package metrics

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

// Compile-time assertion that StateOpCounter implements vm.EVMLogger.
var _ vm.EVMLogger = (*StateOpCounter)(nil)

// StateOpCounter is a vm.EVMLogger that counts SLOAD and SSTORE opcodes.
// It deliberately does nothing else. Allocate one per tx.
type StateOpCounter struct {
	reads, writes uint64
}

// NewStateOpCounter returns a zeroed StateOpCounter ready for use.
func NewStateOpCounter() *StateOpCounter { return &StateOpCounter{} }

// vm.EVMLogger interface — transaction level

func (s *StateOpCounter) CaptureTxStart(uint64)       {}
func (s *StateOpCounter) CaptureTxEnd(uint64)         {}
func (s *StateOpCounter) CaptureSystemTxEnd(uint64)   {}

// vm.EVMLogger interface — top call frame

func (s *StateOpCounter) CaptureStart(_ *vm.EVM, _ common.Address, _ common.Address,
	_ bool, _ []byte, _ uint64, _ *big.Int) {
}

func (s *StateOpCounter) CaptureEnd(_ []byte, _ uint64, _ error) {}

// vm.EVMLogger interface — rest of call frames

func (s *StateOpCounter) CaptureEnter(_ vm.OpCode, _ common.Address, _ common.Address,
	_ []byte, _ uint64, _ *big.Int) {
}

func (s *StateOpCounter) CaptureExit(_ []byte, _ uint64, _ error) {}

// vm.EVMLogger interface — opcode level

// CaptureState is called for every opcode executed. It increments reads for
// SLOAD and writes for SSTORE; all other opcodes are ignored.
func (s *StateOpCounter) CaptureState(_ uint64, op vm.OpCode, _, _ uint64,
	_ *vm.ScopeContext, _ []byte, _ int, _ error) {
	switch op {
	case vm.SLOAD:
		s.reads++
	case vm.SSTORE:
		s.writes++
	}
}

func (s *StateOpCounter) CaptureFault(_ uint64, _ vm.OpCode, _, _ uint64,
	_ *vm.ScopeContext, _ int, _ error) {
}

// Counts returns the accumulated SLOAD count (reads) and SSTORE count (writes).
func (s *StateOpCounter) Counts() (reads, writes uint64) {
	return s.reads, s.writes
}
