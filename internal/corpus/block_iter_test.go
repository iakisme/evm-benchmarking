package corpus

import (
	"bytes"
	"errors"
	"io"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

func TestBlockIterReadsSequentialBlocks(t *testing.T) {
	blocks := makeFakeBlockChain(t, 3)

	var buf bytes.Buffer
	for _, b := range blocks {
		if err := rlp.Encode(&buf, b); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}

	it := NewBlockIter(io.NopCloser(bytes.NewReader(buf.Bytes())))
	defer it.Close()

	for i, want := range blocks {
		got, err := it.Next()
		if err != nil {
			t.Fatalf("[%d] next: %v", i, err)
		}
		if got.NumberU64() != want.NumberU64() {
			t.Errorf("[%d] number = %d", i, got.NumberU64())
		}
	}

	_, err := it.Next()
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected EOF, got %v", err)
	}
}

// makeFakeBlockChain builds n trivial blocks with monotonic numbers and
// linked parentHashes. Used as a stub for RLP round-tripping; not BSC-specific.
func makeFakeBlockChain(t *testing.T, n int) []*types.Block {
	t.Helper()
	out := make([]*types.Block, 0, n)
	var parent common.Hash
	for i := 0; i < n; i++ {
		h := &types.Header{
			Number:     big.NewInt(int64(i + 100)),
			ParentHash: parent,
			GasLimit:   30_000_000,
		}
		blk := types.NewBlockWithHeader(h)
		out = append(out, blk)
		parent = blk.Hash()
	}
	return out
}
