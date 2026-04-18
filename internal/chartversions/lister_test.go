package chartversions_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/emdash/kindle/internal/chartversions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createLocalBareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "chart-repo.git")

	cmds := [][]string{
		{"git", "init", "--bare", repoDir},
		{"git", "-C", dir, "init", "work"},
		{"git", "-C", filepath.Join(dir, "work"), "commit", "--allow-empty", "-m", "init"},
		{"git", "-C", filepath.Join(dir, "work"), "tag", "v1.0.0"},
		{"git", "-C", filepath.Join(dir, "work"), "checkout", "-b", "staging"},
		{"git", "-C", filepath.Join(dir, "work"), "commit", "--allow-empty", "-m", "staging"},
		{"git", "-C", filepath.Join(dir, "work"), "remote", "add", "origin", repoDir},
		{"git", "-C", filepath.Join(dir, "work"), "push", "origin", "--all"},
		{"git", "-C", filepath.Join(dir, "work"), "push", "origin", "--tags"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "cmd: %v, output: %s", args, out)
	}
	return repoDir
}

func TestLister_List(t *testing.T) {
	repoURL := createLocalBareRepo(t)
	lister := chartversions.NewLister(repoURL)

	versions, err := lister.List()
	require.NoError(t, err)

	var names []string
	for _, v := range versions {
		names = append(names, v.Ref)
	}
	assert.Contains(t, names, "refs/tags/v1.0.0")
	assert.Contains(t, names, "refs/heads/staging")
}
