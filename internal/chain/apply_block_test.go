package chain

import "testing"

func TestApplyBlockSignatureExists(t *testing.T) {
	// Smoke test: type-check the function symbol. Behavior is exercised in
	// the integration test (Task 31) once a real state fixture exists.
	var _ = ApplyBlock
}
