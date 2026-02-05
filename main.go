package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vosamoilenko/gitme/internal/config"
	"github.com/vosamoilenko/gitme/internal/identity"
	"github.com/vosamoilenko/gitme/internal/ui"
)

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)

type mixedRepo struct {
	path       string
	identities []string
}

func main() {
	if len(os.Args) < 2 {
		runTUI()
		return
	}

	switch os.Args[1] {
	case "list", "ls":
		cmdList()
	case "add":
		cmdAdd()
	case "remove", "rm":
		cmdRemove()
	case "scan", "refresh":
		cmdScan()
	case "reset":
		cmdReset()
	case "repos":
		cmdRepos()
	case "mixed":
		cmdMixed()
	case "fix:scan":
		cmdFixScan()
	case "fix:rewrite":
		cmdFixRewrite()
	case "current", "whoami":
		cmdCurrent()
	case "set":
		cmdSet()
	case "help", "-h", "--help":
		cmdHelp()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		cmdHelp()
		os.Exit(1)
	}
}

func cmdHelp() {
	fmt.Println(headerStyle.Render("gitme") + " - Git identity switcher")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gitme              Interactive TUI (enter=select, d=delete, r=rescan)")
	fmt.Println("  gitme list         List all known identities")
	fmt.Println("  gitme repos        Show all repos and which identity they use")
	fmt.Println("  gitme mixed        Show repos with multiple identities in history")
	fmt.Println("  gitme fix:scan     Show commits by your identities in current repo")
	fmt.Println("  gitme fix:rewrite <old> <new>  Rewrite commits from old to new email")
	fmt.Println("  gitme add          Add a new identity interactively")
	fmt.Println("  gitme add <n> <e>  Add identity with name and email")
	fmt.Println("  gitme remove <#|e> Remove identity by number or email")
	fmt.Println("  gitme scan         Rescan machine for git identities")
	fmt.Println("  gitme reset        Delete config and rescan from scratch")
	fmt.Println("  gitme current      Show current identity for this folder")
	fmt.Println("  gitme set <email>  Set identity by email (no TUI)")
	fmt.Println("  gitme help         Show this help")
	fmt.Println()
	fmt.Println("Aliases: ls=list, rm=remove, whoami=current, refresh=scan")
	fmt.Println()
	fmt.Println("Config stored in: ~/.config/gitme/config.json")
}

func cmdList() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Scan for new identities
	scanned, _ := identity.Scan()
	cfg.UpdateIdentities(scanned)
	cfg.Save()

	if len(cfg.Identities) == 0 {
		fmt.Println("No identities found.")
		fmt.Println("Add one with: gitme add \"Your Name\" \"your@email.com\"")
		return
	}

	fmt.Println(headerStyle.Render("Identities:"))
	fmt.Println()
	for i, id := range cfg.Identities {
		// Platform icon
		platformIcon := ""
		switch id.Platform {
		case identity.PlatformGitHub:
			platformIcon = "[GitHub] "
		case identity.PlatformGitLab:
			platformIcon = "[GitLab] "
		case identity.PlatformBitbucket:
			platformIcon = "[Bitbucket] "
		}

		fmt.Printf("  %d. %s%s <%s>\n", i+1, platformIcon, id.Name, id.Email)
		// Show ALL sources where this identity was found
		if len(id.Sources) > 0 {
			for _, src := range id.Sources {
				fmt.Printf("     %s\n", dimStyle.Render(src))
			}
		} else if id.Source != "" {
			fmt.Printf("     %s\n", dimStyle.Render(id.Source))
		}
	}

	// Show folder mappings
	if len(cfg.FolderIdentities) > 0 {
		fmt.Println()
		fmt.Println(headerStyle.Render("Folder mappings:"))
		fmt.Println()
		for folder, id := range cfg.FolderIdentities {
			fmt.Printf("  %s\n", folder)
			fmt.Printf("     %s\n", dimStyle.Render(id.Email))
		}
	}
}

