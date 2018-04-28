// +build !go1.10

package apmsql

import "database/sql/driver"

type connGo110 struct{}

func (connGo110) init(in driver.Conn) {}
