package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vosamoilenko/gitme/internal/config"
	"github.com/vosamoilenko/gitme/internal/identity"
)

// MixedRepo holds info about a repo with multiple identities
type MixedRepo struct {
	Path       string
	Identities []string
}

// Repos shows all repos grouped by identity
func Repos() {
	home, _ := os.UserHomeDir()

	globalEmail, globalName := getGlobalIdentity(home)
	globalIdentity := fmt.Sprintf("%s <%s>", globalName, globalEmail)

	reposByIdentity := make(map[string][]string)
	identityOrder := []string{globalIdentity}

	for _, dir := range getWorkspaceDirs(home) {
		if _, err := os.Stat(dir); err == nil {
			collectRepos(dir, 4, globalIdentity, reposByIdentity, &identityOrder)
		}
	}

	fmt.Println(HeaderStyle.Render("All repositories:"))
	fmt.Println()

	for _, ident := range identityOrder {
		repos := reposByIdentity[ident]
		if len(repos) == 0 {
			continue
		}
		fmt.Printf("%s\n", ident)
		for _, repo := range repos {
			fmt.Printf("  %s\n", DimStyle.Render(repo))
		}
		fmt.Println()
	}
}

// Mixed shows repos with multiple identities in history
func Mixed() {
	home, _ := os.UserHomeDir()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	knownEmails := make(map[string]string)
	for _, id := range cfg.Identities {
		key := strings.ToLower(id.Email)
		knownEmails[key] = fmt.Sprintf("%s <%s>", id.Name, id.Email)
	}

	if len(knownEmails) < 2 {
		fmt.Println("You need at least 2 identities configured to check for mixed repos.")
		return
	}

	var mixed []MixedRepo
	for _, dir := range getWorkspaceDirs(home) {
		if _, err := os.Stat(dir); err == nil {
			findMixedRepos(dir, 4, knownEmails, &mixed)
		}
	}

	if len(mixed) == 0 {
		fmt.Println("No repos with mixed identities found.")
		return
	}

	fmt.Println(HeaderStyle.Render("Repos with multiple identities:"))
	fmt.Println()

	for _, repo := range mixed {
		fmt.Printf("%s\n", repo.Path)
		for _, id := range repo.Identities {
			fmt.Printf("  %s\n", DimStyle.Render(id))
		}
		fmt.Println()
	}
}

// Current shows the current identity for the folder
func Current() {
	cwd, _ := os.Getwd()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if id, ok := cfg.GetIdentityForFolder(cwd); ok {
		fmt.Printf("%s <%s>\n", id.Name, id.Email)
		fmt.Println(DimStyle.Render("(from gitme config)"))
		return
	}

	// Check git config
	cmd := exec.Command("git", "config", "user.email")
	cmd.Dir = cwd
	emailOut, err := cmd.Output()
	if err != nil {
		fmt.Println("No identity configured for this folder")
		return
	}

	cmd = exec.Command("git", "config", "user.name")
	cmd.Dir = cwd
	nameOut, _ := cmd.Output()

	email := strings.TrimSpace(string(emailOut))
	name := strings.TrimSpace(string(nameOut))

	fmt.Printf("%s <%s>\n", name, email)
	fmt.Println(DimStyle.Render("(from git config)"))
}

// Set sets the identity for the current folder
func Set() {
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

	if err := ApplyIdentity(cwd, *found); err != nil {
		fmt.Fprintf(os.Stderr, "Error applying identity: %v\n", err)
		os.Exit(1)
	}

	cfg.SetIdentityForFolder(cwd, *found)
	cfg.Save()

	fmt.Println(SuccessStyle.Render("Switched to:"), found.Name, "<"+found.Email+">")
}

// ApplyIdentity applies the identity to git config
func ApplyIdentity(cwd string, id identity.Identity) error {
	cmd := exec.Command("git", "config", "user.email", id.Email)
	cmd.Dir = cwd
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "config", "user.name", id.Name)
	cmd.Dir = cwd
	return cmd.Run()
}

// Helper functions

func getGlobalIdentity(home string) (email, name string) {
	globalConfig := filepath.Join(home, ".gitconfig")
	data, err := os.ReadFile(globalConfig)
	if err != nil {
		return "", ""
	}

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
					email = strings.TrimSpace(parts[1])
				}
			}
			if strings.HasPrefix(line, "name") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					name = strings.TrimSpace(parts[1])
				}
			}
		}
	}
	return
}

func getWorkspaceDirs(home string) []string {
	return []string{
		filepath.Join(home, "Developer"),
		filepath.Join(home, "Projects"),
		filepath.Join(home, "Code"),
		filepath.Join(home, "workspace"),
		filepath.Join(home, "src"),
		filepath.Join(home, "work"),
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
			configPath := filepath.Join(gitDir, "config")
			localEmail, localName := parseGitConfig(configPath)

			repoName := filepath.Base(subdir)
			ident := globalIdentity
			if localEmail != "" {
				ident = fmt.Sprintf("%s <%s>", localName, localEmail)
				found := false
				for _, id := range *identityOrder {
					if id == ident {
						found = true
						break
					}
				}
				if !found {
					*identityOrder = append(*identityOrder, ident)
				}
			}
			reposByIdentity[ident] = append(reposByIdentity[ident], repoName)
		}

		if maxDepth > 1 {
			collectRepos(subdir, maxDepth-1, globalIdentity, reposByIdentity, identityOrder)
		}
	}
}

func parseGitConfig(configPath string) (email, name string) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", ""
	}

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
					email = strings.TrimSpace(parts[1])
				}
			}
			if strings.HasPrefix(line, "name") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					name = strings.TrimSpace(parts[1])
				}
			}
		}
	}
	return
}

func findMixedRepos(dir string, maxDepth int, knownEmails map[string]string, mixed *[]MixedRepo) {
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
			cmd := exec.Command("git", "-C", subdir, "log", "--format=%ae")
			output, err := cmd.Output()
			if err != nil {
				continue
			}

			foundIdentities := make(map[string]bool)
			for _, line := range strings.Split(string(output), "\n") {
				email := strings.ToLower(strings.TrimSpace(line))
				if displayIdentity, ok := knownEmails[email]; ok {
					foundIdentities[displayIdentity] = true
				}
			}

			if len(foundIdentities) > 1 {
				var identities []string
				for id := range foundIdentities {
					identities = append(identities, id)
				}
				*mixed = append(*mixed, MixedRepo{
					Path:       subdir,
					Identities: identities,
				})
			}
		}

		if maxDepth > 1 {
			findMixedRepos(subdir, maxDepth-1, knownEmails, mixed)
		}
	}
}