func cmdAdd() {
	var name, email string

	if len(os.Args) >= 4 {
		name = os.Args[2]
		email = os.Args[3]
	} else {
		fmt.Print("Name: ")
		fmt.Scanln(&name)
		fmt.Print("Email: ")
		fmt.Scanln(&email)
	}

	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)

	if name == "" || email == "" {
		fmt.Fprintf(os.Stderr, "Both name and email are required\n")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	newId := identity.Identity{
		Name:   name,
		Email:  email,
		Source: "manual",
	}

	// Check if already exists
	for _, id := range cfg.Identities {
		if id.Email == email {
			fmt.Printf("Identity with email %s already exists\n", email)
			os.Exit(1)
		}
	}

	cfg.Identities = append(cfg.Identities, newId)
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(successStyle.Render("Added:"), name, "<"+email+">")
}

func cmdRemove() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: gitme remove <number|email>\n")
		fmt.Fprintf(os.Stderr, "  gitme rm 3        Remove identity #3\n")
		fmt.Fprintf(os.Stderr, "  gitme rm gmail    Remove by partial email match\n")
		os.Exit(1)
	}

	arg := os.Args[2]

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Check if arg is a number (index)
	var removeIndex int = -1
	if idx, err := fmt.Sscanf(arg, "%d", &removeIndex); err == nil && idx == 1 {
		removeIndex-- // Convert to 0-based index
		if removeIndex < 0 || removeIndex >= len(cfg.Identities) {
			fmt.Fprintf(os.Stderr, "Invalid index: %s (valid: 1-%d)\n", arg, len(cfg.Identities))
			os.Exit(1)
		}
	}

	// If using partial match, first check how many match
	if removeIndex < 0 {
		var matches []int
		for i, id := range cfg.Identities {
			if id.Email == arg || strings.Contains(id.Email, arg) {
				matches = append(matches, i)
			}
		}

		if len(matches) == 0 {
			fmt.Fprintf(os.Stderr, "No identity found matching: %s\n", arg)
			fmt.Fprintf(os.Stderr, "Run 'gitme list' to see all identities\n")
			os.Exit(1)
		}

		if len(matches) > 1 {
			fmt.Fprintf(os.Stderr, "Multiple identities match '%s':\n\n", arg)
			for _, idx := range matches {
				id := cfg.Identities[idx]
				fmt.Fprintf(os.Stderr, "  %d. %s <%s>\n", idx+1, id.Name, id.Email)
			}
			fmt.Fprintf(os.Stderr, "\nUse the number to remove a specific one: gitme rm %d\n", matches[0]+1)
			os.Exit(1)
		}

		// Exactly one match
		removeIndex = matches[0]
	}

	// Remove the identity at removeIndex
	removed := cfg.Identities[removeIndex]
	newIdentities := append(cfg.Identities[:removeIndex], cfg.Identities[removeIndex+1:]...)

	fmt.Println(successStyle.Render("Removed:"), removed.Name, "<"+removed.Email+">")
	if removed.Source != "" {
		fmt.Println(dimStyle.Render("  was at: " + removed.Source))
	}

	cfg.Identities = newIdentities
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}
}

