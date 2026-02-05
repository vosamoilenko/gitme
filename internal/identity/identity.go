package identity

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Platform represents the git hosting platform
type Platform string

const (
	PlatformUnknown   Platform = ""
	PlatformGitHub    Platform = "github"
	PlatformGitLab    Platform = "gitlab"
	PlatformBitbucket Platform = "bitbucket"
)

// Identity represents a git identity
type Identity struct {
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Source   string   `json:"source"`   // where this identity was found (full path)
	Platform Platform `json:"platform"` // github, gitlab, etc.
}

// String returns a display string for the identity
func (i Identity) String() string {
	return i.Name + " <" + i.Email + ">"
}

// DetectPlatform detects the platform from email
func DetectPlatform(email string) Platform {
	email = strings.ToLower(email)

	if strings.Contains(email, "github") || strings.HasSuffix(email, "@users.noreply.github.com") {
		return PlatformGitHub
	}
	if strings.Contains(email, "gitlab") {
		return PlatformGitLab
	}
	if strings.Contains(email, "bitbucket") {
		return PlatformBitbucket
	}

	return PlatformUnknown
}

// Scan finds all git identities on the machine
func Scan() ([]Identity, error) {
	var identities []Identity
	seen := make(map[string]bool)

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Get global email first (needed to detect platform for repos that inherit it)
	globalEmail := ""
	globalConfig := filepath.Join(home, ".gitconfig")
	if id, err := parseGitConfig(globalConfig, globalConfig, ""); err == nil && id != nil {
		globalEmail = id.Email
	}

	// Scan all repos to build email -> platform mapping
	emailPlatforms := make(map[string]Platform)
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
			scanRepoPlatforms(dir, 2, emailPlatforms, globalEmail)
		}
	}

	// Parse ~/.gitconfig (re-parse to get full identity with platform)
	if id, err := parseGitConfig(globalConfig, globalConfig, ""); err == nil && id != nil {
		// Try to get platform from repos using this email
		if id.Platform == PlatformUnknown {
			if p, ok := emailPlatforms[id.Email]; ok {
				id.Platform = p
			}
		}
		if !seen[id.Email] {
			identities = append(identities, *id)
			seen[id.Email] = true
		}
	}

	// Parse ~/.config/git/config
	xdgConfig := filepath.Join(home, ".config", "git", "config")
	if id, err := parseGitConfig(xdgConfig, xdgConfig, ""); err == nil && id != nil {
		if id.Platform == PlatformUnknown {
			if p, ok := emailPlatforms[id.Email]; ok {
				id.Platform = p
			}
		}
		if !seen[id.Email] {
			identities = append(identities, *id)
			seen[id.Email] = true
		}
	}

	// Scan for .gitconfig includes
	includeIdentities, _ := scanIncludes(globalConfig)
	for _, id := range includeIdentities {
		if id.Platform == PlatformUnknown {
			if p, ok := emailPlatforms[id.Email]; ok {
				id.Platform = p
			}
		}
		if !seen[id.Email] {
			identities = append(identities, id)
			seen[id.Email] = true
		}
	}

	// Scan repos for local identities
	for _, dir := range workspaceDirs {
		if _, err := os.Stat(dir); err == nil {
			found, _ := scanDirectory(dir, 2, seen)
			identities = append(identities, found...)
		}
	}

	return identities, nil
}

// scanRepoPlatforms scans repos to build email -> platform mapping
// globalEmail is used when a repo has no local email configured (inherits global)
func scanRepoPlatforms(dir string, maxDepth int, emailPlatforms map[string]Platform, globalEmail string) {
	if maxDepth <= 0 {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		subdir := filepath.Join(dir, entry.Name())
		gitDir := filepath.Join(subdir, ".git")

		if _, err := os.Stat(gitDir); err == nil {
			// Found a git repo - detect its platform
			platform := detectPlatformFromRemotes(gitDir)
			if platform != PlatformUnknown {
				// Get the email configured for this repo (local or inherited)
				email := getRepoEmail(gitDir)
				if email == "" {
					// No local email - repo uses global email
					email = globalEmail
				}
				if email != "" {
					// Only set if not already set (first match wins)
					if _, exists := emailPlatforms[email]; !exists {
						emailPlatforms[email] = platform
					}
				}
			}
		}

		if maxDepth > 1 {
			scanRepoPlatforms(subdir, maxDepth-1, emailPlatforms, globalEmail)
		}
	}
}

