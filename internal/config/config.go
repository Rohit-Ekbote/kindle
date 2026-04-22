// Package config loads and validates server configuration from environment variables.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	GoogleClientID     string
	GoogleClientSecret string
	CorporateDomain    string
	ZoneDomain         string
	GCPProjectID       string
	ChartRepoURL       string
	SessionSecret      string
	DataDir            string
	PresetsDir         string
	Port               string
}

func Load() (*Config, error) {
	c := &Config{
		DataDir:    envOrDefault("DATA_DIR", "./data"),
		PresetsDir: envOrDefault("PRESETS_DIR", "./presets"),
		Port:       envOrDefault("PORT", "8080"),
	}
	required := []struct {
		key string
		dst *string
	}{
		{"GOOGLE_CLIENT_ID", &c.GoogleClientID},
		{"GOOGLE_CLIENT_SECRET", &c.GoogleClientSecret},
		{"CORPORATE_DOMAIN", &c.CorporateDomain},
		{"ZONE_DOMAIN", &c.ZoneDomain},
		{"GCP_PROJECT_ID", &c.GCPProjectID},
		{"CHART_REPO_URL", &c.ChartRepoURL},
		{"SESSION_SECRET", &c.SessionSecret},
	}
	for _, r := range required {
		v := os.Getenv(r.key)
		if v == "" {
			return nil, fmt.Errorf("required env var %s is not set", r.key)
		}
		*r.dst = v
	}
	return c, nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