func cmdScan() {
	fmt.Println("Scanning for git identities...")

	// Clear existing identities and rescan
	scanned, err := identity.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Keep manual identities, replace scanned ones
	manualIdentities := []identity.Identity{}
	for _, id := range cfg.Identities {
		if id.Source == "manual" {
			manualIdentities = append(manualIdentities, id)
		}
	}

	// Start fresh with scanned + manual
	cfg.Identities = scanned
	for _, id := range manualIdentities {
		// Add manual ones if not already found
		found := false
		for _, s := range scanned {
			if s.Email == id.Email {
				found = true
				break
			}
		}
		if !found {
			cfg.Identities = append(cfg.Identities, id)
		}
	}

	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(successStyle.Render(fmt.Sprintf("Found %d identities", len(cfg.Identities))))
	fmt.Println()
	for i, id := range cfg.Identities {
		platformIcon := ""
		switch id.Platform {
		case identity.PlatformGitHub:
			platformIcon = "[GitHub] "
		case identity.PlatformGitLab:
			platformIcon = "[GitLab] "
		case identity.PlatformBitbucket:
			platformIcon = "[Bitbucket] "
		}
		fmt.Printf("  %d. %s%s <%s>\n", i+1, platformIcon, id.Name, id.Email)
		// Show ALL sources where this identity was found
		if len(id.Sources) > 0 {
			for _, src := range id.Sources {
				fmt.Printf("     %s\n", dimStyle.Render(src))
			}
		} else if id.Source != "" {
			fmt.Printf("     %s\n", dimStyle.Render(id.Source))
		}
	}
}

func cmdReset() {
	fmt.Println("Deleting config and rescanning...")

	if err := config.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting config: %v\n", err)
		os.Exit(1)
	}

	// Now rescan
	scanned, err := identity.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	cfg.Identities = scanned
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(successStyle.Render(fmt.Sprintf("Found %d identities", len(cfg.Identities))))
	fmt.Println()
	for i, id := range cfg.Identities {
		platformIcon := ""
		switch id.Platform {
		case identity.PlatformGitHub:
			platformIcon = "[GitHub] "
		case identity.PlatformGitLab:
			platformIcon = "[GitLab] "
		case identity.PlatformBitbucket:
			platformIcon = "[Bitbucket] "
		}
		fmt.Printf("  %d. %s%s <%s>\n", i+1, platformIcon, id.Name, id.Email)
	}
}

func cmdRepos() {
	home, _ := os.UserHomeDir()

	// Get global identity
	globalEmail := ""
	globalName := ""
	globalConfig := filepath.Join(home, ".gitconfig")
	if data, err := os.ReadFile(globalConfig); err == nil {
		lines := strings.Split(string(data), "\n")
		inUser := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "[user]" {
				inUser = true
				continue
			}
			if strings.HasPrefix(line, "[") {
				inUser = false
			}
			if inUser {
				if strings.HasPrefix(line, "email") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						globalEmail = strings.TrimSpace(parts[1])
					}
				}
				if strings.HasPrefix(line, "name") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						globalName = strings.TrimSpace(parts[1])
					}
				}
			}
		}
	}

	globalIdentity := fmt.Sprintf("%s <%s>", globalName, globalEmail)

	// Map of identity -> list of repo names
	reposByIdentity := make(map[string][]string)
	// Track order of identities (global first)
	identityOrder := []string{globalIdentity}

	workspaceDirs := []string{
		filepath.Join(home, "Developer"),
		filepath.Join(home, "Projects"),
		filepath.Join(home, "Code"),
		filepath.Join(home, "workspace"),
		filepath.Join(home, "src"),
		filepath.Join(home, "work"),
	}

	for _, dir := range workspaceDirs {
		if _, err := os.Stat(dir); err == nil {
			collectRepos(dir, 4, globalIdentity, reposByIdentity, &identityOrder)
		}
	}

	fmt.Println(headerStyle.Render("All repositories:"))
	fmt.Println()

	for _, ident := range identityOrder {
		repos := reposByIdentity[ident]
		if len(repos) == 0 {
			continue
		}
		fmt.Printf("%s\n", ident)
		for _, repo := range repos {
			fmt.Printf("  %s\n", dimStyle.Render(repo))
		}
		fmt.Println()
	}
}

