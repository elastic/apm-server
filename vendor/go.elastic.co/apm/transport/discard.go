package transport

import (
	"context"
	"io"
)

type discardTransport struct {
	err error
}

func (s discardTransport) SendStream(context.Context, io.Reader) error {
	return s.err
}
