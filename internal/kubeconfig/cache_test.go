package kubeconfig_test

import (
	"testing"

	"github.com/emdash/kindle/internal/kubeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeFetcher struct {
	calls int
	data  []byte
}

func (f *fakeFetcher) Fetch(envName string) ([]byte, error) {
	f.calls++
	return f.data, nil
}

func TestCache_FetchesOnFirstCall(t *testing.T) {
	fetcher := &fakeFetcher{data: []byte("kubeconfig-content")}
	cache := kubeconfig.NewCache(fetcher.Fetch)

	data, err := cache.Get("dev-alice")
	require.NoError(t, err)
	assert.Equal(t, []byte("kubeconfig-content"), data)
	assert.Equal(t, 1, fetcher.calls)
}

func TestCache_ReturnsCachedOnSecondCall(t *testing.T) {
	fetcher := &fakeFetcher{data: []byte("kubeconfig-content")}
	cache := kubeconfig.NewCache(fetcher.Fetch)

	cache.Get("dev-alice")
	cache.Get("dev-alice")

	assert.Equal(t, 1, fetcher.calls, "should only fetch once")
}

func TestCache_InvalidateRefetches(t *testing.T) {
	fetcher := &fakeFetcher{data: []byte("kubeconfig-content")}
	cache := kubeconfig.NewCache(fetcher.Fetch)

	cache.Get("dev-alice")
	cache.Invalidate("dev-alice")
	cache.Get("dev-alice")

	assert.Equal(t, 2, fetcher.calls)
}

func TestPatchServerURL(t *testing.T) {
	input := []byte(`apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: default
`)
	output, err := kubeconfig.PatchServerURL(input, "dev-alice.local-dev.example.com")
	require.NoError(t, err)
	assert.Contains(t, string(output), "https://dev-alice.local-dev.example.com:6443")
	assert.NotContains(t, string(output), "127.0.0.1")
}
