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

package component

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
)

// Processor defines the common functions that must be implemented by TracesProcessor
// and MetricsProcessor.
type Processor interface {
	Component
}

// TracesProcessor is a processor that can consume traces.
type TracesProcessor interface {
	Processor
	consumer.Traces
}

// MetricsProcessor is a processor that can consume metrics.
type MetricsProcessor interface {
	Processor
	consumer.Metrics
}

// LogsProcessor is a processor that can consume logs.
type LogsProcessor interface {
	Processor
	consumer.Logs
}

// ProcessorCreateSettings is passed to Create* functions in ProcessorFactory.
type ProcessorCreateSettings struct {
	// Logger that the factory can use during creation and can pass to the created
	// component to be used later as well.
	Logger *zap.Logger

	// TracerProvider that the factory can pass to other instrumented third-party libraries.
	TracerProvider trace.TracerProvider

	// BuildInfo can be used by components for informational purposes
	BuildInfo BuildInfo
}

// ProcessorFactory is factory interface for processors. This is the
// new factory type that can create new style processors.
//
// This interface cannot be directly implemented. Implementations need to embed
// the BaseProcessorFactory or use the processorhelper.NewFactory to implement it.
type ProcessorFactory interface {
	Factory

	// CreateDefaultConfig creates the default configuration for the Processor.
	// This method can be called multiple times depending on the pipeline
	// configuration and should not cause side-effects that prevent the creation
	// of multiple instances of the Processor.
	// The object returned by this method needs to pass the checks implemented by
	// 'configcheck.ValidateConfig'. It is recommended to have these checks in the
	// tests of any implementation of the Factory interface.
	CreateDefaultConfig() config.Processor

	// CreateTracesProcessor creates a trace processor based on this config.
	// If the processor type does not support tracing or if the config is not valid,
	// an error will be returned instead.
	CreateTracesProcessor(
		ctx context.Context,
		set ProcessorCreateSettings,
		cfg config.Processor,
		nextConsumer consumer.Traces,
	) (TracesProcessor, error)

	// CreateMetricsProcessor creates a metrics processor based on this config.
	// If the processor type does not support metrics or if the config is not valid,
	// an error will be returned instead.
	CreateMetricsProcessor(
		ctx context.Context,
		set ProcessorCreateSettings,
		cfg config.Processor,
		nextConsumer consumer.Metrics,
	) (MetricsProcessor, error)

	// CreateLogsProcessor creates a processor based on the config.
	// If the processor type does not support logs or if the config is not valid,
	// an error will be returned instead.
	CreateLogsProcessor(
		ctx context.Context,
		set ProcessorCreateSettings,
		cfg config.Processor,
		nextConsumer consumer.Logs,
	) (LogsProcessor, error)

	// unexportedProcessor is a dummy method to force this interface to not be implemented.
	unexportedProcessor()
}

// BaseProcessorFactory is the interface that must be embedded by all ProcessorFactory implementations.
type BaseProcessorFactory struct{}

var _ ProcessorFactory = (*BaseProcessorFactory)(nil)

// Type must be overridden.
func (b BaseProcessorFactory) Type() config.Type {
	panic("implement me")
}

// CreateDefaultConfig must be overridden.
func (b BaseProcessorFactory) CreateDefaultConfig() config.Processor {
	panic("implement me")
}

// CreateTracesProcessor default implemented as not supported data type.
func (b BaseProcessorFactory) CreateTracesProcessor(context.Context, ProcessorCreateSettings, config.Processor, consumer.Traces) (TracesProcessor, error) {
	return nil, componenterror.ErrDataTypeIsNotSupported
}

// CreateMetricsProcessor default implemented as not supported data type.
func (b BaseProcessorFactory) CreateMetricsProcessor(context.Context, ProcessorCreateSettings, config.Processor, consumer.Metrics) (MetricsProcessor, error) {
	return nil, componenterror.ErrDataTypeIsNotSupported
}

// CreateLogsProcessor default implemented as not supported data type.
func (b BaseProcessorFactory) CreateLogsProcessor(context.Context, ProcessorCreateSettings, config.Processor, consumer.Logs) (LogsProcessor, error) {
	return nil, componenterror.ErrDataTypeIsNotSupported
}

func (b BaseProcessorFactory) unexportedProcessor() {}
