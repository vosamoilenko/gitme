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
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)

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
	case "repos":
		cmdRepos()
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
	fmt.Println("  gitme add          Add a new identity interactively")
	fmt.Println("  gitme add <n> <e>  Add identity with name and email")
	fmt.Println("  gitme remove <#|e> Remove identity by number or email")
	fmt.Println("  gitme scan         Rescan machine for git identities")
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

	// Find and remove identity
	found := false
	newIdentities := []identity.Identity{}
	for i, id := range cfg.Identities {
		shouldRemove := false
		if removeIndex >= 0 {
			// Remove by index
			shouldRemove = (i == removeIndex)
		} else {
			// Remove by email match
			shouldRemove = (id.Email == arg || strings.Contains(id.Email, arg))
		}

		if shouldRemove {
			found = true
			fmt.Println(successStyle.Render("Removed:"), id.Name, "<"+id.Email+">")
			fmt.Println(dimStyle.Render("  was at: " + id.Source))
		} else {
			newIdentities = append(newIdentities, id)
		}
	}

	if !found {
		fmt.Fprintf(os.Stderr, "Identity not found: %s\n", arg)
		fmt.Fprintf(os.Stderr, "Run 'gitme list' to see all identities\n")
		os.Exit(1)
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

	fmt.Println(headerStyle.Render("All repositories:"))
	fmt.Printf("\nGlobal identity: %s <%s>\n\n", globalName, globalEmail)

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
			scanAndShowRepos(dir, 4, globalEmail)
		}
	}
}

func scanAndShowRepos(dir string, maxDepth int, globalEmail string) {
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
			if localEmail != "" {
				// Has local config
				fmt.Printf("  %s\n", repoName)
				fmt.Printf("     %s <%s> %s\n", localName, localEmail, dimStyle.Render("(local)"))
			} else {
				// Uses global
				fmt.Printf("  %s %s\n", dimStyle.Render(repoName), dimStyle.Render("(global)"))
			}
		}

		if maxDepth > 1 {
			scanAndShowRepos(subdir, maxDepth-1, globalEmail)
		}
	}
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
