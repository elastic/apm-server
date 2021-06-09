package cmd

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestCloudEnv(t *testing.T) {
	// no cloud environment variable set
	settings := DefaultSettings()
	assert.Len(t, settings.ConfigOverrides, 2)
	assert.Nil(t, settings.ConfigOverrides[1].Config)

	// cloud environment picked up
	os.Setenv(cloudEnv, "512")
	settings = DefaultSettings()
	assert.Len(t, settings.ConfigOverrides, 2)
	cfg := settings.ConfigOverrides[1].Config
	assert.NotNil(t, cfg)
	workers, err := cfg.Int("output.elasticsearch.worker", -1)
	require.NoError(t, err)
	assert.Equal(t, int64(5), workers)

	// bad cloud environment value
	os.Setenv(cloudEnv, "123")
	settings = DefaultSettings()
	assert.Len(t, settings.ConfigOverrides, 2)
	cfg  = settings.ConfigOverrides[1].Config
	assert.Nil(t, settings.ConfigOverrides[1].Config)
}
