package chartversions

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Version represents a single ref (tag or branch) from a remote Helm chart repository.
type Version struct {
	Ref  string
	SHA  string
	Kind string // "tag" or "branch"
}

// Lister fetches available versions from a remote Helm chart git repository.
type Lister struct {
	repoURL string
	gitPath string
}

// NewLister returns a Lister that queries the given repository URL.
// It resolves the git binary path once at construction time to prevent PATH injection.
func NewLister(repoURL string) (*Lister, error) {
	p, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("git not found: %w", err)
	}
	return &Lister{repoURL: repoURL, gitPath: p}, nil
}

// List calls git ls-remote and returns all tags and branches found in the repository.
// Returns an empty (non-nil) slice when the repository has no refs.
func (l *Lister) List(ctx context.Context) ([]Version, error) {
	cmd := exec.CommandContext(ctx, l.gitPath, "ls-remote", "--heads", "--tags", l.repoURL)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git ls-remote: %w: %s", err, stderr.String())
	}
	if stdout.Len() > 1<<20 { // 1MB
		return nil, fmt.Errorf("git ls-remote output too large (%d bytes)", stdout.Len())
	}
	return ParseRefs(stdout.String()), nil
}

// ParseRefs parses the output of git ls-remote into a slice of Versions.
func ParseRefs(output string) []Version {
	versions := []Version{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		sha, ref := parts[0], parts[1]
		// Skip dereferenced tag objects (e.g. refs/tags/v1.0.0^{})
		if strings.HasSuffix(ref, "^{}") {
			continue
		}
		kind := "branch"
		if strings.HasPrefix(ref, "refs/tags/") {
			kind = "tag"
		}
		versions = append(versions, Version{Ref: ref, SHA: sha, Kind: kind})
	}
	return versions
}
