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

package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

var (
	ip  = "127.0.0.1"
	id  = "123"
	pid = 123
)

func TestContext(t *testing.T) {
	tests := []struct {
		process *Process
		system  *System
		service *Service
		user    *User
		context *Context
	}{
		{
			process: nil,
			system:  nil,
			service: nil,
			user:    nil,
			context: &Context{},
		},
		{
			process: &Process{},
			system:  &System{},
			service: &Service{},
			user:    &User{},
			context: &Context{
				process: common.MapStr{"pid": 0},
				service: common.MapStr{"name": "", "agent": common.MapStr{"version": "", "name": ""}},
				system:  common.MapStr{},
				user:    common.MapStr{},
			},
		},
		{
			process: &Process{Pid: pid},
			system:  &System{IP: &ip},
			service: &Service{Name: "service"},
			user:    &User{Id: &id},
			context: &Context{
				process: common.MapStr{"pid": 123},
				system:  common.MapStr{"ip": ip},
				service: common.MapStr{"name": "service", "agent": common.MapStr{"version": "", "name": ""}},
				user:    common.MapStr{"id": "123"},
			},
		},
	}

	for idx, te := range tests {
		ctx := NewContext(te.service, te.process, te.system, te.user)
		assert.Equal(t, te.context, ctx,
			fmt.Sprintf("<%v> Expected: %v, Actual: %v", idx, te.context, ctx))
	}

}

func TestContextTransform(t *testing.T) {

	tests := []struct {
		context *Context
		m       common.MapStr
		out     common.MapStr
	}{
		{
			context: &Context{},
			m:       common.MapStr{},
			out:     common.MapStr{},
		},
		{
			context: &Context{},
			m:       common.MapStr{"user": common.MapStr{"id": 123}},
			out:     common.MapStr{"user": common.MapStr{"id": 123}},
		},
		{
			context: &Context{
				process: common.MapStr{"pid": 123},
				system:  common.MapStr{"ip": ip},
				service: common.MapStr{"name": "service", "agent": common.MapStr{"version": "", "name": ""}},
				user:    common.MapStr{"id": 456},
			},
			m: common.MapStr{"foo": "bar", "user": common.MapStr{"id": 123, "username": "foo"}},
			out: common.MapStr{
				"foo":     "bar",
				"user":    common.MapStr{"id": 456, "username": "foo"},
				"process": common.MapStr{"pid": 123},
				"system":  common.MapStr{"ip": ip},
				"service": common.MapStr{"name": "service", "agent": common.MapStr{"version": "", "name": ""}},
			},
		},
	}

	for idx, te := range tests {
		out := te.context.Transform(te.m)
		assert.Equal(t, te.out, out,
			fmt.Sprintf("<%v> Expected: %v, Actual: %v", idx, te.out, out))
	}
}
