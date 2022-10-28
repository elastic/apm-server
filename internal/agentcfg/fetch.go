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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-server/internal/kibana"
)

// Error Messages used to signal fetching errors
const (
	ErrMsgReadKibanaResponse = "unable to read Kibana response body"
	ErrMsgSendToKibanaFailed = "sending request to kibana failed"
	ErrUnauthorized          = "Unauthorized"
)

// TransactionSamplingRateKey is the agent configuration key for the
// sampling rate. This is used by the Jaeger handler to adapt our agent
// configuration to the Jaeger remote sampler protocol.
const TransactionSamplingRateKey = "transaction_sample_rate"

const endpoint = "/api/apm/settings/agent-configuration/search"

// Fetcher defines a common interface to retrieving agent config.
type Fetcher interface {
	Fetch(context.Context, Query) (Result, error)
}

// KibanaFetcher holds static information and information shared between requests.
// It implements the Fetch method to retrieve agent configuration information.
type KibanaFetcher struct {
	*cache
	logger *logp.Logger
	client *kibana.Client
}

// NewKibanaFetcher returns a KibanaFetcher instance.
//
// NewKibanaFetcher will panic if passed a nil client.
func NewKibanaFetcher(client *kibana.Client, cacheExpiration time.Duration) *KibanaFetcher {
	if client == nil {
		panic("client is required")
	}
	logger := logp.NewLogger("agentcfg")
	return &KibanaFetcher{
		client: client,
		logger: logger,
		cache:  newCache(logger, cacheExpiration),
	}
}

// Fetch retrieves agent configuration, fetched from Kibana or a local temporary cache.
func (f *KibanaFetcher) Fetch(ctx context.Context, query Query) (Result, error) {
	req := func() (Result, error) {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(query); err != nil {
			return Result{}, err
		}
		return newResult(f.request(ctx, &buf))
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

	result, err := io.ReadAll(resp.Body)
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

// AgentConfig holds an agent configuration definition, as used by DirectFetcher.
type AgentConfig struct {
	// ServiceName holds the service name to which this agent configuration
	// applies. This is optional.
	ServiceName string

	// ServiceEnvironment holds the service environment to which this agent
	// configuration applies. This is optional.
	ServiceEnvironment string

	// AgentName holds the agent name to which this agent configuration
	// applies. This is optional, and is used for filtering configuration
	// settings for unauthenticated agents.
	AgentName string

	// Etag holds a unique ID for the configuration, which agents
	// will send along with their queries. The server uses this to
	// determine whether agent configuration has been applied.
	Etag string

	// Config holds configuration settings that should be sent to
	// agents matching the above constraints.
	Config map[string]string
}

// DirectFetcher is an agent config fetcher which serves requests out of a
// statically defined set of agent configuration. These configurations are
// typically provided via Fleet.
type DirectFetcher struct {
	cfgs []AgentConfig
}

// NewDirectFetcher returns a new DirectFetcher that serves agent configuration
// requests using cfgs.
func NewDirectFetcher(cfgs []AgentConfig) *DirectFetcher {
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
	var nameConf, envConf, defaultConf *AgentConfig

	for i, cfg := range f.cfgs {
		if cfg.ServiceName == name && cfg.ServiceEnvironment == env {
			nameConf = &f.cfgs[i]
			break
		} else if cfg.ServiceName == name && cfg.ServiceEnvironment == "" {
			nameConf = &f.cfgs[i]
		} else if cfg.ServiceName == "" && cfg.ServiceEnvironment == env {
			envConf = &f.cfgs[i]
		} else if cfg.ServiceName == "" && cfg.ServiceEnvironment == "" {
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
