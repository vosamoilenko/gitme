package identity

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Identity represents a git identity
type Identity struct {
	Name   string
	Email  string
	Source string // where this identity was found
}

// String returns a display string for the identity
func (i Identity) String() string {
	return i.Name + " <" + i.Email + ">"
}

// Scan finds all git identities on the machine
func Scan() ([]Identity, error) {
	var identities []Identity
	seen := make(map[string]bool)

	// Check global gitconfig
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Parse ~/.gitconfig
	globalConfig := filepath.Join(home, ".gitconfig")
	if id, err := parseGitConfig(globalConfig, "global"); err == nil && id != nil {
		key := id.Email
		if !seen[key] {
			identities = append(identities, *id)
			seen[key] = true
		}
	}

	// Parse ~/.config/git/config
	xdgConfig := filepath.Join(home, ".config", "git", "config")
	if id, err := parseGitConfig(xdgConfig, "xdg"); err == nil && id != nil {
		key := id.Email
		if !seen[key] {
			identities = append(identities, *id)
			seen[key] = true
		}
	}

	// Scan for .gitconfig includes and conditional includes
	includeIdentities, _ := scanIncludes(globalConfig)
	for _, id := range includeIdentities {
		key := id.Email
		if !seen[key] {
			identities = append(identities, id)
			seen[key] = true
		}
	}

	// Scan common workspace directories for local configs
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
			found, _ := scanDirectory(dir, 2, seen)
			identities = append(identities, found...)
		}
	}

	return identities, nil
}

func parseGitConfig(path, source string) (*Identity, error) {
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
		return &Identity{Name: name, Email: email, Source: source}, nil
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
			// Expand ~ to home directory
			if strings.HasPrefix(includePath, "~") {
				includePath = filepath.Join(home, includePath[1:])
			}
			if id, err := parseGitConfig(includePath, "include: "+filepath.Base(includePath)); err == nil && id != nil {
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

		// Check for .git/config
		gitConfig := filepath.Join(subdir, ".git", "config")
		if id, err := parseGitConfig(gitConfig, "repo: "+entry.Name()); err == nil && id != nil {
			key := id.Email
			if !seen[key] {
				identities = append(identities, *id)
				seen[key] = true
			}
		}

		// Recurse
		if maxDepth > 1 {
			found, _ := scanDirectory(subdir, maxDepth-1, seen)
			identities = append(identities, found...)
		}
	}

	return identities, nil
}
