package beater

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/beat"
)

func TestNotifyUpServerDown(t *testing.T) {
	config := defaultConfig()
	var saved []beat.Event
	var reporter = func(events []beat.Event) error {
		saved = append(saved, events...)
		return nil
	}

	lis, err := net.Listen("tcp", "localhost:0")
	assert.NoError(t, err)
	defer lis.Close()
	config.Host = lis.Addr().String()

	server := newServer(config, reporter)
	go run(server, lis, config)

	notifyListening(config, reporter)

	listening := saved[0].Fields["listening"].(string)
	assert.Equal(t, config.Host, listening)
}
