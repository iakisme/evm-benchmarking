package corpus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const StateDirName = "state"

// Corpus represents a validated input directory. Use Open() to construct.
type Corpus struct {
	dir      string
	manifest *Manifest
}

func Open(dir string) (*Corpus, error) {
	mPath := filepath.Join(dir, ManifestFileName)
	m, err := LoadManifest(mPath)
	if err != nil {
		return nil, err
	}

	bPath := filepath.Join(dir, BlocksFileName)
	if st, err := os.Stat(bPath); err != nil {
		return nil, fmt.Errorf("%s: %w", BlocksFileName, err)
	} else if st.Size() == 0 {
		return nil, errors.New("blocks.rlp is empty")
	}

	sPath := filepath.Join(dir, StateDirName)
	if st, err := os.Stat(sPath); err != nil {
		return nil, fmt.Errorf("%s: %w", StateDirName, err)
	} else if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", StateDirName)
	}

	return &Corpus{dir: dir, manifest: m}, nil
}

func (c *Corpus) Manifest() *Manifest  { return c.manifest }
func (c *Corpus) Dir() string          { return c.dir }
func (c *Corpus) StateDir() string     { return filepath.Join(c.dir, StateDirName) }
func (c *Corpus) BlocksPath() string   { return filepath.Join(c.dir, BlocksFileName) }

// OpenBlockIter opens blocks.rlp for streaming.
func (c *Corpus) OpenBlockIter() (*BlockIter, error) {
	f, err := os.Open(c.BlocksPath())
	if err != nil {
		return nil, fmt.Errorf("open blocks: %w", err)
	}
	return NewBlockIter(f), nil
}

func (c *Corpus) Close() error { return nil }
