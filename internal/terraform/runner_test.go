package terraform_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emdash/kindle/internal/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_Apply_WritesLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "apply.log")

	fakeScript := filepath.Join(dir, "terraform")
	require.NoError(t, os.WriteFile(fakeScript, []byte("#!/bin/sh\necho 'apply complete'\n"), 0755))

	runner := terraform.NewRunner(terraform.RunnerConfig{
		TerraformBin: fakeScript,
	})

	err := runner.Apply(dir, logPath)
	require.NoError(t, err)

	logContent, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(logContent), "apply complete")
}

func TestRunner_Apply_FailsOnNonZeroExit(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "apply.log")

	fakeScript := filepath.Join(dir, "terraform")
	require.NoError(t, os.WriteFile(fakeScript, []byte("#!/bin/sh\necho 'error' >&2\nexit 1\n"), 0755))

	runner := terraform.NewRunner(terraform.RunnerConfig{
		TerraformBin: fakeScript,
	})

	err := runner.Apply(dir, logPath)
	assert.Error(t, err)
}
