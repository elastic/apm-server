package elasticapm

import (
	"math/rand"
	"sync"

	"github.com/pkg/errors"
)

// Sampler provides a means of sampling transactions.
type Sampler interface {
	// Sample indicates whether or not the transaction
	// should be sampled. This method will be invoked
	// by calls to Tracer.StartTransaction, so it must
	// be goroutine-safe, and should avoid synchronization
	// as far as possible.
	Sample(*Transaction) bool
}

// RatioSampler is a Sampler that samples probabilistically
// based on the given ratio within the range [0,1.0].
//
// A ratio of 1.0 samples 100% of transactions, a ratio of 0.5
// samples ~50%, and so on.
type RatioSampler struct {
	mu  sync.Mutex
	rng *rand.Rand
	r   float64
}

// NewRatioSampler returns a new RatioSampler with the given ratio
// and math/rand.Source. The source will be called from multiple
// goroutines, but the sampler will internally synchronise access
// to it; the source should not be used by any other goroutines.
//
// If the ratio provided does not lie within the range [0,1.0],
// NewRatioSampler will panic.
func NewRatioSampler(r float64, source rand.Source) *RatioSampler {
	if r < 0 || r > 1.0 {
		panic(errors.Errorf("ratio %v out of range [0,1.0]", r))
	}
	return &RatioSampler{
		rng: rand.New(source),
		r:   r,
	}
}

// Sample samples the transaction according to the configured
// ratio and pseudo-random source.
func (s *RatioSampler) Sample(*Transaction) bool {
	s.mu.Lock()
	v := s.rng.Float64()
	s.mu.Unlock()
	return s.r > v
}
