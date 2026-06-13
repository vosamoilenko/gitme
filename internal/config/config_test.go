package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchesPattern(t *testing.T) {
	path := "/Users/test/Developer/github.com/acme/repo"

	if !matchesPattern(path, "github.com/acme") {
		t.Fatalf("expected boundary pattern to match")
	}
	if matchesPattern(path, "hub.com/ac") {
		t.Fatalf("expected partial fragment to not match")
	}
	if !matchesPattern(path, "/Users/test/Developer/github.com/acme/repo") {
		t.Fatalf("expected exact path pattern to match")
	}
}

func TestMatchesPatternTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	path := filepath.Join(home, "work", "repo")
	if !matchesPattern(path, "~/work") {
		t.Fatalf("expected ~ expansion pattern to match")
	}
}
