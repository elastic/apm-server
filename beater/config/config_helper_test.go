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

package config

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/go-ucfg"
)

type testURLConfig struct {
	Hosts urls `config:"hosts"`
}

var defaultTestURLConfig = testURLConfig{
	Hosts: urls{&url.URL{Scheme: "http", Host: "elastic.co"}},
}

// TestConfigURLs tests common usage of "urls" configuration type, as if loaded from yml
func TestConfigURLs(t *testing.T) {
	cases := []struct {
		cfg map[string][]interface{}
		want,
		got *testURLConfig
	}{
		{
			// use default
			cfg:  map[string][]interface{}{"hosts": nil},
			want: &defaultTestURLConfig,
		},
		{
			// invalid URL
			cfg: map[string][]interface{}{"hosts": {"http://a b.com/"}},
		},
		{
			// single host in list
			cfg:  map[string][]interface{}{"hosts": {"http://one"}},
			want: &testURLConfig{Hosts: urls{&url.URL{Scheme: "http", Host: "one"}}},
		},
		{
			// multiple hosts in list
			cfg: map[string][]interface{}{"hosts": {"http://one", "http://two"}},
			want: &testURLConfig{Hosts: urls{
				&url.URL{Scheme: "http", Host: "one"},
				&url.URL{Scheme: "http", Host: "two"},
			}},
		},
		{
			// single host in list, multiple in default - ensure defaults are completely overwritten
			cfg:  map[string][]interface{}{"hosts": {"http://one"}},
			want: &testURLConfig{Hosts: urls{&url.URL{Scheme: "http", Host: "one"}}},
			got:  &testURLConfig{Hosts: urls{&url.URL{Host: "elastic.co"}, &url.URL{Host: "go.elastic.co"}}},
		},
	}

	for _, c := range cases {
		got := defaultTestURLConfig
		if c.got != nil {
			got = *c.got
		}
		err := ucfg.MustNewFrom(c.cfg).Unpack(&got)
		if c.want == nil {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, c.want.Hosts, got.Hosts)
	}

	{
		// string instead of list of host strings
		got := defaultTestURLConfig
		err := ucfg.MustNewFrom(map[string]interface{}{"hosts": "http://single"}).Unpack(&got)
		require.Error(t, err)
	}

	{
		// non-string instead of list of host strings
		got := defaultTestURLConfig
		err := ucfg.MustNewFrom(map[string]int{"hosts": 5}).Unpack(&got)
		require.Error(t, err)
	}
}
