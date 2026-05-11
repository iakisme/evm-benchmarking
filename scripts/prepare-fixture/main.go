// prepare-fixture builds a synthetic BSC corpus suitable for the integration
// test (cmd/evmbench/replay_integration_test.go) and for local performance
// runs of evmbench. With default flags it produces a fixture sized so that
// `evmbench replay` (canonical double-pass) runs ~5 minutes total.
//
// Run from the repo root:
//
//	go run ./scripts/prepare-fixture                                  # default: ~5min replay
//	go run ./scripts/prepare-fixture --blocks=5000                    # quick probe
//	go run ./scripts/prepare-fixture --blocks=200 --tx-per-block=2 \
//	    --out=testdata/integration/chapel-smoke                       # CI smoke fixture
//
// chain_id=97 (chapel) with block heights well below any BSC fork
// (Ramanujan @ 1,010,000); consensus is faked via ethash.NewFaker since
// chain.ApplyBlock bypasses consensus anyway.
//
// Workload mix per replay block (txPerBlock = 15 by default):
//   - 10× call to Writer contract: SLOAD slot 0 + SSTORE slot 0 (warm modify)
//     + SSTORE unique cold slot + LOG1 (~50k gas each)
//   - 5×  value transfer to a fresh per-block-and-index recipient (~21k gas each)
//
// Total per block: ~605k gas, ~15 txs. This stresses cold/warm SSTORE,
// SLOAD, account creation, and log emission paths.
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	pebbledb "github.com/ethereum/go-ethereum/ethdb/pebble"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/pathdb"
	"github.com/holiman/uint256"

	"github.com/iakisme/evm-benchmarking/internal/corpus"
)

const (
	// fromBlock = pre-replay heat-up blocks (just enough for the manifest's
	// non-zero from_block invariant). Replay = (fromBlock, fromBlock+nReplay].
	fromBlock = uint64(10)

	senderKeyHex = "b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291"

	// Default block count is calibrated so that `evmbench replay` (canonical
	// double-pass) completes in roughly 5 minutes wall time on a modern x86
	// desktop (AMD Ryzen 9-class, NVMe). Each pass ≈ 2.5 min.
	//
	// Measured single-pass replay throughput (skip-warmup) on AMD 8945HS:
	//
	//	  blocks │ canonical wall │ measured rate │ commit p50 │ state
	//	    5k   │     ~10 s      │   942 b/s     │  <0.5 ms   │  165 MiB
	//	   25k   │   ~115 s       │   437 b/s     │   1.3 ms   │  900 MiB
	//	   30k   │    183 s       │   318 b/s     │   1.7 ms   │ 1100 MiB
	//	   40k   │   ~300 s       │   ~220 b/s    │  ~3.5 ms   │ 1500 MiB
	//	   50k   │    656 s       │   148 b/s     │   5.3 ms   │ 1900 MiB
	//
	// Per-block trie-commit time grows superlinearly with cumulative cold
	// SSTORE volume — cold-write benchmarks fundamentally don't scale linearly.
	// 40k targets ~5 min canonical (≈ 2.5 min/pass) on the reference host.
	defaultBlocks = 40000
	defaultTx     = 15
	writerTxPct   = 67 // 10 of every 15 txs are writer calls; remainder are transfers
)

// writerRuntime: 26-byte EVM runtime. Per call:
//   - SLOAD slot 0           (warm after first call: 100; first: 2100)
//   - SSTORE slot 0 (modify) (warm: 5000)
//   - SSTORE calldataload(0) (cold new slot: 22100)
//   - LOG1 with 32 bytes of memory and topic=0 (~1000)
//
//	60 00         PUSH1 0x00
//	54            SLOAD              ; counter
//	60 01         PUSH1 0x01
//	01            ADD                ; counter+1
//	80            DUP1
//	60 00         PUSH1 0x00
//	55            SSTORE             ; storage[0] := counter+1 (warm modify)
//	80            DUP1
//	60 00         PUSH1 0x00
//	35            CALLDATALOAD       ; slot from calldata
//	55            SSTORE             ; storage[slot] := counter+1 (cold new)
//	60 00         PUSH1 0x00
//	52            MSTORE             ; memory[0..32] = counter+1
//	60 00         PUSH1 0x00
//	60 20         PUSH1 0x20
//	60 00         PUSH1 0x00
//	a1            LOG1               ; log(memory[0..32], topic=0)
//	00            STOP
var writerRuntime = []byte{
	0x60, 0x00, 0x54, 0x60, 0x01, 0x01, 0x80, 0x60,
	0x00, 0x55, 0x80, 0x60, 0x00, 0x35, 0x55, 0x60,
	0x00, 0x52, 0x60, 0x00, 0x60, 0x20, 0x60, 0x00,
	0xa1, 0x00,
}

