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

package adaptive

import (
	"math"
	"strconv"
	"sync"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/beats/v7/libbeat/logp"
)

// AgentConfig configures
type AgentConfig struct {
	mu       sync.RWMutex
	factor   float64
	max      float64
	min      float64
	decision DecisionType
	logger   logp.Logger
}

func NewAgentConfig(min, max float64) *AgentConfig {
	return &AgentConfig{
		max:    math.Min(max, 1),
		min:    math.Max(min, 0.001),
		logger: *logp.NewLogger("adaptive_agent_config"),
	}
}

func (a *AgentConfig) Name() string { return "adaptive_agent_config" }

func (a *AgentConfig) Do(decision Decision) error {
	switch decision.Type {
	case DecisionDownsample:
		a.mu.Lock()
		defer a.mu.Unlock()
		a.factor = math.Min(math.Max(decision.Factor+a.factor, a.min), 1)
		a.decision = decision.Type
		a.logger.Infof("received adaptive downsample decision, resulting factor: -%0.5f", a.factor)
	case DecisionUpsample:
		a.mu.Lock()
		defer a.mu.Unlock()
		a.factor = math.Min(decision.Factor+a.factor, a.max)
		a.decision = decision.Type
		a.logger.Infof("received adaptive upsample decision, resulting factor: +%0.5f", a.factor)
	}
	return nil
}

// Adapt receives an agent configuration and modifies its sampling rate
// with the stored factor and decision. If no adapative decision has been
// taken, adapt won't do anything.
func (a *AgentConfig) Adapt(result *agentcfg.Result) {
	settings := result.Source.Settings
	if v, ok := settings[agentcfg.TransactionSamplingRateKey]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			a.mu.RLock()
			defer a.mu.RUnlock()

			delta := f * a.factor
			var result float64
			switch a.decision {
			case DecisionDownsample:
				result = f - delta
			case DecisionUpsample:
				result = f + delta
			default:
				return
			}
			a.logger.Infof("adapted sampling rate to: %0.5f", result)
			settings[agentcfg.TransactionSamplingRateKey] = strconv.FormatFloat(
				result, 'f', 5, 64,
			)
		}
	}
}
