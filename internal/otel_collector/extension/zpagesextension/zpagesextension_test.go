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

package zpagesextension

import (
	"context"
	"net"
	"net/http"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/testutil"
)

func TestZPagesExtensionUsage(t *testing.T) {
	config := Config{
		Endpoint: testutil.GetAvailableLocalAddress(t),
	}

	zpagesExt := newServer(config, zap.NewNop())
	require.NotNil(t, zpagesExt)

	require.NoError(t, zpagesExt.Start(context.Background(), componenttest.NewNopHost()))
	defer zpagesExt.Shutdown(context.Background())

	// Give a chance for the server goroutine to run.
	runtime.Gosched()

	_, zpagesPort, err := net.SplitHostPort(config.Endpoint)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Get("http://localhost:" + zpagesPort + "/debug/tracez")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestZPagesExtensionPortAlreadyInUse(t *testing.T) {
	endpoint := testutil.GetAvailableLocalAddress(t)
	ln, err := net.Listen("tcp", endpoint)
	require.NoError(t, err)
	defer ln.Close()

	config := Config{
		Endpoint: endpoint,
	}
	zpagesExt := newServer(config, zap.NewNop())
	require.NotNil(t, zpagesExt)

	require.Error(t, zpagesExt.Start(context.Background(), componenttest.NewNopHost()))
}

func TestZPagesMultipleStarts(t *testing.T) {
	config := Config{
		Endpoint: testutil.GetAvailableLocalAddress(t),
	}

	zpagesExt := newServer(config, zap.NewNop())
	require.NotNil(t, zpagesExt)

	require.NoError(t, zpagesExt.Start(context.Background(), componenttest.NewNopHost()))
	defer zpagesExt.Shutdown(context.Background())

	// Try to start it again, it will fail since it is on the same endpoint.
	require.Error(t, zpagesExt.Start(context.Background(), componenttest.NewNopHost()))
}

func TestZPagesMultipleShutdowns(t *testing.T) {
	config := Config{
		Endpoint: testutil.GetAvailableLocalAddress(t),
	}

	zpagesExt := newServer(config, zap.NewNop())
	require.NotNil(t, zpagesExt)

	require.NoError(t, zpagesExt.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, zpagesExt.Shutdown(context.Background()))
	require.NoError(t, zpagesExt.Shutdown(context.Background()))
}

func TestZPagesShutdownWithoutStart(t *testing.T) {
	config := Config{
		Endpoint: testutil.GetAvailableLocalAddress(t),
	}

	zpagesExt := newServer(config, zap.NewNop())
	require.NotNil(t, zpagesExt)

	require.NoError(t, zpagesExt.Shutdown(context.Background()))
}
