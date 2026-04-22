package config_test

import (
	"os"
	"testing"

	"github.com/emdash/kindle/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_MissingRequired(t *testing.T) {
	os.Clearenv()
	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GOOGLE_CLIENT_ID")
}

func TestLoad_AllRequired(t *testing.T) {
	os.Clearenv()
	os.Setenv("GOOGLE_CLIENT_ID", "client-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "client-secret")
	os.Setenv("CORPORATE_DOMAIN", "example.com")
	os.Setenv("ZONE_DOMAIN", "local-dev.example.com")
	os.Setenv("GCP_PROJECT_ID", "my-project")
	os.Setenv("CHART_REPO_URL", "https://github.com/org/repo")
	os.Setenv("SESSION_SECRET", "secret")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "client-id", cfg.GoogleClientID)
	assert.Equal(t, "./data", cfg.DataDir)       // default
	assert.Equal(t, "./presets", cfg.PresetsDir) // default
	assert.Equal(t, "8080", cfg.Port)            // default
}

func TestLoad_OverrideDefaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("GOOGLE_CLIENT_ID", "client-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "client-secret")
	os.Setenv("CORPORATE_DOMAIN", "example.com")
	os.Setenv("ZONE_DOMAIN", "local-dev.example.com")
	os.Setenv("GCP_PROJECT_ID", "my-project")
	os.Setenv("CHART_REPO_URL", "https://github.com/org/repo")
	os.Setenv("SESSION_SECRET", "secret")
	os.Setenv("DATA_DIR", "/custom/data")
	os.Setenv("PORT", "9090")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "/custom/data", cfg.DataDir)
	assert.Equal(t, "9090", cfg.Port)
}
