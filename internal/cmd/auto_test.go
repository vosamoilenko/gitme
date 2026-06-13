package cmd

import (
	"testing"

	"github.com/vosamoilenko/gitme/internal/identity"
)

func TestDeriveIdentityFromPathSingleCandidate(t *testing.T) {
	ids := []identity.Identity{
		{Name: "GitHub A", Email: "a@example.com", Platform: identity.PlatformGitHub},
		{Name: "GitLab B", Email: "b@example.com", Platform: identity.PlatformGitLab},
	}

	got, _, ambiguous := deriveIdentityFromPath("/Users/test/Developer/github.com/acme/repo", ids)
	if ambiguous {
		t.Fatalf("expected non-ambiguous match")
	}
	if got == nil || got.Email != "a@example.com" {
		t.Fatalf("expected GitHub identity, got %+v", got)
	}
}

func TestDeriveIdentityFromPathAmbiguous(t *testing.T) {
	ids := []identity.Identity{
		{Name: "GitHub A", Email: "a@example.com", Platform: identity.PlatformGitHub},
		{Name: "GitHub B", Email: "b@example.com", Platform: identity.PlatformGitHub},
	}

	got, _, ambiguous := deriveIdentityFromPath("/Users/test/Developer/github.com/acme/repo", ids)
	if !ambiguous {
		t.Fatalf("expected ambiguous match")
	}
	if got != nil {
		t.Fatalf("expected nil identity for ambiguous match, got %+v", got)
	}
}
