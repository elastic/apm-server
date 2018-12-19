package apm

import (
	"encoding/binary"
	"math"
	"math/big"

	"github.com/pkg/errors"
)

// Sampler provides a means of sampling transactions.
type Sampler interface {
	// Sample indicates whether or not a transaction
	// should be sampled. This method will be invoked
	// by calls to Tracer.StartTransaction for the root
	// of a trace, so it must be goroutine-safe, and
	// should avoid synchronization as far as possible.
	Sample(TraceContext) bool
}

// NewRatioSampler returns a new Sampler with the given ratio
//
// A ratio of 1.0 samples 100% of transactions, a ratio of 0.5
// samples ~50%, and so on. If the ratio provided does not lie
// within the range [0,1.0], NewRatioSampler will panic.
//
// The returned Sampler bases its decision on the value of the
// transaction ID, so there is no synchronization involved.
func NewRatioSampler(r float64) Sampler {
	if r < 0 || r > 1.0 {
		panic(errors.Errorf("ratio %v out of range [0,1.0]", r))
	}
	var x big.Float
	x.SetUint64(math.MaxUint64)
	x.Mul(&x, big.NewFloat(r))
	ceil, _ := x.Uint64()
	return ratioSampler{ceil}
}

type ratioSampler struct {
	ceil uint64
}

// Sample samples the transaction according to the configured
// ratio and pseudo-random source.
func (s ratioSampler) Sample(c TraceContext) bool {
	v := binary.BigEndian.Uint64(c.Span[:])
	return v > 0 && v-1 < s.ceil
}
