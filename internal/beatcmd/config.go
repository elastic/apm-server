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

package beatcmd

import (
	"errors"
	"fmt"

	"github.com/elastic/beats/v7/libbeat/cfgfile"
	"github.com/elastic/beats/v7/libbeat/cloudid"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/pprof"
	"github.com/elastic/elastic-agent-libs/config"
	libkeystore "github.com/elastic/elastic-agent-libs/keystore"
	"github.com/elastic/elastic-agent-libs/paths"
	ucfg "github.com/elastic/go-ucfg"
)

type Config struct {
	// APMServer holds apm-server.* configuration.
	APMServer *config.C `config:"apm-server"`

	MaxProcs  int `config:"max_procs"`
	GCPercent int `config:"gc_percent"`

	HTTP          *config.C     `config:"http"`
	HTTPPprof     *pprof.Config `config:"http.pprof"`
	BufferConfig  *config.C     `config:"http.buffer"`
	Logging       *config.C     `config:"logging"`
	MetricLogging *config.C     `config:"logging.metrics"`

	Instrumentation *config.C `config:"instrumentation"`
	Monitoring      *config.C `config:"monitoring"`

	Output     config.Namespace `config:"output"`
	Management *config.C        `config:"management"`
}

type loadConfigOptions struct {
	disableConfigResolution bool
	mergeConfig             []*config.C
}

// LoadConfigOption is an option for controlling LoadConfig behaviour.
type LoadConfigOption func(*loadConfigOptions)

// LoadConfig loads apm-server configuration from apm-server.yml,
// using command-line flags.
func LoadConfig(opts ...LoadConfigOption) (*Config, *config.C, libkeystore.Keystore, error) {
	var loadConfigOptions loadConfigOptions
	for _, opt := range opts {
		opt(&loadConfigOptions)
	}

	cfg, err := cfgfile.Load("", nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error loading config file: %w", err)
	}
	for _, mc := range loadConfigOptions.mergeConfig {
		if err := cfg.Merge(mc); err != nil {
			return nil, nil, nil, fmt.Errorf("error merging config: %w", err)
		}
	}
	if err := initPaths(cfg); err != nil {
		return nil, nil, nil, err
	}

	keystore, err := loadKeystore(cfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not initialize the keystore: %w", err)
	}

	configOpts := []ucfg.Option{ucfg.PathSep(".")}
	if loadConfigOptions.disableConfigResolution {
		configOpts = append(configOpts, ucfg.ResolveNOOP)
	} else {
		configOpts = append(configOpts,
			ucfg.Resolve(libkeystore.ResolverWrap(keystore)),
			ucfg.ResolveEnv,
			ucfg.VarExp,
		)
	}
	config.OverwriteConfigOpts(configOpts)

	if err := cloudid.OverwriteSettings(cfg); err != nil {
		return nil, nil, nil, err
	}

	var config Config
	if err := validateConfig(cfg); err != nil {
		return nil, nil, nil, fmt.Errorf("invalid config: %w", err)
	}
	if err := cfg.Unpack(&config); err != nil {
		return nil, nil, nil, fmt.Errorf("error unpacking config data: %w", err)
	}
	return &config, cfg, keystore, nil
}

func validateConfig(rawConfig *config.C) error {
	if rawConfig.HasField("processors") {
		return errors.New("libbeat processors are not supported")
	}
	return nil
}

// WithDisableConfigResolution returns a LoadConfigOption
// that disables resolution of variables.
func WithDisableConfigResolution() LoadConfigOption {
	return func(opts *loadConfigOptions) {
		opts.disableConfigResolution = true
	}
}

// WithMergeConfig returns a LoadConfigOption that merges
// the given config.C objects into the raw configuration.
func WithMergeConfig(cfg ...*config.C) LoadConfigOption {
	return func(opts *loadConfigOptions) {
		opts.mergeConfig = cfg
	}
}

// loadKeystore returns the appropriate keystore based on the configuration.
func loadKeystore(cfg *config.C) (libkeystore.Keystore, error) {
	keystoreCfg, _ := cfg.Child("keystore", -1)
	defaultPathConfig := paths.Resolve(paths.Data, "apm-server.keystore")
	return libkeystore.Factory(keystoreCfg, defaultPathConfig, common.IsStrictPerms())
}

func initPaths(cfg *config.C) error {
	// To Fix the chicken-egg problem with the Keystore and the loading of the configuration
	// files we are doing a partial unpack of the configuration file and only take into consideration
	// the paths field. After we will unpack the complete configuration and keystore reference
	// will be correctly replaced.
	partialConfig := struct {
		Path paths.Path `config:"path"`
	}{}

	if err := cfg.Unpack(&partialConfig); err != nil {
		return fmt.Errorf("error extracting default paths: %w", err)
	}

	if err := paths.InitPaths(&partialConfig.Path); err != nil {
		return fmt.Errorf("error setting default paths: %w", err)
	}
	return nil
}
