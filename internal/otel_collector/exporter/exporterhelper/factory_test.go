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

package exporterhelper

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
)

const typeStr = "test"

var (
	defaultCfg = &configmodels.ExporterSettings{
		TypeVal: typeStr,
		NameVal: typeStr,
	}
	nopTracesExporter, _ = NewTraceExporter(defaultCfg, zap.NewNop(), func(ctx context.Context, td pdata.Traces) (droppedSpans int, err error) {
		return 0, nil
	})
	nopMetricsExporter, _ = NewMetricsExporter(defaultCfg, zap.NewNop(), func(ctx context.Context, md pdata.Metrics) (droppedTimeSeries int, err error) {
		return 0, nil
	})
	nopLogsExporter, _ = NewLogsExporter(defaultCfg, zap.NewNop(), func(ctx context.Context, md pdata.Logs) (droppedTimeSeries int, err error) {
		return 0, nil
	})
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory(
		typeStr,
		defaultConfig)
	assert.EqualValues(t, typeStr, factory.Type())
	assert.EqualValues(t, defaultCfg, factory.CreateDefaultConfig())
	_, ok := factory.(component.ConfigUnmarshaler)
	assert.False(t, ok)
	_, err := factory.CreateTracesExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, defaultCfg)
	assert.Equal(t, configerror.ErrDataTypeIsNotSupported, err)
	_, err = factory.CreateMetricsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, defaultCfg)
	assert.Equal(t, configerror.ErrDataTypeIsNotSupported, err)
	_, err = factory.CreateLogsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, defaultCfg)
	assert.Equal(t, configerror.ErrDataTypeIsNotSupported, err)
}

func TestNewFactory_WithConstructors(t *testing.T) {
	factory := NewFactory(
		typeStr,
		defaultConfig,
		WithTraces(createTraceExporter),
		WithMetrics(createMetricsExporter),
		WithLogs(createLogsExporter),
		WithCustomUnmarshaler(customUnmarshaler))
	assert.EqualValues(t, typeStr, factory.Type())
	assert.EqualValues(t, defaultCfg, factory.CreateDefaultConfig())

	fu, ok := factory.(component.ConfigUnmarshaler)
	assert.True(t, ok)
	assert.Equal(t, errors.New("my error"), fu.Unmarshal(nil, nil))

	te, err := factory.CreateTracesExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, defaultCfg)
	assert.NoError(t, err)
	assert.Same(t, nopTracesExporter, te)

	me, err := factory.CreateMetricsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, defaultCfg)
	assert.NoError(t, err)
	assert.Same(t, nopMetricsExporter, me)

	le, err := factory.CreateLogsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, defaultCfg)
	assert.NoError(t, err)
	assert.Same(t, nopLogsExporter, le)
}

func defaultConfig() configmodels.Exporter {
	return defaultCfg
}

func createTraceExporter(context.Context, component.ExporterCreateParams, configmodels.Exporter) (component.TracesExporter, error) {
	return nopTracesExporter, nil
}

func createMetricsExporter(context.Context, component.ExporterCreateParams, configmodels.Exporter) (component.MetricsExporter, error) {
	return nopMetricsExporter, nil
}

func createLogsExporter(context.Context, component.ExporterCreateParams, configmodels.Exporter) (component.LogsExporter, error) {
	return nopLogsExporter, nil
}

func customUnmarshaler(*viper.Viper, interface{}) error {
	return errors.New("my error")
}
