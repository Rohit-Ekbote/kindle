package secrets

import (
	"context"
	_ "embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed secrets.yaml
var configData []byte

type SecretFetcher interface {
	FetchSecrets(ctx context.Context, envName string) (map[string]string, error)
}

type secretsConfig struct {
	Backend string   `yaml:"backend"`
	Secrets []string `yaml:"secrets"`
}

func NewFetcherFromConfig() (SecretFetcher, error) {
	var cfg secretsConfig
	if err := yaml.Unmarshal(configData, &cfg); err != nil {
		return nil, fmt.Errorf("parse secrets config: %w", err)
	}
	switch cfg.Backend {
	case "vault":
		return NewVaultFetcher(cfg.Secrets)
	case "gcp":
		return NewGCPFetcher(cfg.Secrets)
	case "none", "":
		return NewNoOpFetcher(), nil
	default:
		return nil, fmt.Errorf("unknown secret backend: %q", cfg.Backend)
	}
}

// Temporary stub — replaced by gcp.go in Task 4
func NewGCPFetcher(secrets []string) (SecretFetcher, error) {
	return nil, fmt.Errorf("gcp backend not yet implemented")
}
