package helm_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emdash/kindle/internal/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeFakeHelm(t *testing.T, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "helm")
	content := fmt.Sprintf("#!/bin/sh\necho \"helm $@\"\nexit %d\n", exitCode)
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))
	return script
}

func TestRunner_Upgrade_Success(t *testing.T) {
	kubeconfigFile := filepath.Join(t.TempDir(), "kubeconfig.yaml")
	require.NoError(t, os.WriteFile(kubeconfigFile, []byte("test"), 0600))

	r := helm.NewRunner(helm.RunnerConfig{HelmBin: makeFakeHelm(t, 0)})
	err := r.Upgrade(helm.UpgradeParams{
		ReleaseName:    "dev-alice",
		Namespace:      "dev-alice",
		ChartPath:      "/opt/chart-repo",
		ValuesFile:     "/data/envs/dev-alice/values.yaml",
		KubeconfigPath: kubeconfigFile,
		LogPath:        filepath.Join(t.TempDir(), "helm.log"),
	})
	assert.NoError(t, err)
}

func TestRunner_Upgrade_Failure(t *testing.T) {
	kubeconfigFile := filepath.Join(t.TempDir(), "kubeconfig.yaml")
	require.NoError(t, os.WriteFile(kubeconfigFile, []byte("test"), 0600))

	r := helm.NewRunner(helm.RunnerConfig{HelmBin: makeFakeHelm(t, 1)})
	err := r.Upgrade(helm.UpgradeParams{
		ReleaseName:    "dev-alice",
		Namespace:      "dev-alice",
		ChartPath:      "/opt/chart-repo",
		ValuesFile:     "/data/envs/dev-alice/values.yaml",
		KubeconfigPath: kubeconfigFile,
		LogPath:        filepath.Join(t.TempDir(), "helm.log"),
	})
	assert.Error(t, err)
}

func TestRunner_Upgrade_WithSecretValues(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "helm")
	captureFile := filepath.Join(dir, "args.txt")
	content := fmt.Sprintf("#!/bin/sh\necho \"$@\" > %s\nexit 0\n", captureFile)
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	kubeconfigFile := filepath.Join(t.TempDir(), "kubeconfig.yaml")
	require.NoError(t, os.WriteFile(kubeconfigFile, []byte("test"), 0600))

	r := helm.NewRunner(helm.RunnerConfig{HelmBin: script})
	err := r.Upgrade(helm.UpgradeParams{
		ReleaseName:    "dev-alice",
		Namespace:      "dev-alice",
		ChartPath:      "/opt/chart-repo",
		ValuesFile:     "/data/envs/dev-alice/values.yaml",
		KubeconfigPath: kubeconfigFile,
		LogPath:        filepath.Join(t.TempDir(), "helm.log"),
		SecretValues:   map[string]string{"openai-api-key": "sk-test"},
	})
	require.NoError(t, err)

	raw, err := os.ReadFile(captureFile)
	require.NoError(t, err)
	capturedArgs := strings.Fields(string(raw))

	// Count --values occurrences: one for ValuesFile, one for secrets temp file
	valuesCount := 0
	for _, a := range capturedArgs {
		if a == "--values" {
			valuesCount++
		}
	}
	assert.Equal(t, 2, valuesCount, "expected two --values flags when SecretValues provided")
}
