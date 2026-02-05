package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vosamoilenko/gitme/internal/config"
)

// FixScan shows commits by your identities in current repo
func FixScan() {
	cwd, _ := os.Getwd()

	gitDir := filepath.Join(cwd, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: not a git repository\n")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	knownEmails := make(map[string]bool)
	for _, id := range cfg.Identities {
		knownEmails[strings.ToLower(id.Email)] = true
	}

	cmd := exec.Command("git", "log", "--format=%H|%an|%ae")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running git log: %v\n", err)
		os.Exit(1)
	}

	type commitInfo struct {
		name  string
		email string
		count int
	}
	identityCounts := make(map[string]*commitInfo)

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		name := parts[1]
		email := parts[2]
		emailLower := strings.ToLower(email)

		if !knownEmails[emailLower] {
			continue
		}

		key := emailLower
		if _, ok := identityCounts[key]; !ok {
			identityCounts[key] = &commitInfo{name: name, email: email, count: 0}
		}
		identityCounts[key].count++
	}

	if len(identityCounts) == 0 {
		fmt.Println("No commits found from your known identities in this repo.")
		return
	}

	var configuredEmail string
	cmdEmail := exec.Command("git", "config", "user.email")
	cmdEmail.Dir = cwd
	if out, err := cmdEmail.Output(); err == nil {
		configuredEmail = strings.ToLower(strings.TrimSpace(string(out)))
	}

	fmt.Println(HeaderStyle.Render("Commits by your identities in this repo:"))
	fmt.Println()

	for _, info := range identityCounts {
		marker := ""
		emailLower := strings.ToLower(info.email)
		if emailLower == configuredEmail {
			marker = " " + SuccessStyle.Render("(current)")
		}
		fmt.Printf("  %s <%s>%s\n", info.name, info.email, marker)
		fmt.Printf("    %s\n", DimStyle.Render(fmt.Sprintf("%d commits", info.count)))
	}

	if len(identityCounts) > 1 {
		fmt.Println()
		fmt.Println(DimStyle.Render("To rewrite history, use:"))
		fmt.Println(DimStyle.Render("  gitme fix:rewrite <old-email> <new-email>"))
	}
}

// FixRewrite rewrites commits from old email to new email
func FixRewrite() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: gitme fix:rewrite <old-email> <new-email>\n")
		os.Exit(1)
	}

	cwd, _ := os.Getwd()

	gitDir := filepath.Join(cwd, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: not a git repository\n")
		os.Exit(1)
	}

	oldEmail := os.Args[2]
	newEmail := os.Args[3]

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	var newName string
	for _, id := range cfg.Identities {
		if strings.EqualFold(id.Email, newEmail) {
			newName = id.Name
			break
		}
	}
	if newName == "" {
		fmt.Fprintf(os.Stderr, "Error: %s is not a known identity\n", newEmail)
		fmt.Fprintf(os.Stderr, "Add it first with: gitme add \"Name\" \"%s\"\n", newEmail)
		os.Exit(1)
	}

	cmd := exec.Command("git", "log", "--format=%ae")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running git log: %v\n", err)
		os.Exit(1)
	}

	count := 0
	for _, line := range strings.Split(string(output), "\n") {
		if strings.EqualFold(strings.TrimSpace(line), oldEmail) {
			count++
		}
	}

	if count == 0 {
		fmt.Printf("No commits found from %s\n", oldEmail)
		return
	}

	fmt.Println(HeaderStyle.Render("Rewrite plan:"))
	fmt.Println()
	fmt.Printf("  From: %s\n", oldEmail)
	fmt.Printf("  To:   %s <%s>\n", newName, newEmail)
	fmt.Printf("  Commits to rewrite: %d\n", count)
	fmt.Println()
	fmt.Println(WarnStyle.Render("WARNING: This rewrites git history!"))
	fmt.Println(DimStyle.Render("You will need to force push after this."))
	fmt.Println()
	fmt.Print("Continue? [y/N] ")

	var response string
	fmt.Scanln(&response)
	if strings.ToLower(response) != "y" {
		fmt.Println("Aborted.")
		return
	}

	fmt.Println()
	fmt.Println("Rewriting commits...")

	err = RewriteAuthor(cwd, oldEmail, newName, newEmail)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rewriting history: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(SuccessStyle.Render("Done!"))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println(DimStyle.Render("  git push --force-with-lease"))
}

// RewriteAuthor rewrites commits from oldEmail to newName/newEmail using git filter-branch
func RewriteAuthor(repoPath, oldEmail, newName, newEmail string) error {
	script := `
if [ "$GIT_COMMITTER_EMAIL" = "` + oldEmail + `" ]; then
    export GIT_COMMITTER_NAME="` + newName + `"
    export GIT_COMMITTER_EMAIL="` + newEmail + `"
fi
if [ "$GIT_AUTHOR_EMAIL" = "` + oldEmail + `" ]; then
    export GIT_AUTHOR_NAME="` + newName + `"
    export GIT_AUTHOR_EMAIL="` + newEmail + `"
fi
`
	cmd := exec.Command("git", "filter-branch", "-f", "--env-filter", script, "--", "--all")
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "FILTER_BRANCH_SQUELCH_WARNING=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "nothing to rewrite") ||
			strings.Contains(string(output), "Found nothing to rewrite") {
			return nil
		}
		return fmt.Errorf("%v: %s", err, output)
	}
	return nil
}
