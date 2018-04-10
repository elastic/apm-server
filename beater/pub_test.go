package beater

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/beat"
)

func TestNumberGoRoutines(t *testing.T) {
	before := runtime.NumGoroutine()
	_, err := newPublisher(pip{}, 1, time.Duration(0))
	assert.NoError(t, err)
	assert.Equal(t, before+runtime.GOMAXPROCS(0), runtime.NumGoroutine())
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
