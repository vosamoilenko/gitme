package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRepoRootFromSubdirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitme-reporoot-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v (%s)", err, out)
	}

	subdir := filepath.Join(tmpDir, "a", "b")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	root, err := RepoRoot(subdir)
	if err != nil {
		t.Fatalf("RepoRoot returned error: %v", err)
	}

	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("failed to resolve root path: %v", err)
	}
	canonicalTmp, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve temp path: %v", err)
	}
	if canonicalRoot != canonicalTmp {
		t.Fatalf("expected root %q, got %q", canonicalTmp, canonicalRoot)
	}
}
