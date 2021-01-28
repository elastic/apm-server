// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package componenthelper

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestDefaultSettings(t *testing.T) {
	st := DefaultComponentSettings()
	require.NotNil(t, st)
	cp := NewComponent(st)
	require.NoError(t, cp.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, cp.Shutdown(context.Background()))
}

func TestWithStart(t *testing.T) {
	startCalled := false
	st := DefaultComponentSettings()
	st.Start = func(context.Context, component.Host) error { startCalled = true; return nil }
	cp := NewComponent(st)
	assert.NoError(t, cp.Start(context.Background(), componenttest.NewNopHost()))
	assert.True(t, startCalled)
}

func TestWithStart_ReturnError(t *testing.T) {
	want := errors.New("my_error")
	st := DefaultComponentSettings()
	st.Start = func(context.Context, component.Host) error { return want }
	cp := NewComponent(st)
	assert.Equal(t, want, cp.Start(context.Background(), componenttest.NewNopHost()))
}

func TestWithShutdown(t *testing.T) {
	shutdownCalled := false
	st := DefaultComponentSettings()
	st.Shutdown = func(context.Context) error { shutdownCalled = true; return nil }
	cp := NewComponent(st)
	assert.NoError(t, cp.Shutdown(context.Background()))
	assert.True(t, shutdownCalled)
}

func TestWithShutdown_ReturnError(t *testing.T) {
	want := errors.New("my_error")
	st := DefaultComponentSettings()
	st.Shutdown = func(context.Context) error { return want }
	cp := NewComponent(st)
	assert.Equal(t, want, cp.Shutdown(context.Background()))
}
