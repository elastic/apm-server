// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package nullable

// Bool represents a bool value which can be set to nil.
// Using uint32 since uint32 is smallest proto type.
type Bool uint32

const (
	// Nil represents an unset bool value.
	Nil Bool = iota
	// False represents a false bool value.
	False
	// True represents a true bool value.
	True
)

// ParseBoolPtr sets nullable bool from bool pointer.
func (nb *Bool) ParseBoolPtr(b *bool) {
	if b == nil {
		*nb = Nil
		return
	}
	if *b {
		*nb = True
		return
	}
	*nb = False
}

// ToBoolPtr converts nullable bool to bool pointer.
func (nb *Bool) ToBoolPtr() *bool {
	if nb == nil || *nb == Nil {
		return nil
	}
	var b bool
	switch *nb {
	case False:
		b = false
	case True:
		b = true
	}
	return &b
}
