// prepare-fixture builds a small synthetic BSC corpus suitable for the
// integration test (cmd/bscbench/replay_integration_test.go).
//
// Run from the repo root:
//
//	go run ./scripts/prepare-fixture --out=testdata/integration/chapel-50blocks
//
// The generated fixture has chain_id=97 (chapel) and uses block heights well
// below any BSC-specific fork (Ramanujan @ 1,010,000), so default Ethereum
// post-MuirGlacier semantics apply. Consensus is faked via ethash.NewFaker —
// our chain.ApplyBlock bypasses consensus anyway.
package main

import (
	"crypto/sha256"
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
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	pebbledb "github.com/ethereum/go-ethereum/ethdb/pebble"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/triedb"

	"github.com/kai-w/bscbench/internal/corpus"
)

const (
	// Block #(fromBlock) is the snapshot head. Replay covers (fromBlock, toBlock].
	fromBlock = uint64(100)
	nReplay   = uint64(50)

	// Deterministic ECDSA key — well-known dev key, also used by go-ethereum tests.
	senderKeyHex = "b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291"
)

func main() {
	out := flag.String("out", "testdata/integration/chapel-50blocks", "output directory")
	flag.Parse()

	chaindata := filepath.Join(*out, "state", "chaindata")
	ancient := filepath.Join(*out, "state", "ancient")
	mustMkdirAll(chaindata)
	mustMkdirAll(ancient)

	// Open pebble + freezer.
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

	// Sender / recipient.
	key, err := crypto.HexToECDSA(senderKeyHex)
	if err != nil {
		die("HexToECDSA", err)
	}
	sender := crypto.PubkeyToAddress(key.PublicKey)
	recipient := common.HexToAddress("0x000000000000000000000000000000000000c0de")

	// Genesis: chapel chain config, sender funded with 1M ether. Timestamp=1
	// keeps every derived block well before any time-based fork.
	funded, _ := new(big.Int).SetString("1000000000000000000000000", 10)
	chainCfg := params.ChapelChainConfig
	genesis := &core.Genesis{
		Config:     chainCfg,
		Alloc:      core.GenesisAlloc{sender: {Balance: funded}},
		GasLimit:   30_000_000,
		Difficulty: big.NewInt(1),
		Timestamp:  1,
	}

	tdb := triedb.NewDatabase(diskdb, triedb.HashDefaults)
	genesisBlock, err := genesis.Commit(diskdb, tdb)
	if err != nil {
		die("genesis.Commit", err)
	}
	// triedb.HashDefaults retains every node on disk; safe to close after commit.
	if err := tdb.Close(); err != nil {
		die("triedb.Close (genesis)", err)
	}

	total := int(fromBlock + nReplay)
	engine := ethash.NewFaker()
	blocks, _ := core.GenerateChain(chainCfg, genesisBlock, engine, diskdb, total,
		func(i int, b *core.BlockGen) {
			b.SetCoinbase(common.HexToAddress("0x000000000000000000000000000000000000bee5"))
			signer := types.MakeSigner(chainCfg, b.Number(), b.Timestamp())
			tx, err := types.SignTx(types.NewTransaction(
				b.TxNonce(sender), recipient, big.NewInt(1), 21000, big.NewInt(1_000_000_000), nil,
			), signer, key)
			if err != nil {
				panic(fmt.Errorf("sign tx %d: %w", i, err))
			}
			b.AddTx(tx)
		})

	// blocks[k] is block #(k+1). Snapshot head is block #fromBlock = blocks[fromBlock-1].
	// Replay = blocks (fromBlock, fromBlock+nReplay] = blocks[fromBlock : fromBlock+nReplay].
	headBlock := blocks[fromBlock-1]
	stateRootAtFrom := headBlock.Root()
	replayBlocks := blocks[fromBlock : fromBlock+nReplay]

	// Write blocks.rlp (concatenated RLP encodings, matching corpus.BlockIter).
	blocksPath := filepath.Join(*out, "blocks.rlp")
	bf, err := os.Create(blocksPath)
	if err != nil {
		die("create blocks.rlp", err)
	}
	for i, b := range replayBlocks {
		if err := rlp.Encode(bf, b); err != nil {
			die(fmt.Sprintf("rlp encode block %d", i), err)
		}
	}
	if err := bf.Close(); err != nil {
		die("close blocks.rlp", err)
	}

	// input_hash = sha256(blocks.rlp). State tarball is intentionally excluded
	// to keep the script self-contained; the load-time validator only requires
	// the field to be non-empty.
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
		Generator:               "synthetic-chapel-fixture",
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

	if err := diskdb.Close(); err != nil {
		die("diskdb.Close", err)
	}

	// Self-validation: re-open the corpus + state DB the same way the runner
	// does, and confirm state.New succeeds at expected_state_root_at_from.
	if err := validate(*out); err != nil {
		die("post-write validation", err)
	}

	totalSize := dirSize(*out)
	fmt.Printf("fixture written:\n")
	fmt.Printf("  out = %s\n", *out)
	fmt.Printf("  blocks = %d (#%d..#%d)\n", nReplay, fromBlock+1, fromBlock+nReplay)
	fmt.Printf("  state_root_at_from = %s\n", stateRootAtFrom.Hex())
	fmt.Printf("  input_hash = %s\n", inputHash)
	fmt.Printf("  total size = %.2f MiB\n", float64(totalSize)/(1<<20))
}

// validate re-opens the freshly written fixture and walks it through the same
// preflight the runner performs: corpus.Open, then state.New at the manifest
// root. Any failure here means the fixture is internally inconsistent.
func validate(dir string) error {
	c, err := corpus.Open(dir)
	if err != nil {
		return fmt.Errorf("corpus.Open: %w", err)
	}
	defer c.Close()

	chaindata := filepath.Join(c.StateDir(), "chaindata")
	ancient := filepath.Join(c.StateDir(), "ancient")
	kv, err := pebbledb.New(chaindata, 16, 16, "validate/", true /*readonly*/)
	if err != nil {
		return fmt.Errorf("validate pebble.New: %w", err)
	}
	diskdb, err := rawdb.Open(kv, rawdb.OpenOptions{Ancient: ancient, ReadOnly: true})
	if err != nil {
		return fmt.Errorf("validate rawdb.Open: %w", err)
	}
	defer diskdb.Close()

	tdb := triedb.NewDatabase(diskdb, nil) // auto-detect scheme — must match writer
	defer tdb.Close()

	root := common.HexToHash(c.Manifest().ExpectedStateRootAtFrom)
	if _, err := state.New(root, state.NewDatabase(tdb, nil)); err != nil {
		return fmt.Errorf("state.New(%s): %w", root.Hex(), err)
	}

	// Sanity-check block stream length.
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
