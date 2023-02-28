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

package idxmgmt

import (
	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/idxmgmt"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-server/internal/logs"
)

// NewSupporter creates a new idxmgmt.Supporter which directs all events
// to data streams. The given root config will be checked for deprecated/removed
// configuration, and if any are present warnings will be logged.
func NewSupporter(log *logp.Logger, configRoot *config.C) idxmgmt.Supporter {
	if log == nil {
		log = logp.NewLogger(logs.IndexManagement)
	} else {
		log = log.Named(logs.IndexManagement)
	}
	if configRoot != nil {
		logWarnings(log, configRoot)
	}
	return dataStreamsSupporter{}
}

func logWarnings(log *logp.Logger, cfg *config.C) {
	type deprecatedConfig struct {
		name string
		info string
	}
	deprecatedConfigs := []deprecatedConfig{
		{"apm-server.data_streams.enabled", "data streams are always enabled"},
		{"apm-server.ilm", "ILM policies are managed by Fleet"},
		{"apm-server.register.ingest.pipeline", "ingest pipelines are managed by Fleet"},
		{"output.elasticsearch.index", "indices cannot be customised, APM Server now produces data streams"},
		{"output.elasticsearch.indices", "indices cannot be customised, APM Server now produces data streams"},
		{"setup.template", "index templates are managed by Fleet"},
	}
	format := "`%s` specified, but was removed in 8.0 and will be ignored: %s"
	for _, deprecated := range deprecatedConfigs {
		ok, err := cfg.Has(deprecated.name, -1)
		if err != nil {
			log.Warn(err)
		} else if ok {
			log.Warnf(format, deprecated.name, deprecated.info)
		}
	}
}

type dataStreamsSupporter struct{}

// BuildSelector always returns an IndexSelector that returns an error.
//
// BuildSupporter must be implemented by idxmgmt.Supporter, and is used only by
// the "apm-server test output" command.
func (dataStreamsSupporter) BuildSelector(*config.C) (outputs.IndexSelector, error) {
	return unsupportedIndexSelector{}, nil
}

type unsupportedIndexSelector struct{}

// Select always returns an error.
//
// Select must be implemented by idxmgmt.Supporter, but is only used by the libbeat
// Elasticsearch output. We do not use the libbeat Elasticsearch output, except for
// the "apm-server test output" command; calls to this method are unexpected.
func (unsupportedIndexSelector) Select(event *beat.Event) (string, error) {
	return "", errors.New("unexpected call to Select")
}

// Enabled always returns false, indicating that this idxmgmt.Supporter does
// not setting up templates or ILM policies.
func (dataStreamsSupporter) Enabled() bool {
	return false
}

// Manager returns a no-op idxmgmt.Manager.
func (dataStreamsSupporter) Manager(client idxmgmt.ClientHandler, assets idxmgmt.Asseter) idxmgmt.Manager {
	return dataStreamsManager{}
}

type dataStreamsManager struct{}

// VerifySetup always returns true and an empty string, to avoid logging
// duplicate warnings.
func (dataStreamsManager) VerifySetup(template, ilm idxmgmt.LoadMode) (bool, string) {
	// Just return true to avoid logging warnings. We'll error out in Setup.
	return true, ""
}

// Setup will always return an error, in response to manual setup (i.e. `apm-server setup`).
func (dataStreamsManager) Setup(template, ilm idxmgmt.LoadMode) error {
	return errors.New("index setup must be performed externally when using data streams, by installing the 'apm' integration package")
}
