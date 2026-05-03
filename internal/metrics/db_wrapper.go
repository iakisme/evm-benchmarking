package metrics

import (
	"sync/atomic"

	"github.com/ethereum/go-ethereum/ethdb"
)

// CountingDB wraps an ethdb.Database, counting bytes read and written.
// Wrapping is intended for read paths; in BSC's StateDB the heavy reads happen
// through Get/Has and the writes through Batch.Put. We instrument both.
type CountingDB struct {
	ethdb.Database
	readBytes, writeBytes atomic.Uint64
}

func NewCountingDB(inner ethdb.Database) *CountingDB {
	return &CountingDB{Database: inner}
}

func (c *CountingDB) Get(key []byte) ([]byte, error) {
	v, err := c.Database.Get(key)
	if err == nil {
		c.readBytes.Add(uint64(len(v)))
	}
	return v, err
}

func (c *CountingDB) Put(key, value []byte) error {
	c.writeBytes.Add(uint64(len(value)))
	return c.Database.Put(key, value)
}

func (c *CountingDB) NewBatch() ethdb.Batch {
	return &countingBatch{Batch: c.Database.NewBatch(), counter: c}
}

func (c *CountingDB) Counts() (read, write uint64) {
	return c.readBytes.Load(), c.writeBytes.Load()
}

func (c *CountingDB) ResetCounts() {
	c.readBytes.Store(0)
	c.writeBytes.Store(0)
}

type countingBatch struct {
	ethdb.Batch
	counter *CountingDB
}

func (b *countingBatch) Put(key, value []byte) error {
	b.counter.writeBytes.Add(uint64(len(value)))
	return b.Batch.Put(key, value)
}
