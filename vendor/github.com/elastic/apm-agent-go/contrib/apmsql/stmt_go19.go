// +build go1.9

package apmsql

import "database/sql/driver"

type stmtGo19 struct {
	namedValueChecker driver.NamedValueChecker
}

func (s *stmtGo19) init(in driver.Stmt) {
	s.namedValueChecker, _ = in.(driver.NamedValueChecker)
}

func (s *stmt) CheckNamedValue(nv *driver.NamedValue) error {
	if s.namedValueChecker != nil {
		return s.namedValueChecker.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}
