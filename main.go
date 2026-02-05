package main

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vosamoilenko/gitme/internal/config"
	"github.com/vosamoilenko/gitme/internal/identity"
	"github.com/vosamoilenko/gitme/internal/ui"
)

func main() {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	// Load config
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

	// Merge with stored identities
	cfg.UpdateIdentities(identities)

	// Use all known identities
	allIdentities := cfg.Identities
	if len(allIdentities) == 0 {
		fmt.Println("No git identities found on this machine.")
		fmt.Println("Configure at least one identity with:")
		fmt.Println("  git config --global user.name \"Your Name\"")
		fmt.Println("  git config --global user.email \"your@email.com\"")
		os.Exit(1)
	}

	// Check if there's already an identity set for this folder
	currentIdentity, hasIdentity := cfg.GetIdentityForFolder(cwd)
	var currentPtr *identity.Identity
	if hasIdentity {
		currentPtr = &currentIdentity
	}

	// Create and run the UI
	model := ui.New(allIdentities, currentPtr, cwd)
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running UI: %v\n", err)
		os.Exit(1)
	}

	// Get the choice
	m := finalModel.(ui.Model)
	choice := m.Choice()
	if choice == nil {
		// User quit without selecting
		os.Exit(0)
	}

	// Apply the identity to the current repo
	if err := applyIdentity(cwd, *choice); err != nil {
		fmt.Fprintf(os.Stderr, "Error applying identity: %v\n", err)
		os.Exit(1)
	}

	// Save to config
	cfg.SetIdentityForFolder(cwd, *choice)
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Switched to: %s <%s>\n", choice.Name, choice.Email)
}

func applyIdentity(folder string, id identity.Identity) error {
	// Set local git config
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
