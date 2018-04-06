package beater

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/beat"
)

func TestNumberGoRoutines(t *testing.T) {
	for _, test := range []struct {
		goMaxProcs         int
		expectedGoRoutines func() int
	}{
		{goMaxProcs: 0, expectedGoRoutines: func() int { return runtime.NumGoroutine() + runtime.NumCPU() }},
		{goMaxProcs: 1, expectedGoRoutines: func() int { return runtime.NumGoroutine() + 1 }},
		{goMaxProcs: 15, expectedGoRoutines: func() int { return runtime.NumGoroutine() + runtime.NumCPU() }},
	} {
		expected := test.expectedGoRoutines()
		runtime.GOMAXPROCS(test.goMaxProcs)
		_, err := newPublisher(pip{}, 1, time.Duration(0))
		assert.NoError(t, err)
		assert.Equal(t, expected, runtime.NumGoroutine())
	}
}

type pip struct{}

func (p pip) Connect() (beat.Client, error) {
	return nil, nil
}
func (p pip) ConnectWith(c beat.ClientConfig) (beat.Client, error) {
	return nil, nil
}
func (p pip) SetACKHandler(ah beat.PipelineACKHandler) error {
	return nil
}
