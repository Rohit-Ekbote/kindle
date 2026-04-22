package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type VaultFetcher struct {
	addr       string
	token      string
	pathPrefix string
	secrets    []string
}

func NewVaultFetcher(secrets []string) (*VaultFetcher, error) {
	addr := os.Getenv("VAULT_ADDR")
	if addr == "" {
		return nil, fmt.Errorf("VAULT_ADDR is required for vault backend")
	}
	token := os.Getenv("VAULT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("VAULT_TOKEN is required for vault backend")
	}
	prefix := os.Getenv("VAULT_SECRET_PATH_PREFIX")
	if prefix == "" {
		prefix = "secret/data"
	}
	return &VaultFetcher{addr: addr, token: token, pathPrefix: prefix, secrets: secrets}, nil
}

func (f *VaultFetcher) FetchSecrets(ctx context.Context, _ string) (map[string]string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	result := make(map[string]string, len(f.secrets))
	for _, name := range f.secrets {
		url := fmt.Sprintf("%s/v1/%s/%s", f.addr, f.pathPrefix, name)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build request for secret %q: %w", name, err)
		}
		req.Header.Set("X-Vault-Token", f.token)
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch secret %q: %w", name, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch secret %q: vault returned HTTP %d", name, resp.StatusCode)
		}
		var body struct {
			Data struct {
				Data map[string]any `json:"data"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("decode secret %q: %w", name, err)
		}
		val, ok := body.Data.Data["value"]
		if !ok {
			return nil, fmt.Errorf("secret %q is missing key \"value\" in vault response", name)
		}
		strVal, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("secret %q value is not a string", name)
		}
		result[name] = strVal
	}
	return result, nil
}
