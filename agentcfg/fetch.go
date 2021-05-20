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

package agentcfg

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/convert"
	"github.com/elastic/apm-server/kibana"
)

// Error Messages used to signal fetching errors
const (
	ErrMsgKibanaDisabled             = "disabled Kibana configuration"
	ErrMsgKibanaVersionNotCompatible = "not a compatible Kibana version"
	ErrMsgNoKibanaConnection         = "unable to retrieve connection to Kibana"
	ErrMsgReadKibanaResponse         = "unable to read Kibana response body"
	ErrMsgSendToKibanaFailed         = "sending request to kibana failed"
	ErrUnauthorized                  = "Unauthorized"
	TransactionSamplingRateKey       = "transaction_sample_rate"
)

var (
	errMsgKibanaDisabled     = errors.New(ErrMsgKibanaDisabled)
	errMsgNoKibanaConnection = errors.New(ErrMsgNoKibanaConnection)
)

// KibanaMinVersion specifies the minimal required version of Kibana
// that supports agent configuration management
var KibanaMinVersion = common.MustNewVersion("7.5.0")

const endpoint = "/api/apm/settings/agent-configuration/search"

// Fetcher defines a common interface to retrieving agent config.
type Fetcher interface {
	Fetch(context.Context, Query) (Result, error)
}

