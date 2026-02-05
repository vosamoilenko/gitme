package main

import (
	"fmt"
	"os"
	"os/exec"
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
	fmt.Println("  gitme              Interactive TUI to select identity")
	fmt.Println("  gitme list         List all known identities")
	fmt.Println("  gitme add          Add a new identity interactively")
	fmt.Println("  gitme add <n> <e>  Add identity with name and email")
	fmt.Println("  gitme remove <e>   Remove an identity by email")
	fmt.Println("  gitme current      Show current identity for this folder")
	fmt.Println("  gitme set <email>  Set identity by email (no TUI)")
	fmt.Println("  gitme help         Show this help")
	fmt.Println()
	fmt.Println("Aliases: ls=list, rm=remove, whoami=current")
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
		fmt.Printf("  %d. %s <%s>\n", i+1, id.Name, id.Email)
		if id.Source != "" {
			fmt.Printf("     %s\n", dimStyle.Render("source: "+id.Source))
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
		fmt.Fprintf(os.Stderr, "Usage: gitme remove <email>\n")
		os.Exit(1)
	}

	email := os.Args[2]

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Find and remove identity
	found := false
	newIdentities := []identity.Identity{}
	for _, id := range cfg.Identities {
		if id.Email == email || strings.Contains(id.Email, email) {
			found = true
			fmt.Println(successStyle.Render("Removed:"), id.Name, "<"+id.Email+">")
		} else {
			newIdentities = append(newIdentities, id)
		}
	}

	if !found {
		fmt.Fprintf(os.Stderr, "Identity not found: %s\n", email)
		os.Exit(1)
	}

	cfg.Identities = newIdentities
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
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