// getRepoEmail gets the user.email for a repo
func getRepoEmail(gitDir string) string {
	configPath := filepath.Join(gitDir, "config")
	file, err := os.Open(configPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	inUserSection := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[user]") {
			inUserSection = true
			continue
		}
		if strings.HasPrefix(line, "[") && inUserSection {
			break
		}
		if inUserSection && strings.HasPrefix(line, "email") {
			return extractValue(line)
		}
	}
	return ""
}

func parseGitConfig(path, source, repoPath string) (*Identity, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var name, email string
	inUserSection := false
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "[user]") {
			inUserSection = true
			continue
		}
		if strings.HasPrefix(line, "[") && inUserSection {
			break
		}

		if inUserSection {
			if strings.HasPrefix(line, "name") {
				name = extractValue(line)
			} else if strings.HasPrefix(line, "email") {
				email = extractValue(line)
			}
		}
	}

	if name != "" && email != "" {
		platform := DetectPlatform(email)

		// If platform not detected from email, try to detect from remotes
		if platform == PlatformUnknown && repoPath != "" {
			platform = detectPlatformFromRemotes(repoPath)
		}

		return &Identity{
			Name:     name,
			Email:    email,
			Source:   source,
			Platform: platform,
		}, nil
	}
	return nil, nil
}

func extractValue(line string) string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func scanIncludes(gitconfigPath string) ([]Identity, error) {
	var identities []Identity

	file, err := os.Open(gitconfigPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	home, _ := os.UserHomeDir()
	includeRegex := regexp.MustCompile(`^\s*path\s*=\s*(.+)$`)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if matches := includeRegex.FindStringSubmatch(line); len(matches) == 2 {
			includePath := strings.TrimSpace(matches[1])
			if strings.HasPrefix(includePath, "~") {
				includePath = filepath.Join(home, includePath[1:])
			}
			if id, err := parseGitConfig(includePath, includePath, ""); err == nil && id != nil {
				identities = append(identities, *id)
			}
		}
	}

	return identities, nil
}

func scanDirectory(dir string, maxDepth int, seen map[string]bool) ([]Identity, error) {
	var identities []Identity

	if maxDepth <= 0 {
		return identities, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		subdir := filepath.Join(dir, entry.Name())
		gitDir := filepath.Join(subdir, ".git")
		gitConfig := filepath.Join(gitDir, "config")

		if id, err := parseGitConfig(gitConfig, gitConfig, gitDir); err == nil && id != nil {
			if !seen[id.Email] {
				identities = append(identities, *id)
				seen[id.Email] = true
			}
		}

		if maxDepth > 1 {
			found, _ := scanDirectory(subdir, maxDepth-1, seen)
			identities = append(identities, found...)
		}
	}

	return identities, nil
}

// detectPlatformFromRemotes checks git remotes to detect the platform
func detectPlatformFromRemotes(gitDir string) Platform {
	configPath := filepath.Join(gitDir, "config")
	file, err := os.Open(configPath)
	if err != nil {
		return PlatformUnknown
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.ToLower(scanner.Text())
		if strings.Contains(line, "url") {
			if strings.Contains(line, "github.com") {
				return PlatformGitHub
			}
			if strings.Contains(line, "gitlab.com") || strings.Contains(line, "gitlab") {
				return PlatformGitLab
			}
			if strings.Contains(line, "bitbucket") {
				return PlatformBitbucket
			}
		}
	}

	return PlatformUnknown
}
