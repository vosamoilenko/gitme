package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/vosamoilenko/gitme/internal/config"
	"github.com/vosamoilenko/gitme/internal/identity"
)

// List shows all known identities
func List() {
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

	fmt.Println(HeaderStyle.Render("Identities:"))
	fmt.Println()
	for i, id := range cfg.Identities {
		platformIcon := getPlatformIcon(id.Platform)
		fmt.Printf("  %d. %s%s <%s>\n", i+1, platformIcon, id.Name, id.Email)
		if len(id.Sources) > 0 {
			for _, src := range id.Sources {
				fmt.Printf("     %s\n", DimStyle.Render(src))
			}
		} else if id.Source != "" {
			fmt.Printf("     %s\n", DimStyle.Render(id.Source))
		}
	}

	if len(cfg.FolderIdentities) > 0 {
		fmt.Println()
		fmt.Println(HeaderStyle.Render("Folder mappings:"))
		fmt.Println()
		for folder, id := range cfg.FolderIdentities {
			fmt.Printf("  %s\n", folder)
			fmt.Printf("     %s\n", DimStyle.Render(id.Email))
		}
	}
}

// Add adds a new identity
func Add() {
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

	fmt.Println(SuccessStyle.Render("Added:"), name, "<"+email+">")
}

// Remove removes an identity
func Remove() {
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

	var removeIndex int = -1
	if idx, err := fmt.Sscanf(arg, "%d", &removeIndex); err == nil && idx == 1 {
		removeIndex--
		if removeIndex < 0 || removeIndex >= len(cfg.Identities) {
			fmt.Fprintf(os.Stderr, "Invalid index: %s (valid: 1-%d)\n", arg, len(cfg.Identities))
			os.Exit(1)
		}
	}

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

		removeIndex = matches[0]
	}

	removed := cfg.Identities[removeIndex]
	cfg.Identities = append(cfg.Identities[:removeIndex], cfg.Identities[removeIndex+1:]...)

	fmt.Println(SuccessStyle.Render("Removed:"), removed.Name, "<"+removed.Email+">")
	if removed.Source != "" {
		fmt.Println(DimStyle.Render("  was at: " + removed.Source))
	}

	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}
}

// Scan rescans for git identities
func Scan() {
	fmt.Println("Scanning for git identities...")

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

	// Keep manual identities
	manualIdentities := []identity.Identity{}
	for _, id := range cfg.Identities {
		if id.Source == "manual" {
			manualIdentities = append(manualIdentities, id)
		}
	}

	cfg.Identities = scanned
	for _, id := range manualIdentities {
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

	fmt.Println(SuccessStyle.Render(fmt.Sprintf("Found %d identities", len(cfg.Identities))))
	fmt.Println()
	printIdentities(cfg.Identities)
}

// Reset deletes config and rescans
func Reset() {
	fmt.Println("Deleting config and rescanning...")

	if err := config.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting config: %v\n", err)
		os.Exit(1)
	}

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

	fmt.Println(SuccessStyle.Render(fmt.Sprintf("Found %d identities", len(cfg.Identities))))
	fmt.Println()
	for i, id := range cfg.Identities {
		platformIcon := getPlatformIcon(id.Platform)
		fmt.Printf("  %d. %s%s <%s>\n", i+1, platformIcon, id.Name, id.Email)
	}
}

// Helper functions

func getPlatformIcon(platform identity.Platform) string {
	switch platform {
	case identity.PlatformGitHub:
		return "[GitHub] "
	case identity.PlatformGitLab:
		return "[GitLab] "
	case identity.PlatformBitbucket:
		return "[Bitbucket] "
	default:
		return ""
	}
}

func printIdentities(identities []identity.Identity) {
	for i, id := range identities {
		platformIcon := getPlatformIcon(id.Platform)
		fmt.Printf("  %d. %s%s <%s>\n", i+1, platformIcon, id.Name, id.Email)
		if len(id.Sources) > 0 {
			for _, src := range id.Sources {
				fmt.Printf("     %s\n", DimStyle.Render(src))
			}
		} else if id.Source != "" {
			fmt.Printf("     %s\n", DimStyle.Render(id.Source))
		}
	}
}
