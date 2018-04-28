// +build !go1.9

package apmsql

import "database/sql/driver"

type stmtGo19 struct{}

func (stmtGo19) init(in driver.Stmt) {}
