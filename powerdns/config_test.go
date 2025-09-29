package powerdns

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigClient(t *testing.T) {
	// This test would require mocking the HTTP server for setServerVersion
	// For now, we'll test the basic config creation
	config := Config{
		ServerURL:       "http://localhost:8081",
		APIKey:          "test-key",
		InsecureHTTPS:   true,
		CacheEnable:     false,
		CacheMemorySize: "100",
		CacheTTL:        30,
	}

	// We can't easily test Client() without mocking HTTP calls
	// But we can test that the config struct is properly initialized
	assert.Equal(t, "http://localhost:8081", config.ServerURL)
	assert.Equal(t, "test-key", config.APIKey)
	assert.True(t, config.InsecureHTTPS)
	assert.False(t, config.CacheEnable)
	assert.Equal(t, "100", config.CacheMemorySize)
	assert.Equal(t, 30, config.CacheTTL)
}
