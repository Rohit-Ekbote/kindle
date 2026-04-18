package helm_test

import (
	"fmt"
	"os"
	"path/filepath"
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