func collectRepos(dir string, maxDepth int, globalIdentity string, reposByIdentity map[string][]string, identityOrder *[]string) {
	if maxDepth <= 0 {
		return
	}

	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		subdir := filepath.Join(dir, entry.Name())
		gitDir := filepath.Join(subdir, ".git")

		if _, err := os.Stat(gitDir); err == nil {
			// Found a repo - check if it has local user config
			configPath := filepath.Join(gitDir, "config")
			localEmail := ""
			localName := ""

			if data, err := os.ReadFile(configPath); err == nil {
				lines := strings.Split(string(data), "\n")
				inUser := false
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "[user]" {
						inUser = true
						continue
					}
					if strings.HasPrefix(line, "[") {
						inUser = false
					}
					if inUser {
						if strings.HasPrefix(line, "email") {
							parts := strings.SplitN(line, "=", 2)
							if len(parts) == 2 {
								localEmail = strings.TrimSpace(parts[1])
							}
						}
						if strings.HasPrefix(line, "name") {
							parts := strings.SplitN(line, "=", 2)
							if len(parts) == 2 {
								localName = strings.TrimSpace(parts[1])
							}
						}
					}
				}
			}

			repoName := filepath.Base(subdir)
			identity := globalIdentity
			if localEmail != "" {
				identity = fmt.Sprintf("%s <%s>", localName, localEmail)
				// Add to order if new
				found := false
				for _, id := range *identityOrder {
					if id == identity {
						found = true
						break
					}
				}
				if !found {
					*identityOrder = append(*identityOrder, identity)
				}
			}
			reposByIdentity[identity] = append(reposByIdentity[identity], repoName)
		}

		if maxDepth > 1 {
			collectRepos(subdir, maxDepth-1, globalIdentity, reposByIdentity, identityOrder)
		}
	}
}

func cmdMixed() {
	home, _ := os.UserHomeDir()

	// Load known identities
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Build set of known emails (lowercase for comparison)
	knownEmails := make(map[string]string) // lowercase email -> display identity
	for _, id := range cfg.Identities {
		key := strings.ToLower(id.Email)
		knownEmails[key] = fmt.Sprintf("%s <%s>", id.Name, id.Email)
	}

	if len(knownEmails) < 2 {
		fmt.Println("You need at least 2 identities configured to check for mixed repos.")
		return
	}

	workspaceDirs := []string{
		filepath.Join(home, "Developer"),
		filepath.Join(home, "Projects"),
		filepath.Join(home, "Code"),
		filepath.Join(home, "workspace"),
		filepath.Join(home, "src"),
		filepath.Join(home, "work"),
	}

	var mixed []mixedRepo

	for _, dir := range workspaceDirs {
		if _, err := os.Stat(dir); err == nil {
			findMixedRepos(dir, 4, knownEmails, &mixed)
		}
	}

	if len(mixed) == 0 {
		fmt.Println("No repos with mixed identities found.")
		return
	}

	fmt.Println(headerStyle.Render("Repos with multiple identities:"))
	fmt.Println()

	for _, repo := range mixed {
		fmt.Printf("%s\n", repo.path)
		for _, id := range repo.identities {
			fmt.Printf("  %s\n", dimStyle.Render(id))
		}
		fmt.Println()
	}
}

func findMixedRepos(dir string, maxDepth int, knownEmails map[string]string, mixed *[]mixedRepo) {
	if maxDepth <= 0 {
		return
	}

	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		subdir := filepath.Join(dir, entry.Name())
		gitDir := filepath.Join(subdir, ".git")

		if _, err := os.Stat(gitDir); err == nil {
			// Found a repo - get unique author emails from git log
			cmd := exec.Command("git", "-C", subdir, "log", "--format=%ae")
			output, err := cmd.Output()
			if err != nil {
				continue
			}

			// Find which of YOUR identities are used in this repo
			foundIdentities := make(map[string]bool)
			for _, line := range strings.Split(string(output), "\n") {
				email := strings.ToLower(strings.TrimSpace(line))
				if displayIdentity, ok := knownEmails[email]; ok {
					foundIdentities[displayIdentity] = true
				}
			}

			// Only show if 2+ of your identities are used
			if len(foundIdentities) > 1 {
				var identities []string
				for id := range foundIdentities {
					identities = append(identities, id)
				}
				*mixed = append(*mixed, mixedRepo{
					path:       subdir,
					identities: identities,
				})
			}
		}

		if maxDepth > 1 {
			findMixedRepos(subdir, maxDepth-1, knownEmails, mixed)
		}
	}
}