// writerContractAddr: fixed address well outside the 0x...1000..3000 system-
// contract range used by chain.ApplyBlock's skip-list.
var writerContractAddr = common.HexToAddress("0x000000000000000000000000000000000000c0de")

func main() {
	out := flag.String("out", "testdata/integration/chapel-bench", "output directory")
	nReplayFlag := flag.Int("blocks", defaultBlocks, "number of replay blocks")
	txPerBlockFlag := flag.Int("tx-per-block", defaultTx, "transactions per replay block")
	scheme := flag.String("scheme", "path", "state scheme: 'path' (PBSS, faster) or 'hash' (legacy)")
	flag.Parse()

	if *nReplayFlag <= 0 {
		die("invalid --blocks", fmt.Errorf("must be > 0, got %d", *nReplayFlag))
	}
	if *txPerBlockFlag <= 0 {
		die("invalid --tx-per-block", fmt.Errorf("must be > 0, got %d", *txPerBlockFlag))
	}
	if *scheme != "hash" && *scheme != "path" {
		die("invalid --scheme", fmt.Errorf("must be 'hash' or 'path', got %q", *scheme))
	}
	nReplay := uint64(*nReplayFlag)
	txPerBlock := *txPerBlockFlag
	writerTxs := txPerBlock * writerTxPct / 100
	if writerTxs == 0 && txPerBlock > 0 {
		writerTxs = 1
	}
	transferTxs := txPerBlock - writerTxs

	chaindata := filepath.Join(*out, "state", "chaindata")
	ancient := filepath.Join(*out, "state", "ancient")
	mustMkdirAll(chaindata)
	mustMkdirAll(ancient)

	key, err := crypto.HexToECDSA(senderKeyHex)
	if err != nil {
		die("HexToECDSA", err)
	}
	sender := crypto.PubkeyToAddress(key.PublicKey)

	// Funded with 10B BNB to cover even the largest --blocks settings.
	funded, _ := new(big.Int).SetString("10000000000000000000000000000", 10)
	chainCfg := params.ChapelChainConfig
	genesis := &core.Genesis{
		Config: chainCfg,
		Alloc: core.GenesisAlloc{
			sender: {Balance: funded},
			writerContractAddr: {
				Code:    writerRuntime,
				Balance: new(big.Int),
			},
		},
		GasLimit:   100_000_000,
		Difficulty: big.NewInt(1),
		Timestamp:  1,
	}

	total := int(fromBlock + nReplay)
	gasPrice := big.NewInt(1_000_000_000)
	engine := ethash.NewFaker()

	blockGenFn := func(i int, b *core.BlockGen) {
		b.SetCoinbase(common.HexToAddress("0x000000000000000000000000000000000000bee5"))
		signer := types.MakeSigner(chainCfg, b.Number(), b.Timestamp())
		blockNum := uint64(i + 1)

		if blockNum <= fromBlock {
			tx, err := types.SignTx(types.NewTransaction(
				b.TxNonce(sender), sender, big.NewInt(0), 21000, gasPrice, nil,
			), signer, key)
			if err != nil {
				panic(fmt.Errorf("warmup tx %d: %w", i, err))
			}
			b.AddTx(tx)
			return
		}

		for j := 0; j < writerTxs; j++ {
			slot := blockNum*1024 + uint64(j)
			data := make([]byte, 32)
			binary.BigEndian.PutUint64(data[24:32], slot)
			tx, err := types.SignTx(types.NewTransaction(
				b.TxNonce(sender), writerContractAddr,
				big.NewInt(0), 100_000, gasPrice, data,
			), signer, key)
			if err != nil {
				panic(fmt.Errorf("writer tx block=%d j=%d: %w", blockNum, j, err))
			}
			b.AddTx(tx)
		}
		for j := 0; j < transferTxs; j++ {
			recipient := common.BytesToAddress(deriveRecipient(blockNum, uint64(j)))
			tx, err := types.SignTx(types.NewTransaction(
				b.TxNonce(sender), recipient,
				big.NewInt(1), 21000, gasPrice, nil,
			), signer, key)
			if err != nil {
				panic(fmt.Errorf("transfer tx block=%d j=%d: %w", blockNum, j, err))
			}
			b.AddTx(tx)
		}
	}

	fmt.Fprintf(os.Stderr, "[prepare-fixture] scheme=%s, generating %d blocks (%d heat-up + %d replay × (%d writer + %d transfer)) ...\n",
		*scheme, total, fromBlock, nReplay, writerTxs, transferTxs)
	t0 := time.Now()

	var (
		blocks       []*types.Block
		genesisBlock *types.Block
	)
	if *scheme == "hash" {
		// Direct path: GenerateChain writes hash-scheme nodes straight into the
		// final diskdb, since GenerateChain hard-codes triedb.HashDefaults.
		kv, err := pebbledb.New(chaindata, 64, 64, "fixture/", false)
		if err != nil {
			die("pebble.New", err)
		}
		diskdb, err := rawdb.Open(kv, rawdb.OpenOptions{
			Ancient:          ancient,
			MetricsNamespace: "fixture/",
		})
		if err != nil {
			die("rawdb.Open", err)
		}
		hashTdb := triedb.NewDatabase(diskdb, triedb.HashDefaults)
		genesisBlock, err = genesis.Commit(diskdb, hashTdb)
		if err != nil {
			die("genesis.Commit (hash)", err)
		}
		if err := hashTdb.Close(); err != nil {
			die("triedb.Close (hash)", err)
		}
		blocks, _ = core.GenerateChain(chainCfg, genesisBlock, engine, diskdb, total, blockGenFn)
		if err := diskdb.Close(); err != nil {
			die("diskdb.Close (hash)", err)
		}
	} else {
		// Path scheme: GenerateChain forces hash, so use a TEMP hash diskdb
		// to obtain the blocks, then replay only the heat-up phase
		// (1..fromBlock) into the final path-scheme diskdb. Replay blocks
		// fromBlock+1..total are just serialised to blocks.rlp; their state is
		// reproduced by evmbench at run time.
		tmpRoot, err := os.MkdirTemp("", "prepare-fixture-tmp-")
		if err != nil {
			die("temp dir", err)
		}
		defer os.RemoveAll(tmpRoot)
		tmpKv, err := pebbledb.New(filepath.Join(tmpRoot, "chaindata"), 64, 64, "tmp/", false)
		if err != nil {
			die("temp pebble", err)
		}
		tmpDisk, err := rawdb.Open(tmpKv, rawdb.OpenOptions{
			Ancient:          filepath.Join(tmpRoot, "ancient"),
			MetricsNamespace: "tmp/",
		})
		if err != nil {
			die("temp rawdb", err)
		}
		tmpTdb := triedb.NewDatabase(tmpDisk, triedb.HashDefaults)
		genesisBlock, err = genesis.Commit(tmpDisk, tmpTdb)
		if err != nil {
			die("genesis.Commit (tmp)", err)
		}
		if err := tmpTdb.Close(); err != nil {
			die("triedb.Close (tmp)", err)
		}
		blocks, _ = core.GenerateChain(chainCfg, genesisBlock, engine, tmpDisk, total, blockGenFn)
		if err := tmpDisk.Close(); err != nil {
			die("tmpDisk.Close", err)
		}

		// Now reproduce blocks[0..fromBlock-1] against a fresh path-scheme
		// diskdb at the final destination.
		fkv, err := pebbledb.New(chaindata, 64, 64, "fixture/", false)
		if err != nil {
			die("final pebble", err)
		}
		finalDisk, err := rawdb.Open(fkv, rawdb.OpenOptions{
			Ancient:          ancient,
			MetricsNamespace: "fixture/",
		})
		if err != nil {
			die("final rawdb", err)
		}
		pathTdb := triedb.NewDatabase(finalDisk, &triedb.Config{PathDB: pathdb.Defaults})
		genesisFinal, err := genesis.Commit(finalDisk, pathTdb)
		if err != nil {
			die("genesis.Commit (path)", err)
		}
		if genesisFinal.Root() != genesisBlock.Root() {
			die("genesis root mismatch",
				fmt.Errorf("tmp=%s vs final=%s", genesisBlock.Root().Hex(), genesisFinal.Root().Hex()))
		}
		replayHeatUp(chainCfg, finalDisk, pathTdb, genesisFinal, blocks[:fromBlock])
		// Persist the in-memory diff layer hierarchy to disk. Without
		// Journal, pathdb.Close discards everything still buffered in RAM,
		// and the runner sees only the genesis state on the next open.
		headRoot := blocks[fromBlock-1].Root()
		if err := pathTdb.Journal(headRoot); err != nil {
			die("pathTdb.Journal", err)
		}
		if err := pathTdb.Close(); err != nil {
			die("pathTdb.Close", err)
		}
		if err := finalDisk.Close(); err != nil {
			die("finalDisk.Close", err)
		}
	}

	fmt.Fprintf(os.Stderr, "[prepare-fixture] generation done in %s (%.0f blocks/s)\n",
		time.Since(t0).Round(time.Millisecond),
		float64(total)/time.Since(t0).Seconds())

	headBlock := blocks[fromBlock-1]
	stateRootAtFrom := headBlock.Root()
	replayBlocks := blocks[fromBlock : fromBlock+nReplay]

	blocksPath := filepath.Join(*out, "blocks.rlp")
	bf, err := os.Create(blocksPath)
	if err != nil {
		die("create blocks.rlp", err)
	}
	for i, blk := range replayBlocks {
		if err := rlp.Encode(bf, blk); err != nil {
			die(fmt.Sprintf("rlp encode block %d", i), err)
		}
	}
	if err := bf.Close(); err != nil {
		die("close blocks.rlp", err)
	}

	bytes, err := os.ReadFile(blocksPath)
	if err != nil {
		die("re-read blocks.rlp", err)
	}
	sum := sha256.Sum256(bytes)
	inputHash := "sha256:" + hex.EncodeToString(sum[:])

	manifest := corpus.Manifest{
		SchemaVersion:           "1",
		ChainID:                 97,
		FromBlock:               fromBlock,
		ToBlock:                 fromBlock + nReplay,
		BlockCount:              nReplay,
		ExpectedStateRootAtFrom: stateRootAtFrom.Hex(),
		ForkSchedule:            map[string]uint64{},
		Generator:               fmt.Sprintf("synthetic-chapel-mixed-%s-%dblocks-%dtx", *scheme, nReplay, txPerBlock),
		GeneratedAt:             time.Now().UTC().Format(time.RFC3339),
		InputHash:               inputHash,
		BSCVersionRecommended:   "v1.7.3",
	}
	mb, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		die("marshal manifest", err)
	}
	if err := os.WriteFile(filepath.Join(*out, "manifest.json"), mb, 0o644); err != nil {
		die("write manifest", err)
	}

	if err := validate(*out); err != nil {
		die("post-write validation", err)
	}

	totalSize := dirSize(*out)
	fmt.Printf("fixture written:\n")
	fmt.Printf("  out = %s\n", *out)
	fmt.Printf("  blocks = %d (#%d..#%d, %d tx each = %d total tx)\n",
		nReplay, fromBlock+1, fromBlock+nReplay, txPerBlock, nReplay*uint64(txPerBlock))
	fmt.Printf("  state_root_at_from = %s\n", stateRootAtFrom.Hex())
	fmt.Printf("  input_hash = %s\n", inputHash)
	fmt.Printf("  total size = %.2f MiB\n", float64(totalSize)/(1<<20))
}

