package chain

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

// ApplyBlockResult is what one consensus-bypassed block execution produces.
type ApplyBlockResult struct {
	Receipts        types.Receipts
	UsedGas         uint64
	StateRoot       common.Hash
	StateReadCount  uint64
	StateWriteCount uint64
	ExecNs          uint64
	TrieCommitNs    uint64
	SystemTxSkipped uint32
}

// Hooks is set of optional callbacks for instrumentation. Nil-checked.
type Hooks struct {
	NewTracer  func() vm.EVMLogger
	ReadTracer func(vm.EVMLogger) (reads, writes uint64)
}

// ApplyBlock executes the block's transactions against statedb and returns aggregated info.
// Header verification, Finalize, and validator-rotation system tx handling are all bypassed.
// System transactions (to == BSC system contract addresses) are detected and skipped
// (counted separately).
func ApplyBlock(
	cfg *params.ChainConfig,
	stateDB *state.StateDB,
	header *types.Header,
	txs types.Transactions,
	hooks *Hooks,
	timer Timer,
) (ApplyBlockResult, error) {
	res := ApplyBlockResult{}
	gp := new(core.GasPool).AddGas(header.GasLimit)
	chainCtx := stubChainContext{}

	signer := types.MakeSigner(cfg, header.Number, header.Time)

	t0 := timer.Now()
	for i, tx := range txs {
		if isSystemTx(tx, header, signer) {
			res.SystemTxSkipped++
			continue
		}
		stateDB.SetTxContext(tx.Hash(), i)

		var tracer vm.EVMLogger
		if hooks != nil && hooks.NewTracer != nil {
			tracer = hooks.NewTracer()
		}
		vmCfg := vm.Config{Tracer: tracer, NoBaseFee: false}

		receipt, err := core.ApplyTransaction(cfg, chainCtx, &header.Coinbase,
			gp, stateDB, header, tx, &res.UsedGas, vmCfg)
		if err != nil {
			return res, fmt.Errorf("tx %d (%s): %w", i, tx.Hash().Hex(), err)
		}
		res.Receipts = append(res.Receipts, receipt)

		if hooks != nil && hooks.ReadTracer != nil && tracer != nil {
			r, w := hooks.ReadTracer(tracer)
			res.StateReadCount += r
			res.StateWriteCount += w
		}
	}
	t1 := timer.Now()
	res.ExecNs = uint64(t1 - t0)

	res.StateRoot = stateDB.IntermediateRoot(true /*deleteEmptyObjects*/)
	t2 := timer.Now()
	res.TrieCommitNs = uint64(t2 - t1)

	return res, nil
}

// isSystemTx mirrors BSC Parlia's IsSystemTransaction:
//  1. tx.To() is a system contract address
//  2. sender == header.Coinbase (validator is the sender)
//  3. tx.GasPrice() == 0
//
// All three must hold; user-originated txs to system contracts (e.g.,
// StakeHub.delegate()) must NOT be skipped.
func isSystemTx(tx *types.Transaction, header *types.Header, signer types.Signer) bool {
	if tx.To() == nil {
		return false
	}
	to := *tx.To()
	matched := false
	for _, addr := range systemContractAddresses {
		if to == addr {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	if tx.GasPrice().Sign() != 0 {
		return false
	}
	sender, err := types.Sender(signer, tx)
	if err != nil {
		// Bad signature — defer judgment to ApplyTransaction's own validation;
		// don't skip.
		return false
	}
	return sender == header.Coinbase
}

// systemContractAddresses mirrors the systemContracts map in BSC Parlia
// (consensus/parlia/parlia.go lines 86-101, v1.4.8). All 14 entries must be
// present so that user-initiated transactions to the remaining contracts are
// correctly NOT skipped.
var systemContractAddresses = []common.Address{
	common.HexToAddress("0x0000000000000000000000000000000000001000"), // ValidatorContract
	common.HexToAddress("0x0000000000000000000000000000000000001001"), // SlashContract
	common.HexToAddress("0x0000000000000000000000000000000000001002"), // SystemRewardContract
	common.HexToAddress("0x0000000000000000000000000000000000001003"), // LightClientContract
	common.HexToAddress("0x0000000000000000000000000000000000001004"), // TokenHubContract
	common.HexToAddress("0x0000000000000000000000000000000000001005"), // RelayerIncentivizeContract
	common.HexToAddress("0x0000000000000000000000000000000000001006"), // RelayerHubContract
	common.HexToAddress("0x0000000000000000000000000000000000001007"), // GovHubContract
	common.HexToAddress("0x0000000000000000000000000000000000002000"), // CrossChainContract
	common.HexToAddress("0x0000000000000000000000000000000000002002"), // StakeHubContract
	common.HexToAddress("0x0000000000000000000000000000000000002004"), // GovernorContract
	common.HexToAddress("0x0000000000000000000000000000000000002005"), // GovTokenContract
	common.HexToAddress("0x0000000000000000000000000000000000002006"), // TimelockContract
	common.HexToAddress("0x0000000000000000000000000000000000003000"), // TokenRecoverPortalContract
}

// Timer is a small wrapper so tests can fake the monotonic clock.
type Timer interface {
	Now() int64 // nanoseconds since some monotonic epoch
}

type RealTimer struct{}

func (RealTimer) Now() int64 { return time.Now().UnixNano() }

// stubChainContext satisfies core.ChainContext for ApplyTransaction without a
// live blockchain. We pass a non-nil author to NewEVMBlockContext, so
// Engine().Author() is never invoked. GetHeader is only consulted lazily by the
// BLOCKHASH opcode for ancestors not yet cached; returning nil there causes
// BLOCKHASH to yield zero, which is acceptable for benchmark replay.
type stubChainContext struct{}

func (stubChainContext) Engine() consensus.Engine                    { return nil }
func (stubChainContext) GetHeader(common.Hash, uint64) *types.Header { return nil }
