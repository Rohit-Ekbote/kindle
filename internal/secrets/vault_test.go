package secrets_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emdash/kindle/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVaultFetcher_MissingAddr(t *testing.T) {
	t.Setenv("VAULT_ADDR", "")
	t.Setenv("VAULT_TOKEN", "tok")
	_, err := secrets.NewVaultFetcher([]string{"openai-key"})
	assert.ErrorContains(t, err, "VAULT_ADDR")
}

func TestVaultFetcher_MissingToken(t *testing.T) {
	t.Setenv("VAULT_ADDR", "http://vault:8200")
	t.Setenv("VAULT_TOKEN", "")
	_, err := secrets.NewVaultFetcher([]string{"openai-key"})
	assert.ErrorContains(t, err, "VAULT_TOKEN")
}

func TestVaultFetcher_FetchSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "mytoken", r.Header.Get("X-Vault-Token"))
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"data": map[string]any{"value": "sk-test123"},
			},
		})
	}))
	defer srv.Close()

	t.Setenv("VAULT_ADDR", srv.URL)
	t.Setenv("VAULT_TOKEN", "mytoken")
	t.Setenv("VAULT_SECRET_PATH_PREFIX", "secret/data")

	f, err := secrets.NewVaultFetcher([]string{"openai-key"})
	require.NoError(t, err)

	got, err := f.FetchSecrets(context.Background(), "my-env")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"openai-key": "sk-test123"}, got)
}

func TestVaultFetcher_SecretNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	t.Setenv("VAULT_ADDR", srv.URL)
	t.Setenv("VAULT_TOKEN", "tok")

	f, err := secrets.NewVaultFetcher([]string{"missing-secret"})
	require.NoError(t, err)

	_, err = f.FetchSecrets(context.Background(), "my-env")
	assert.ErrorContains(t, err, "missing-secret")
}