// replayHeatUp re-executes blocks[0..len-1] (= chain blocks 1..fromBlock)
// against the given path-scheme triedb so the on-disk state arrives at the
// expected root[fromBlock]. Each block is committed with statedb.Commit +
// triedb.Commit, and the resulting root is sanity-checked against the block
// header's root.
func replayHeatUp(cfg *params.ChainConfig, disk ethdb.Database, tdb *triedb.Database, genesisBlock *types.Block, blocks []*types.Block) {
	chainCtx := stubChainContext{cfg: cfg}
	parent := genesisBlock
	for i, blk := range blocks {
		statedb, err := state.New(parent.Root(), state.NewDatabase(tdb, nil))
		if err != nil {
			die(fmt.Sprintf("state.New block %d", i+1), err)
		}
		blockCtx := core.NewEVMBlockContext(blk.Header(), chainCtx, &blk.Header().Coinbase)
		evm := vm.NewEVM(blockCtx, statedb, cfg, vm.Config{})
		gp := new(core.GasPool).AddGas(blk.GasLimit())
		var usedGas uint64
		for j, tx := range blk.Transactions() {
			statedb.SetTxContext(tx.Hash(), j)
			if _, err := core.ApplyTransaction(evm, gp, statedb, blk.Header(), tx, &usedGas); err != nil {
				die(fmt.Sprintf("ApplyTx block %d j %d", i+1, j), err)
			}
		}
		// Mirror ethash.Faker.FinalizeAndAssemble: accumulate block reward
		// to coinbase. Chapel chain config is post-Constantinople from genesis,
		// so reward is the static 2-ETH constant. No uncles in our chain.
		blockReward := uint256.NewInt(2e+18)
		statedb.AddBalance(blk.Header().Coinbase, blockReward, tracing.BalanceIncreaseRewardMineBlock)

		root, err := statedb.Commit(blk.NumberU64(),
			cfg.IsEIP158(blk.Number()),
			cfg.IsCancun(blk.Number(), blk.Time()))
		if err != nil {
			die(fmt.Sprintf("statedb.Commit block %d", i+1), err)
		}
		if err := tdb.Commit(root, false); err != nil {
			die(fmt.Sprintf("tdb.Commit block %d", i+1), err)
		}
		if root != blk.Root() {
			die("heat-up root mismatch",
				fmt.Errorf("block %d: replayed=%s expected=%s", blk.NumberU64(), root.Hex(), blk.Root().Hex()))
		}
		parent = blk
	}
	_ = disk // disk is referenced via tdb; kept in signature for parity / future use
}