func cmdFixScan() {
	cwd, _ := os.Getwd()

	// Check if we're in a git repo
	gitDir := filepath.Join(cwd, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: not a git repository\n")
		os.Exit(1)
	}
	// Load known identities
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Build set of known emails
	knownEmails := make(map[string]bool)
	for _, id := range cfg.Identities {
		knownEmails[strings.ToLower(id.Email)] = true
	}

	// Get all commits with author info
	cmd := exec.Command("git", "log", "--format=%H|%an|%ae")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running git log: %v\n", err)
		os.Exit(1)
	}

	// Count commits per identity (only your identities)
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

		// Only count your identities
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

	// Get current repo's configured identity
	var configuredEmail string
	cmdEmail := exec.Command("git", "config", "user.email")
	cmdEmail.Dir = cwd
	if out, err := cmdEmail.Output(); err == nil {
		configuredEmail = strings.ToLower(strings.TrimSpace(string(out)))
	}

	fmt.Println(headerStyle.Render("Commits by your identities in this repo:"))
	fmt.Println()

	for _, info := range identityCounts {
		marker := ""
		emailLower := strings.ToLower(info.email)
		if emailLower == configuredEmail {
			marker = " " + successStyle.Render("(current)")
		}
		fmt.Printf("  %s <%s>%s\n", info.name, info.email, marker)
		fmt.Printf("    %s\n", dimStyle.Render(fmt.Sprintf("%d commits", info.count)))
	}

	if len(identityCounts) > 1 {
		fmt.Println()
		fmt.Println(dimStyle.Render("To rewrite history, use:"))
		fmt.Println(dimStyle.Render("  gitme fix:rewrite <old-email> <new-email>"))
	}
}

func cmdFixRewrite() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: gitme fix:rewrite <old-email> <new-email>\n")
		os.Exit(1)
	}

	cwd, _ := os.Getwd()

	// Check if we're in a git repo
	gitDir := filepath.Join(cwd, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: not a git repository\n")
		os.Exit(1)
	}

	oldEmail := os.Args[2]
	newEmail := os.Args[3]

	// Load config to find the new identity's name
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Find the new identity
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

	// Count commits that will be affected
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

	// Show what will happen and ask for confirmation
	fmt.Println(headerStyle.Render("Rewrite plan:"))
	fmt.Println()
	fmt.Printf("  From: %s\n", oldEmail)
	fmt.Printf("  To:   %s <%s>\n", newName, newEmail)
	fmt.Printf("  Commits to rewrite: %d\n", count)
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render("WARNING: This rewrites git history!"))
	fmt.Println(dimStyle.Render("You will need to force push after this."))
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

	err = rewriteAuthor(cwd, oldEmail, newName, newEmail)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rewriting history: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(successStyle.Render("Done!"))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println(dimStyle.Render("  git push --force-with-lease"))
}

// rewriteAuthor rewrites commits from oldEmail to newName/newEmail using git filter-branch
func rewriteAuthor(repoPath, oldEmail, newName, newEmail string) error {
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
		// Check if it's just "nothing to rewrite" which is not an error
		if strings.Contains(string(output), "nothing to rewrite") ||
			strings.Contains(string(output), "Found nothing to rewrite") {
			return nil
		}
		return fmt.Errorf("%v: %s", err, output)
	}
	return nil
}

