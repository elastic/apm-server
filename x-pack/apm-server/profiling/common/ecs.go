// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package common

// EcsVersionString is the value for the `ecs.version` metrics field.
// It is relatively arbitrary and currently has no consumer.
// APM server is using 1.12.0. We stick with it as well.
const EcsVersionString = "1.12.0"

type ecsVersion string

func (e ecsVersion) MarshalJSON() ([]byte, error) {
	return []byte("\"" + EcsVersionString + "\""), nil
}

type EcsVersion struct {
	V ecsVersion `json:"ecs.version"`
}
