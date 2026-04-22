package secrets_test

import (
	"context"
	"testing"

	"github.com/emdash/kindle/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoOpFetcher_ReturnsEmptyMap(t *testing.T) {
	f := secrets.NewNoOpFetcher()
	got, err := f.FetchSecrets(context.Background(), "my-env")
	require.NoError(t, err)
	assert.Empty(t, got)
}
