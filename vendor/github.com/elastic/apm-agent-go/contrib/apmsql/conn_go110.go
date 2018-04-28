// +build go1.10

package apmsql

import (
	"context"
	"database/sql/driver"
)

// Support for Conn interfaces introduced in Go 1.10 and later.
type connGo110 struct {
	sessionResetter driver.SessionResetter
}

func (c *connGo110) init(in driver.Conn) {
	c.sessionResetter, _ = in.(driver.SessionResetter)
}

func (c *connGo110) ResetSession(ctx context.Context) error {
	if c.sessionResetter != nil {
		return c.sessionResetter.ResetSession(ctx)
	}
	return nil
}
