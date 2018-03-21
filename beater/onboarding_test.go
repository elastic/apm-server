package beater

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/beat"
)

func TestNotifyUpServerDown(t *testing.T) {
	config := defaultConfig("7.0.0")
	var saved beat.Event
	var publisher = func(e beat.Event) { saved = e }

	lis, err := net.Listen("tcp", "localhost:0")
	assert.NoError(t, err)
	defer lis.Close()
	config.Host = lis.Addr().String()

	server := newServer(config, nopReporter)
	go run(server, lis, config)

	notifyListening(config, publisher)

	listening := saved.Fields["listening"].(string)
	assert.Equal(t, config.Host, listening)

}
