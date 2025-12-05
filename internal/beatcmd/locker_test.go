// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

//go:build !integration
// +build !integration

package beatcmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocker(t *testing.T) {
	configYAML := "output.console.enabled: true"
	home := t.TempDir()
	configYAML += "\npath.home: " + home

	running := make(chan struct{})
	beat1 := newBeat(t, configYAML, func(RunnerParams) (Runner, error) {
		return runnerFunc(func(ctx context.Context) error {
			close(running)
			<-ctx.Done()
			return nil
		}), nil
	})

	stopBeat1 := runBeat(t, beat1)

	// Wait for the first beater to be running, at which point
	// the lock should be held.
	<-running

	// Create another Beat using the same configuration and data directory;
	// its Run method should fail to acquire the lock while beat1 is running.
	beat2 := newBeat(t, configYAML, func(RunnerParams) (Runner, error) {
		t.Fatal("should not be called")
		return nil, nil
	})
	err := beat2.Run(context.Background())
	require.ErrorIs(t, err, ErrAlreadyLocked)

	assert.NoError(t, stopBeat1())
}