func cmdCurrent() {
	cwd, _ := os.Getwd()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Check gitme's stored identity for this folder
	if id, ok := cfg.GetIdentityForFolder(cwd); ok {
		fmt.Printf("%s <%s>\n", id.Name, id.Email)
		fmt.Println(dimStyle.Render("(from gitme config)"))
		return
	}

	// Fall back to git config
	name, _ := exec.Command("git", "config", "user.name").Output()
	email, _ := exec.Command("git", "config", "user.email").Output()

	if len(name) > 0 || len(email) > 0 {
		fmt.Printf("%s <%s>\n", strings.TrimSpace(string(name)), strings.TrimSpace(string(email)))
		fmt.Println(dimStyle.Render("(from git config)"))
	} else {
		fmt.Println("No identity configured for this folder")
	}
}

func cmdSet() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: gitme set <email>\n")
		os.Exit(1)
	}

	email := os.Args[2]
	cwd, _ := os.Getwd()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Find identity by email
	var found *identity.Identity
	for _, id := range cfg.Identities {
		if id.Email == email || strings.Contains(id.Email, email) {
			found = &id
			break
		}
	}

	if found == nil {
		fmt.Fprintf(os.Stderr, "Identity not found: %s\n", email)
		fmt.Fprintf(os.Stderr, "Run 'gitme list' to see available identities\n")
		os.Exit(1)
	}

	if err := applyIdentity(cwd, *found); err != nil {
		fmt.Fprintf(os.Stderr, "Error applying identity: %v\n", err)
		os.Exit(1)
	}

	cfg.SetIdentityForFolder(cwd, *found)
	cfg.Save()

	fmt.Println(successStyle.Render("Switched to:"), found.Name, "<"+found.Email+">")
}

func runTUI() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Scan for identities
	identities, err := identity.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning identities: %v\n", err)
		os.Exit(1)
	}

	cfg.UpdateIdentities(identities)

	allIdentities := cfg.Identities
	if len(allIdentities) == 0 {
		fmt.Println("No git identities found.")
		fmt.Println("Add one with: gitme add \"Your Name\" \"your@email.com\"")
		os.Exit(1)
	}

	currentIdentity, hasIdentity := cfg.GetIdentityForFolder(cwd)
	var currentPtr *identity.Identity
	if hasIdentity {
		currentPtr = &currentIdentity
	}

	model := ui.New(allIdentities, currentPtr, cwd)
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running UI: %v\n", err)
		os.Exit(1)
	}

	m := finalModel.(ui.Model)

	switch m.Action() {
	case ui.ActionSelect:
		choice := m.Choice()
		if choice == nil {
			os.Exit(0)
		}
		if err := applyIdentity(cwd, *choice); err != nil {
			fmt.Fprintf(os.Stderr, "Error applying identity: %v\n", err)
			os.Exit(1)
		}
		cfg.SetIdentityForFolder(cwd, *choice)
		if err := cfg.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(successStyle.Render("Switched to:"), choice.Name, "<"+choice.Email+">")

	case ui.ActionDelete:
		target := m.DeleteTarget()
		if target == nil {
			os.Exit(0)
		}
		// Remove from config
		newIdentities := []identity.Identity{}
		for _, id := range cfg.Identities {
			if id.Email != target.Email {
				newIdentities = append(newIdentities, id)
			}
		}
		cfg.Identities = newIdentities
		if err := cfg.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(successStyle.Render("Deleted:"), target.Name, "<"+target.Email+">")

	case ui.ActionRescan:
		fmt.Println("Rescanning...")
		cmdScan()

	default:
		// User quit without action
		os.Exit(0)
	}
}

func applyIdentity(folder string, id identity.Identity) error {
	cmd := exec.Command("git", "config", "--local", "user.name", id.Name)
	cmd.Dir = folder
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set user.name: %w", err)
	}

	cmd = exec.Command("git", "config", "--local", "user.email", id.Email)
	cmd.Dir = folder
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set user.email: %w", err)
	}

	return nil
}
