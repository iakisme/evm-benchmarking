package metrics

import (
	"testing"

	"github.com/ethereum/go-ethereum/core/rawdb"
)

func TestCountingDBTracksBytes(t *testing.T) {
	mem := rawdb.NewMemoryDatabase()
	cdb := NewCountingDB(mem)

	if err := cdb.Put([]byte("k"), []byte("hello")); err != nil {
		t.Fatalf("put: %v", err)
	}
	v, err := cdb.Get([]byte("k"))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(v) != "hello" {
		t.Errorf("get = %q", v)
	}

	r, w := cdb.Counts()
	if r != 5 {
		t.Errorf("read bytes = %d", r)
	}
	if w != 5 {
		t.Errorf("write bytes = %d", w)
	}
}
