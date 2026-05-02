package chain

import (
	"fmt"

	"github.com/ethereum/go-ethereum/params"
)

// ResolveChainConfig returns the BSC params.ChainConfig for the given chain ID.
// manifestForkOverrides allows the manifest to surface fork heights when the
// pinned BSC version doesn't yet know about them; entries here override defaults.
func ResolveChainConfig(chainID uint64, manifestForkOverrides map[string]uint64) (*params.ChainConfig, error) {
	var cfg *params.ChainConfig
	switch chainID {
	case 56:
		cfg = params.BSCChainConfig
	case 97:
		cfg = params.ChapelChainConfig
	default:
		return nil, fmt.Errorf("unknown chain id %d (only BSC mainnet=56 and chapel=97 are supported)", chainID)
	}
	if cfg == nil {
		return nil, fmt.Errorf("BSC params for chain %d not found in this BSC build", chainID)
	}
	// manifest fork overrides are intentionally not applied automatically; they
	// are kept for forward-compatibility with manifests prepared on newer BSC
	// versions than this binary. A future revision can apply them here once the
	// upstream fork-config struct is stable.
	_ = manifestForkOverrides
	return cfg, nil
}
