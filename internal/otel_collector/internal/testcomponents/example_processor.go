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

package testcomponents // import "go.opentelemetry.io/collector/internal/testcomponents"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
)

const procType = "exampleprocessor"

// ExampleProcessorConfig config for ExampleProcessor.
type ExampleProcessorConfig struct {
	config.ProcessorSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
}

// ExampleProcessorFactory is factory for ExampleProcessor.
var ExampleProcessorFactory = component.NewProcessorFactory(
	procType,
	createDefaultConfig,
	component.WithTracesProcessorAndStabilityLevel(createTracesProcessor, component.StabilityLevelInDevelopment),
	component.WithMetricsProcessorAndStabilityLevel(createMetricsProcessor, component.StabilityLevelInDevelopment),
	component.WithLogsProcessorAndStabilityLevel(createLogsProcessor, component.StabilityLevelInDevelopment))

// CreateDefaultConfig creates the default configuration for the Processor.
func createDefaultConfig() config.Processor {
	return &ExampleProcessorConfig{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(procType)),
	}
}

func createTracesProcessor(_ context.Context, _ component.ProcessorCreateSettings, _ config.Processor, nextConsumer consumer.Traces) (component.TracesProcessor, error) {
	return &ExampleProcessor{Traces: nextConsumer}, nil
}

func createMetricsProcessor(_ context.Context, _ component.ProcessorCreateSettings, _ config.Processor, nextConsumer consumer.Metrics) (component.MetricsProcessor, error) {
	return &ExampleProcessor{Metrics: nextConsumer}, nil
}

func createLogsProcessor(_ context.Context, _ component.ProcessorCreateSettings, _ config.Processor, nextConsumer consumer.Logs) (component.LogsProcessor, error) {
	return &ExampleProcessor{Logs: nextConsumer}, nil
}

type ExampleProcessor struct {
	consumer.Traces
	consumer.Metrics
	consumer.Logs
	Started bool
	Stopped bool
}

func (ep *ExampleProcessor) Start(_ context.Context, _ component.Host) error {
	ep.Started = true
	return nil
}

func (ep *ExampleProcessor) Shutdown(_ context.Context) error {
	ep.Stopped = true
	return nil
}

func (ep *ExampleProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}
