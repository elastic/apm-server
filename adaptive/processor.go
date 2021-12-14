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

import "github.com/elastic/beats/v7/libbeat/logp"

// Processor receives events on the decission channel and runs the configured
// actions.
type Processor struct {
	c       <-chan Decision
	logger  *logp.Logger
	actions []Action
}

// Action represents an action to be taken given an adaptive Decision.
type Action interface {
	// Name returns the action name.
	Name() string

	// Do performs performs the action given the decision.
	Do(Decision) error
}

// NewProcessor creates a new processor that will run all the configured actions
// for each of the adaptive decisions that are received.
func NewProcessor(c <-chan Decision, actions ...Action) *Processor {
	return &Processor{
		c:       c,
		actions: actions,
		logger:  logp.NewLogger("adaptive_processor"),
	}
}

// Run executes the processor, blocking execution until the channel is closed.
func (p *Processor) Run() {
	for decision := range p.c {
		for _, action := range p.actions {
			if err := action.Do(decision); err != nil {
				p.logger.Warn("failed running action %s: %v",
					action.Name(), err,
				)
			}
		}
	}
}
