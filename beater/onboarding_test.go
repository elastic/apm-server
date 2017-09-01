package beater

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/beat"
)

func TestNotifyUpServerDown(t *testing.T) {
	config := defaultConfig
	var saved []beat.Event
	var reporter = func(events []beat.Event) error {
		saved = append(saved, events...)
		return nil
	}

	server := newServer(config, reporter)
	go server.run()

	notifyListening(config, reporter)

	listening := saved[0].Fields["listening"].(string)
	assert.Equal(t, "localhost:8200", listening)
}
