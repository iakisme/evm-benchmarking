package chain

import "testing"

func TestResolveChainConfigBSCMainnet(t *testing.T) {
	cfg, err := ResolveChainConfig(56, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cfg.ChainID == nil || cfg.ChainID.Uint64() != 56 {
		t.Errorf("chain id = %v", cfg.ChainID)
	}
}

func TestResolveChainConfigUnknownChainErrors(t *testing.T) {
	_, err := ResolveChainConfig(999999999, nil)
	if err == nil {
		t.Fatal("expected error for unknown chain id")
	}
}
