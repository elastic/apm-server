package config

import (
	"testing"

	"github.com/elastic/beats/libbeat/common"

	"github.com/elastic/beats/libbeat/logp"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/elasticsearch"
	logs "github.com/elastic/apm-server/log"

	"github.com/stretchr/testify/assert"
)

func TestAPIKeyConfig(t *testing.T) {
	outputESCfg := common.MustNewConfigFrom(`{"hosts":["192.0.0.168:9200"]}`)

	t.Run("API Key disabled", func(t *testing.T) {
		var apiKeyCfg *APIKeyConfig
		assert.False(t, apiKeyCfg.IsEnabled())

		cfg := defaultAuthConfig()
		assert.False(t, cfg.APIKeyConfig.IsEnabled())

		require.NoError(t, cfg.setup(nil, nil, "", outputESCfg))
		assert.Equal(t, defaultAuthConfig(), cfg)
	})

	t.Run("secret token configured", func(t *testing.T) {
		cfg := defaultAuthConfig()
		require.NoError(t, cfg.setup(logp.NewLogger(logs.Config), nil, "abc", nil))
		assert.Equal(t, "abc", cfg.BearerToken)

		cfg.BearerToken = "configured"
		require.NoError(t, cfg.setup(logp.NewLogger(logs.Config), nil, "abc", nil))
		assert.Equal(t, "configured", cfg.BearerToken)
	})

	t.Run("ES config from output", func(t *testing.T) {
		cfg := defaultAuthConfig()
		cfg.APIKeyConfig.Enabled = true

		require.NoError(t, cfg.setup(nil, nil, "", outputESCfg))
		assert.Equal(t, elasticsearch.Hosts([]string{"192.0.0.168:9200"}), cfg.APIKeyConfig.ESConfig.Hosts)
	})

	t.Run("ES directly configured", func(t *testing.T) {
		cfg := defaultAuthConfig()
		cfg.APIKeyConfig.Enabled = true
		cfg.APIKeyConfig.ESConfig = &elasticsearch.Config{Hosts: []string{"172.10.0.1:9200"}}
		ucfg := common.MustNewConfigFrom(`{"authorization":{"api_key":{"elasticsearch":{"hosts":["172.10.0.1:9200"]}}}}`)
		cfg.APIKeyConfig.ESConfig = &elasticsearch.Config{Hosts: elasticsearch.Hosts{"172.10.0.123"}}

		require.NoError(t, cfg.setup(nil, ucfg, "", outputESCfg))
		assert.Equal(t, elasticsearch.Hosts([]string{"172.10.0.123"}), cfg.APIKeyConfig.ESConfig.Hosts)

	})

}
