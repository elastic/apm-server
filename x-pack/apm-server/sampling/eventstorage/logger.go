// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import "github.com/elastic/beats/v7/libbeat/logp"

// LogpAdaptor adapts logp.Logger to the badger.Logger interface.
type LogpAdaptor struct {
	*logp.Logger
}

// Warningf adapts badger.Logger.Warningf to logp.Logger.Warngf.
func (a LogpAdaptor) Warningf(format string, args ...interface{}) {
	a.Warnf(format, args...)
}
