package beater

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/go-ucfg"
)

type testUrlConfig struct {
	Hosts urls `config:"hosts"`
}

var defaultTestUrlConfig = testUrlConfig{
	Hosts: urls{&url.URL{Scheme: "http", Host: "elastic.co"}},
}

// TestConfigURLs tests common usage of "urls" configuration type, as if loaded from yml
func TestConfigURLs(t *testing.T) {
	cases := []struct {
		cfg map[string][]interface{}
		want,
		got *testUrlConfig
	}{
		{
			// use default
			cfg:  map[string][]interface{}{"hosts": nil},
			want: &defaultTestUrlConfig,
		},
		{
			// invalid URL
			cfg: map[string][]interface{}{"hosts": {"http://a b.com/"}},
		},
		{
			// single host in list
			cfg:  map[string][]interface{}{"hosts": {"http://one"}},
			want: &testUrlConfig{Hosts: urls{&url.URL{Scheme: "http", Host: "one"}}},
		},
		{
			// multiple hosts in list
			cfg: map[string][]interface{}{"hosts": {"http://one", "http://two"}},
			want: &testUrlConfig{Hosts: urls{
				&url.URL{Scheme: "http", Host: "one"},
				&url.URL{Scheme: "http", Host: "two"},
			}},
		},
		{
			// single host in list, multiple in default - ensure defaults are completely overwritten
			cfg:  map[string][]interface{}{"hosts": {"http://one"}},
			want: &testUrlConfig{Hosts: urls{&url.URL{Scheme: "http", Host: "one"}}},
			got:  &testUrlConfig{Hosts: urls{&url.URL{Host: "elastic.co"}, &url.URL{Host: "go.elastic.co"}}},
		},
	}

	for _, c := range cases {
		got := defaultTestUrlConfig
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
		got := defaultTestUrlConfig
		err := ucfg.MustNewFrom(map[string]interface{}{"hosts": "http://single"}).Unpack(&got)
		require.Error(t, err)
	}

	{
		// non-string instead of list of host strings
		got := defaultTestUrlConfig
		err := ucfg.MustNewFrom(map[string]int{"hosts": 5}).Unpack(&got)
		require.Error(t, err)
	}
}
