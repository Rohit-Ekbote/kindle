package chartversions

import (
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
}

// NewLister returns a Lister that queries the given repository URL.
func NewLister(repoURL string) *Lister {
	return &Lister{repoURL: repoURL}
}

// List calls git ls-remote and returns all tags and branches found in the repository.
// Returns an empty (non-nil) slice when the repository has no refs.
func (l *Lister) List() ([]Version, error) {
	out, err := exec.Command("git", "ls-remote", "--heads", "--tags", l.repoURL).Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-remote: %w", err)
	}
	return parseRefs(string(out)), nil
}

func parseRefs(output string) []Version {
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
