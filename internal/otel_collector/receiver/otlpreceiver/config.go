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

package otlpreceiver

import (
	"fmt"

	"github.com/spf13/cast"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configparser"
)

const (
	// Protocol values.
	protoGRPC          = "grpc"
	protoHTTP          = "http"
	protocolsFieldName = "protocols"
)

// Protocols is the configuration for the supported protocols.
type Protocols struct {
	GRPC *configgrpc.GRPCServerSettings `mapstructure:"grpc"`
	HTTP *confighttp.HTTPServerSettings `mapstructure:"http"`
}

// Config defines configuration for OTLP receiver.
type Config struct {
	config.ReceiverSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	// Protocols is the configuration for the supported protocols, currently gRPC and HTTP (Proto and JSON).
	Protocols `mapstructure:"protocols"`
}

var _ config.Receiver = (*Config)(nil)
var _ config.CustomUnmarshable = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	if cfg.GRPC == nil &&
		cfg.HTTP == nil {
		return fmt.Errorf("must specify at least one protocol when using the OTLP receiver")
	}
	return nil
}

// Unmarshal a config.Parser into the config struct.
func (cfg *Config) Unmarshal(componentParser *configparser.Parser) error {
	if componentParser == nil || len(componentParser.AllKeys()) == 0 {
		return fmt.Errorf("empty config for OTLP receiver")
	}
	// first load the config normally
	err := componentParser.UnmarshalExact(cfg)
	if err != nil {
		return err
	}

	// next manually search for protocols in viper, if a protocol is not present it means it is disable.
	protocols := cast.ToStringMap(componentParser.Get(protocolsFieldName))

	// UnmarshalExact will ignore empty entries like a protocol with no values, so if a typo happened
	// in the protocol that is intended to be enabled will not be enabled. So check if the protocols
	// include only known protocols.
	knownProtocols := 0
	if _, ok := protocols[protoGRPC]; !ok {
		cfg.GRPC = nil
	} else {
		knownProtocols++
	}

	if _, ok := protocols[protoHTTP]; !ok {
		cfg.HTTP = nil
	} else {
		knownProtocols++
	}

	if len(protocols) != knownProtocols {
		return fmt.Errorf("unknown protocols in the OTLP receiver")
	}
	return nil
}
