package pipelistener_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/pipelistener"
)

func TestNew(t *testing.T) {
	l := pipelistener.New()
	assert.NotNil(t, l)
	defer l.Close()
	assert.Implements(t, new(net.Listener), l)
}

func TestAddr(t *testing.T) {
	l := pipelistener.New()
	assert.NotNil(t, l)
	defer l.Close()

	addr := l.Addr()
	assert.NotNil(t, addr)
	assert.Equal(t, "pipe", addr.Network())
	assert.Equal(t, "pipe", addr.String())
}

func TestDialAccept(t *testing.T) {
	l := pipelistener.New()
	assert.NotNil(t, l)
	defer l.Close()

	clientCh := make(chan net.Conn, 1)
	go func() {
		defer close(clientCh)
		client, err := l.DialContext(context.Background(), "foo", "bar")
		if assert.NoError(t, err) {
			clientCh <- client
		}
	}()

	server, err := l.Accept()
	assert.NoError(t, err)
	client := <-clientCh
	defer server.Close()
	defer client.Close()

	hello := []byte("hello!")
	go client.Write(hello)
	buf := make([]byte, len(hello))
	_, err = server.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, string(hello), string(buf))
}

func TestAcceptClosed(t *testing.T) {
	l := pipelistener.New()
	assert.NotNil(t, l)
	defer l.Close()

	err := l.Close()
	assert.NoError(t, err)
	_, err = l.Accept()
	assert.Error(t, pipelistener.ErrListenerClosed, err)
}

func TestDialClosed(t *testing.T) {
	l := pipelistener.New()
	assert.NotNil(t, l)
	defer l.Close()

	err := l.Close()
	assert.NoError(t, err)
	_, err = l.DialContext(context.Background(), "foo", "bar")
	assert.Error(t, pipelistener.ErrListenerClosed, err)
}

func TestDialContextCanceled(t *testing.T) {
	l := pipelistener.New()
	assert.NotNil(t, l)
	defer l.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := l.DialContext(ctx, "foo", "bar")
	assert.Error(t, context.Canceled, err)
}
