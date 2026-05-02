package corpus

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

const BlocksFileName = "blocks.rlp"

// BlockIter streams blocks from a reader containing concatenated RLP-encoded blocks.
type BlockIter struct {
	rc    io.ReadCloser
	br    *bufio.Reader
	rlpS  *rlp.Stream
	count uint64
}

func NewBlockIter(rc io.ReadCloser) *BlockIter {
	br := bufio.NewReaderSize(rc, 1<<20)
	return &BlockIter{
		rc:   rc,
		br:   br,
		rlpS: rlp.NewStream(br, 0),
	}
}

// Next returns the next block or io.EOF when the stream is exhausted.
func (it *BlockIter) Next() (*types.Block, error) {
	var b types.Block
	if err := it.rlpS.Decode(&b); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("decode block #%d: %w", it.count, err)
	}
	it.count++
	return &b, nil
}

func (it *BlockIter) Close() error {
	return it.rc.Close()
}

func (it *BlockIter) Count() uint64 { return it.count }
