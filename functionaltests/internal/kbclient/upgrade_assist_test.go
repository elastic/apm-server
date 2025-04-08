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

package kbclient_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ResolveMigrationDeprecations(t *testing.T) {
	kbc := newRecordedTestClient(t)

	ctx := context.Background()
	// Check that there are some critical deprecation warnings.
	deprecations, err := kbc.QueryCriticalESDeprecations(ctx)
	require.NoError(t, err)
	require.Greater(t, len(deprecations), 0)
	for _, deprecation := range deprecations {
		require.True(t, deprecation.IsCritical)
	}

	// Resolve them.
	// NOTE: This test will not be effective against different deprecation
	// types than what was initially set.
	err = kbc.ResolveMigrationDeprecations(ctx)
	require.NoError(t, err)

	// Check that there are no more.
	deprecations, err = kbc.QueryCriticalESDeprecations(ctx)
	require.NoError(t, err)
	assert.Len(t, deprecations, 0)
}
