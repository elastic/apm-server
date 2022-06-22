package beater

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestDebouncer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fired := make(chan struct{})
	d := &debouncer{
		triggerc: make(chan chan error),
		timeout:  50 * time.Millisecond,
		fn: func() error {
			close(fired)
			return errors.New("err")
		},
	}

	go d.loop(ctx)

	c1 := d.trigger()
	select {
	case <-fired:
		t.Fatal("didn't debounce")
	case <-time.After(30 * time.Millisecond):
	}
	c2 := d.trigger()
	select {
	case <-fired:
		t.Fatal("didn't debounce")
	case <-time.After(30 * time.Millisecond):
	}
	c3 := d.trigger()
	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("fn did not fire")
	}

	assert.Error(t, <-c1)
	assert.Error(t, <-c2)
	assert.Error(t, <-c3)
}
