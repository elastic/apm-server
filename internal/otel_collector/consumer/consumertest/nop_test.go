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

package consumertest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/consumer/pdata"
)

func TestTracesNop(t *testing.T) {
	nt := NewTracesNop()
	require.NotNil(t, nt)
	require.NoError(t, nt.ConsumeTraces(context.Background(), pdata.NewTraces()))
}

func TestMetricsNop(t *testing.T) {
	nm := NewMetricsNop()
	require.NotNil(t, nm)
	require.NoError(t, nm.ConsumeMetrics(context.Background(), pdata.NewMetrics()))
}

func TestLogsNop(t *testing.T) {
	nl := NewLogsNop()
	require.NotNil(t, nl)
	require.NoError(t, nl.ConsumeLogs(context.Background(), pdata.NewLogs()))
}
