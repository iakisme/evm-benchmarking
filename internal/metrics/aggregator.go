package metrics

import (
	"math"
	"sort"
)

// Percentile returns the q-th percentile of v (q in [0,1]) using nearest-rank
// (1-based rank = ceil(q*n), converted to 0-based index).
// v is not modified. Empty or nil input returns 0.
func Percentile(v []uint64, q float64) uint64 {
	if len(v) == 0 {
		return 0
	}
	if q < 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	cp := append([]uint64(nil), v...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	// nearest-rank: rank = ceil(q * n), index = rank - 1, clamped to [0, n-1]
	rank := int(math.Ceil(q * float64(len(cp))))
	if rank < 1 {
		rank = 1
	}
	idx := rank - 1
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}
