package cmd

import (
	"os/exec"
	"strings"
)

// RepoRoot returns the git repository root for a working directory.
func RepoRoot(cwd string) (string, error) {
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
