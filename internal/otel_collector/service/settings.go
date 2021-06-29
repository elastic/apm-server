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

package service

import (
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/parserprovider"
)

// svcSettings holds configuration for building a new service.
type svcSettings struct {
	// Factories component factories.
	Factories component.Factories

	// BuildInfo provides application start information.
	BuildInfo component.BuildInfo

	// Config represents the configuration of the service.
	Config *config.Config

	// Logger represents the logger used for all the components.
	Logger *zap.Logger

	// AsyncErrorChannel is the channel that is used to report fatal errors.
	AsyncErrorChannel chan error
}

// AppSettings holds configuration for creating a new Application.
type AppSettings struct {
	// Factories component factories.
	Factories component.Factories

	// BuildInfo provides application start information.
	BuildInfo component.BuildInfo

	// ParserProvider provides the configuration's Parser.
	// If it is not provided a default provider is used. The default provider loads the configuration
	// from a config file define by the --config command line flag and overrides component's configuration
	// properties supplied via --set command line flag.
	ParserProvider parserprovider.ParserProvider

	// LoggingOptions provides a way to change behavior of zap logging.
	LoggingOptions []zap.Option
}
