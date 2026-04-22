package secrets_test

import (
	"os"
	"testing"

	"github.com/emdash/kindle/internal/secrets"
	"github.com/stretchr/testify/assert"
)

func TestGCPFetcher_MissingProjectID(t *testing.T) {
	orig := os.Getenv("GCP_PROJECT_ID")
	os.Unsetenv("GCP_PROJECT_ID")
	t.Cleanup(func() { os.Setenv("GCP_PROJECT_ID", orig) })

	_, err := secrets.NewGCPFetcher([]string{"openai-key"})
	assert.ErrorContains(t, err, "GCP_PROJECT_ID")
}

func TestGCPFetcher_EmptySecrets_NoError(t *testing.T) {
	t.Setenv("GCP_PROJECT_ID", "my-project")
	// NewGCPFetcher with empty list should succeed without dialing GCP
	f, err := secrets.NewGCPFetcher([]string{})
	assert.NoError(t, err)
	assert.NotNil(t, f)
}