// stubChainContext satisfies core.ChainContext for the heat-up replay loop.
// The Header lookups return nil — only consulted lazily by BLOCKHASH on
// historical blocks not in cache, which yields zero — fine for fixture gen.
type stubChainContext struct {
	cfg *params.ChainConfig
}

func (s stubChainContext) Config() *params.ChainConfig               { return s.cfg }
func (stubChainContext) CurrentHeader() *types.Header                { return nil }
func (stubChainContext) GetHeader(common.Hash, uint64) *types.Header { return nil }
func (stubChainContext) GetHeaderByNumber(uint64) *types.Header      { return nil }
func (stubChainContext) GetHeaderByHash(common.Hash) *types.Header   { return nil }
func (stubChainContext) Engine() consensus.Engine                    { return nil }

// deriveRecipient returns a deterministic 20-byte address for (blockNum, idx).
// We tag the high byte with 0xa0 so addresses don't collide with the system
// contract range, the writer contract, or the sender.
func deriveRecipient(blockNum, idx uint64) []byte {
	out := make([]byte, 20)
	out[0] = 0xa0
	binary.BigEndian.PutUint64(out[4:12], blockNum)
	binary.BigEndian.PutUint64(out[12:20], idx)
	return out
}

func validate(dir string) error {
	c, err := corpus.Open(dir)
	if err != nil {
		return fmt.Errorf("corpus.Open: %w", err)
	}
	defer c.Close()

	chaindata := filepath.Join(c.StateDir(), "chaindata")
	ancient := filepath.Join(c.StateDir(), "ancient")
	kv, err := pebbledb.New(chaindata, 16, 16, "validate/", true)
	if err != nil {
		return fmt.Errorf("validate pebble.New: %w", err)
	}
	diskdb, err := rawdb.Open(kv, rawdb.OpenOptions{Ancient: ancient, ReadOnly: true})
	if err != nil {
		return fmt.Errorf("validate rawdb.Open: %w", err)
	}
	defer diskdb.Close()

	tdb := triedb.NewDatabase(diskdb, nil)
	defer tdb.Close()

	root := common.HexToHash(c.Manifest().ExpectedStateRootAtFrom)
	if _, err := state.New(root, state.NewDatabase(tdb, nil)); err != nil {
		return fmt.Errorf("state.New(%s): %w", root.Hex(), err)
	}

	it, err := c.OpenBlockIter()
	if err != nil {
		return fmt.Errorf("OpenBlockIter: %w", err)
	}
	defer it.Close()
	for {
		_, err := it.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("block iter: %w", err)
		}
	}
	if got := it.Count(); got != c.Manifest().BlockCount {
		return fmt.Errorf("block_count mismatch: stream has %d, manifest says %d",
			got, c.Manifest().BlockCount)
	}
	return nil
}

func mustMkdirAll(p string) {
	if err := os.MkdirAll(p, 0o755); err != nil {
		die("mkdir "+p, err)
	}
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func die(msg string, err error) {
	fmt.Fprintf(os.Stderr, "ERROR %s: %v\n", msg, err)
	os.Exit(1)
}
