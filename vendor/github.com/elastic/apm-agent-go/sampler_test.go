package elasticapm_test

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-agent-go"
)

func TestRatioSampler(t *testing.T) {
	ratio := 0.75
	source := rand.NewSource(0) // fixed seed for test
	s := elasticapm.NewRatioSampler(ratio, source)

	const (
		numGoroutines = 100
		numIterations = 1000
	)

	sampled := make([]int, numGoroutines)
	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				if s.Sample(nil) {
					sampled[i]++
				}
			}

		}(i)
	}
	wg.Wait()

	var total int
	for i := 0; i < numGoroutines; i++ {
		total += sampled[i]
	}
	assert.InDelta(t, ratio, float64(total)/(numGoroutines*numIterations), 0.1)
}
