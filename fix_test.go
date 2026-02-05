package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo creates a temporary git repo with commits from different identities
func setupTestRepo(t *testing.T) string {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "gitme-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Initialize git repo
	runGit(t, tmpDir, "init")

	// Create commits with different identities
	commits := []struct {
		name    string
		email   string
		message string
	}{
		{"John Doe", "john@example.com", "First commit"},
		{"John Doe", "john@example.com", "Second commit"},
		{"John Doe", "johndoe@gmail.com", "Third commit with different email"},
		{"John Doe", "john@example.com", "Fourth commit"},
		{"John Doe", "john.doe@work.com", "Fifth commit from work"},
	}

	for i, c := range commits {
		// Create a file for each commit
		filename := filepath.Join(tmpDir, "file"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(filename, []byte(c.message), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}

		runGit(t, tmpDir, "add", ".")
		runGitWithEnv(t, tmpDir, []string{
			"GIT_AUTHOR_NAME=" + c.name,
			"GIT_AUTHOR_EMAIL=" + c.email,
			"GIT_COMMITTER_NAME=" + c.name,
			"GIT_COMMITTER_EMAIL=" + c.email,
		}, "commit", "-m", c.message)
	}

	return tmpDir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return runGitWithEnv(t, dir, nil, args...)
}

func runGitWithEnv(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, output)
	}
	return string(output)
}

// getCommitEmails returns all unique author emails in the repo
func getCommitEmails(t *testing.T, dir string) []string {
	t.Helper()
	cmd := exec.Command("git", "log", "--format=%ae")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get commit emails: %v", err)
	}

	seen := make(map[string]bool)
	var emails []string
	for _, line := range strings.Split(string(output), "\n") {
		email := strings.TrimSpace(line)
		if email != "" && !seen[email] {
			seen[email] = true
			emails = append(emails, email)
		}
	}
	return emails
}

// countCommitsByEmail returns the number of commits by a specific email
func countCommitsByEmail(t *testing.T, dir string, email string) int {
	t.Helper()
	cmd := exec.Command("git", "log", "--format=%ae")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to count commits: %v", err)
	}

	count := 0
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) == email {
			count++
		}
	}
	return count
}

func TestSetupTestRepo(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	emails := getCommitEmails(t, tmpDir)

	// Should have 3 unique emails
	if len(emails) != 3 {
		t.Errorf("Expected 3 unique emails, got %d: %v", len(emails), emails)
	}

	// Check commit counts
	if count := countCommitsByEmail(t, tmpDir, "john@example.com"); count != 3 {
		t.Errorf("Expected 3 commits from john@example.com, got %d", count)
	}
	if count := countCommitsByEmail(t, tmpDir, "johndoe@gmail.com"); count != 1 {
		t.Errorf("Expected 1 commit from johndoe@gmail.com, got %d", count)
	}
	if count := countCommitsByEmail(t, tmpDir, "john.doe@work.com"); count != 1 {
		t.Errorf("Expected 1 commit from john.doe@work.com, got %d", count)
	}
}

func TestRewriteAuthor(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Verify initial state
	if count := countCommitsByEmail(t, tmpDir, "johndoe@gmail.com"); count != 1 {
		t.Fatalf("Expected 1 commit from johndoe@gmail.com before rewrite, got %d", count)
	}

	// Rewrite johndoe@gmail.com -> john@example.com
	err := rewriteAuthor(tmpDir, "johndoe@gmail.com", "John Doe", "john@example.com")
	if err != nil {
		t.Fatalf("rewriteAuthor failed: %v", err)
	}

	// Verify the email was changed
	if count := countCommitsByEmail(t, tmpDir, "johndoe@gmail.com"); count != 0 {
		t.Errorf("Expected 0 commits from johndoe@gmail.com after rewrite, got %d", count)
	}
	if count := countCommitsByEmail(t, tmpDir, "john@example.com"); count != 4 {
		t.Errorf("Expected 4 commits from john@example.com after rewrite, got %d", count)
	}
}

func TestRewriteAuthorMultiple(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Rewrite both alternate emails to the main one
	err := rewriteAuthor(tmpDir, "johndoe@gmail.com", "John Doe", "john@example.com")
	if err != nil {
		t.Fatalf("First rewriteAuthor failed: %v", err)
	}

	err = rewriteAuthor(tmpDir, "john.doe@work.com", "John Doe", "john@example.com")
	if err != nil {
		t.Fatalf("Second rewriteAuthor failed: %v", err)
	}

	// All commits should now be from john@example.com
	emails := getCommitEmails(t, tmpDir)
	if len(emails) != 1 {
		t.Errorf("Expected 1 unique email after rewrite, got %d: %v", len(emails), emails)
	}
	if emails[0] != "john@example.com" {
		t.Errorf("Expected all commits to be from john@example.com, got %s", emails[0])
	}
	if count := countCommitsByEmail(t, tmpDir, "john@example.com"); count != 5 {
		t.Errorf("Expected 5 commits from john@example.com, got %d", count)
	}
}

func TestRewriteAuthorNonExistent(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Try to rewrite an email that doesn't exist - should not error but do nothing
	err := rewriteAuthor(tmpDir, "nonexistent@example.com", "Nobody", "john@example.com")
	if err != nil {
		t.Fatalf("rewriteAuthor should not fail for non-existent email: %v", err)
	}

	// Commit counts should be unchanged
	if count := countCommitsByEmail(t, tmpDir, "john@example.com"); count != 3 {
		t.Errorf("Expected 3 commits from john@example.com (unchanged), got %d", count)
	}
}

func TestRewriteAuthorPreservesCommitCount(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Count total commits before
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = tmpDir
	output, _ := cmd.Output()
	beforeCount := strings.TrimSpace(string(output))

	// Rewrite
	err := rewriteAuthor(tmpDir, "johndoe@gmail.com", "John Doe", "john@example.com")
	if err != nil {
		t.Fatalf("rewriteAuthor failed: %v", err)
	}

	// Count total commits after
	cmd = exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = tmpDir
	output, _ = cmd.Output()
	afterCount := strings.TrimSpace(string(output))

	if beforeCount != afterCount {
		t.Errorf("Commit count changed: before=%s, after=%s", beforeCount, afterCount)
	}
}

// rewriteAuthor is now in main.go