// NewFetcher returns a new Fetcher based on the provided config.
func NewFetcher(cfg *config.Config) Fetcher {
<<<<<<< HEAD
	if cfg.AgentConfigs != nil || !cfg.Kibana.Enabled {
=======
	if cfg.AgentConfigs != nil {
>>>>>>> b7468c0d (Direct agent configuration (#5177))
		// Direct agent configuration is present, disable communication
		// with kibana.
		return NewDirectFetcher(cfg.AgentConfigs)
	}
	var client kibana.Client
	if cfg.Kibana.Enabled {
		client = kibana.NewConnectingClient(&cfg.Kibana)
	}
<<<<<<< HEAD
=======

	if cfg.KibanaAgentConfig == nil {
		cfg.KibanaAgentConfig = config.DefaultKibanaAgentConfig()
	}

>>>>>>> b7468c0d (Direct agent configuration (#5177))
	return NewKibanaFetcher(client, cfg.KibanaAgentConfig.Cache.Expiration)
}

// KibanaFetcher holds static information and information shared between requests.
// It implements the Fetch method to retrieve agent configuration information.
type KibanaFetcher struct {
	*cache
	logger *logp.Logger
	client kibana.Client
}

// NewKibanaFetcher returns a KibanaFetcher instance.
func NewKibanaFetcher(client kibana.Client, cacheExpiration time.Duration) *KibanaFetcher {
	logger := logp.NewLogger("agentcfg")
	return &KibanaFetcher{
		client: client,
		logger: logger,
		cache:  newCache(logger, cacheExpiration),
	}
}

// ValidationError encapsulates a validation error from the KibanaFetcher.
// ValidationError implements the error interface.
type ValidationError struct {
	keyword, body string
	err           error
}

// Keyword returns the keyword for the ValidationError.
func (v *ValidationError) Keyword() string { return v.keyword }

// Body returns the body for the ValidationError.
func (v *ValidationError) Body() string { return v.body }

// Error() implements the error interface.
func (v *ValidationError) Error() string { return v.err.Error() }

// Validate validates the currently configured KibanaFetcher.
func (f *KibanaFetcher) validate(ctx context.Context) *ValidationError {
	if f.client == nil {
		return &ValidationError{
			keyword: ErrMsgKibanaDisabled,
			body:    ErrMsgKibanaDisabled,
			err:     errMsgKibanaDisabled,
		}
	}
	if supported, err := f.client.SupportsVersion(ctx, KibanaMinVersion, true); !supported {
		if err != nil {
			return &ValidationError{
				keyword: ErrMsgNoKibanaConnection,
				body:    ErrMsgNoKibanaConnection,
				err:     errMsgNoKibanaConnection,
			}
		}

		version, _ := f.client.GetVersion(ctx)
		errMsg := fmt.Sprintf(
			"%s: min version %+v, configured version %+v",
			ErrMsgKibanaVersionNotCompatible, KibanaMinVersion, version.String(),
		)
		return &ValidationError{
			keyword: ErrMsgKibanaVersionNotCompatible,
			body:    errMsg,
			err:     errors.New(errMsg),
		}
	}
	return nil
}

// Fetch retrieves agent configuration, fetched from Kibana or a local temporary cache.
func (f *KibanaFetcher) Fetch(ctx context.Context, query Query) (Result, error) {
	if err := f.validate(ctx); err != nil {
		return zeroResult(), err
	}
	req := func() (Result, error) {
		return newResult(f.request(ctx, convert.ToReader(query)))
	}
	result, err := f.fetch(query, req)
	return sanitize(query.InsecureAgents, result), err
}

func (f *KibanaFetcher) request(ctx context.Context, r io.Reader) ([]byte, error) {
	resp, err := f.client.Send(ctx, http.MethodPost, endpoint, nil, nil, r)
	if err != nil {
		return nil, errors.Wrap(err, ErrMsgSendToKibanaFailed)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	result, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, errors.New(string(result))
	}
	if err != nil {
		return nil, errors.Wrap(err, ErrMsgReadKibanaResponse)
	}
	return result, nil
}

func sanitize(insecureAgents []string, result Result) Result {
	if len(insecureAgents) == 0 {
		return result
	}
	hasDataForAgent := containsAnyPrefix(result.Source.Agent, insecureAgents) || result.Source.Agent == ""
	if !hasDataForAgent {
		return zeroResult()
	}
	settings := Settings{}
	for k, v := range result.Source.Settings {
		if UnrestrictedSettings[k] {
			settings[k] = v
		}
	}
	return Result{Source: Source{Etag: result.Source.Etag, Settings: settings}}
}

func containsAnyPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

type DirectFetcher struct {
	cfgs []config.AgentConfig
}

func NewDirectFetcher(cfgs []config.AgentConfig) *DirectFetcher {
	return &DirectFetcher{cfgs}
}

// Fetch finds a matching AgentConfig based on the received Query.
// Order of precedence:
// - service.name and service.environment match an AgentConfig
// - service.name matches an AgentConfig, service.environment == ""
// - service.environment matches an AgentConfig, service.name == ""
// - an AgentConfig without a name or environment set
// Return an empty result if no matching result is found.
func (f *DirectFetcher) Fetch(_ context.Context, query Query) (Result, error) {
	name, env := query.Service.Name, query.Service.Environment
	result := zeroResult()
	var nameConf, envConf, defaultConf *config.AgentConfig

	for i, cfg := range f.cfgs {
		if cfg.Service.Name == name && cfg.Service.Environment == env {
			nameConf = &f.cfgs[i]
			break
		} else if cfg.Service.Name == name && cfg.Service.Environment == "" {
			nameConf = &f.cfgs[i]
		} else if cfg.Service.Name == "" && cfg.Service.Environment == env {
			envConf = &f.cfgs[i]
		} else if cfg.Service.Name == "" && cfg.Service.Environment == "" {
			defaultConf = &f.cfgs[i]
		}
	}

	if nameConf != nil {
		result = Result{Source{
			Settings: nameConf.Config,
			Etag:     nameConf.Etag,
			Agent:    nameConf.AgentName,
		}}
	} else if envConf != nil {
		result = Result{Source{
			Settings: envConf.Config,
			Etag:     envConf.Etag,
			Agent:    envConf.AgentName,
		}}
	} else if defaultConf != nil {
		result = Result{Source{
			Settings: defaultConf.Config,
			Etag:     defaultConf.Etag,
			Agent:    defaultConf.AgentName,
		}}
	}

	return sanitize(query.InsecureAgents, result), nil
}
