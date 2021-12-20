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
	"crypto/md5"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/elastic/apm-server/agentcfg"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/beats/v7/libbeat/logp"
)

// AgentConfig configures
type AgentConfig struct {
	mu          sync.RWMutex
	logger      *logp.Logger
	max         float64
	min         float64
	ttl         time.Duration
	decision    DecisionType
	downsamples map[time.Time]float64
	rand        *rand.Rand
}

func NewAgentConfig(min, max float64) *AgentConfig {
	agent := &AgentConfig{
		max:         math.Min(max, 0.9999),
		min:         math.Max(min, 0.001),
		logger:      logp.NewLogger("adaptive_agent_config", logs.WithRateLimit(30*time.Second)),
		downsamples: make(map[time.Time]float64),
		ttl:         6 * time.Minute,
		rand:        rand.New(rand.NewSource(time.Now().Unix())),
	}
	return agent
}

func (a *AgentConfig) Name() string { return "adaptive_agent_config" }

func (a *AgentConfig) Do(decision Decision) error {
	switch decision.Type {
	case DecisionDownsample:
		a.mu.Lock()
		defer a.mu.Unlock()

		// Every time a downsampling decision is received, we renew all the
		// downsampling entries.
		now := time.Now()
		for ts, f := range a.downsamples {
			// Remove entries that are older than 60m.
			if ts.Add(time.Hour).Before(now) {
				delete(a.downsamples, ts)
			} else if ts.After(now) {
				a.downsamples[time.Now().Add(a.ttl).Add(time.Minute*time.Duration(a.rand.Intn(10)))] = f
				delete(a.downsamples, ts)
			}
		}
		a.downsamples[time.Now().Add(a.ttl)] = decision.Factor
		a.decision = decision.Type
		a.logger.Infof("received adaptive downsample decision, factor: -%0.5f, entries: %d", decision.Factor, len(a.downsamples))
	case DecisionUpsample:
		a.logger.Warn("received adaptive upsample decision, but no handler is implemented")
	}
	return nil
}

// Adapt receives an agent configuration and modifies its sampling rate
// with the stored factor and decision. If no adapative decision has been
// taken, adapt won't do anything.
func (a *AgentConfig) Adapt(result *agentcfg.Result) {
	// NOTE(marclop): Assuming that agents have the default sampling rate
	// may lead to increased load if the agents have a sampling rate lower
	// than 1 set and no central configuration is set up.
	// Ideally, we'd record each service's sampling rate and change that.
	rate := 1.0
	settings := result.Source.Settings
	key := agentcfg.TransactionSamplingRateKey
	if v, ok := settings[key]; ok {
		if r, err := strconv.ParseFloat(v, 64); err == nil {
			rate = r
		}
	}
	a.mu.RLock()
	defer a.mu.RUnlock()

	newRate := a.newRate(rate)
	if newRate == -1 {
		return
	}

	newSettings := make(map[string]string)
	for k, v := range result.Source.Settings {
		newSettings[k] = v
	}
	formattedFloat := strconv.FormatFloat(newRate, 'f', 5, 64)
	newSettings[key] = formattedFloat
	result.Source.Settings = newSettings
	a.logger.Infof("adapted sampling rate to: %s", formattedFloat)
	result.Source.Etag = fmt.Sprintf("%x", md5.Sum([]byte(formattedFloat)))
}

func (a *AgentConfig) newRate(rate float64) float64 {
	if a.decision != DecisionDownsample {
		return -1.0
	}

	now := time.Now()
	newRate := rate
	for expiry, factor := range a.downsamples {
		if now.Before(expiry) {
			newRate = newRate - newRate*factor
		}
	}
	if newRate < a.min {
		return a.min
	}
	return newRate
}
