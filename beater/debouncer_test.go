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

package beater

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestDebouncer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fired := make(chan struct{})
	d := &debouncer{
		active:   make(chan struct{}, 1),
		triggerc: make(chan chan<- error),
		timeout:  50 * time.Millisecond,
		fn: func() error {
			close(fired)
			return errors.New("err")
		},
	}

	c1 := d.trigger(ctx)
	select {
	case <-fired:
		t.Fatal("didn't debounce")
	case <-time.After(30 * time.Millisecond):
	}
	c2 := d.trigger(ctx)
	select {
	case <-fired:
		t.Fatal("didn't debounce")
	case <-time.After(30 * time.Millisecond):
	}
	c3 := d.trigger(ctx)
	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("fn did not fire")
	}

	assert.Error(t, <-c1)
	assert.Error(t, <-c2)
	assert.Error(t, <-c3)
}
