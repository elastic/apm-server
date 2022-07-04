// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"fmt"
	"sync"

	"github.com/elastic/elastic-agent-libs/logp"
)

// LogpAdaptor adapts logp.Logger to the badger.Logger interface.
type LogpAdaptor struct {
	*logp.Logger

	mu   sync.RWMutex
	last string
}

// Errorf prints the log message when the current message isn't the same as the
// previously logged message.
func (a *LogpAdaptor) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if a.setLast(msg) {
		a.Logger.Errorf(format, args...)
	}
}

func (a *LogpAdaptor) setLast(msg string) bool {
	a.mu.RLock()
	if msg != a.last {
		a.mu.RUnlock()
		return false
	}
	a.mu.RUnlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	shouldSet := msg != a.last
	if shouldSet {
		a.last = msg
	}
	return shouldSet
}

// Warningf adapts badger.Logger.Warningf to logp.Logger.Warngf.
func (a *LogpAdaptor) Warningf(format string, args ...interface{}) {
	a.Warnf(format, args...)
}
