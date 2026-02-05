package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vosamoilenko/gitme/internal/cmd"
	"github.com/vosamoilenko/gitme/internal/config"
	"github.com/vosamoilenko/gitme/internal/identity"
	"github.com/vosamoilenko/gitme/internal/ui"
)

func main() {
	if len(os.Args) < 2 {
		runTUI()
		return
	}

	switch os.Args[1] {
	// Identity management
	case "list", "ls":
		cmd.List()
	case "add":
		cmd.Add()
	case "remove", "rm":
		cmd.Remove()
	case "scan", "refresh":
		cmd.Scan()
	case "reset":
		cmd.Reset()

	// Repository commands
	case "repos":
		cmd.Repos()
	case "mixed":
		cmd.Mixed()
	case "current", "whoami":
		cmd.Current()
	case "set":
		cmd.Set()

	// Fix commands
	case "fix:scan":
		cmd.FixScan()
	case "fix:rewrite":
		cmd.FixRewrite()

	// Auto-switch commands
	case "auto":
		cmd.Auto()
	case "rule":
		cmd.Rule()
	case "config":
		cmd.Config()

	// Statistics
	case "stats":
		cmd.Stats()

	// Help
	case "help", "-h", "--help":
		printHelp()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(cmd.HeaderStyle.Render("gitme") + " - Git identity switcher")
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
	fmt.Println()
	fmt.Println(cmd.HeaderStyle.Render("Auto-switch:"))
	fmt.Println("  gitme auto                  Auto-detect and apply identity for current dir")
	fmt.Println("  gitme rule add <pat> <email> Add auto-switch rule")
	fmt.Println("  gitme rule list             List all rules")
	fmt.Println("  gitme rule rm <pattern>     Remove a rule")
	fmt.Println("  gitme config auto_apply <on|off>  Set auto-apply behavior")
	fmt.Println()
	fmt.Println(cmd.HeaderStyle.Render("Statistics:"))
	fmt.Println("  gitme stats                 Show commit stats by identity in current repo")
	fmt.Println("  gitme stats --all           Show commit stats across all repos")
	fmt.Println()
	fmt.Println("  gitme help         Show this help")
	fmt.Println()
	fmt.Println("Aliases: ls=list, rm=remove, whoami=current, refresh=scan")
	fmt.Println()
	fmt.Println("Config stored in: ~/.config/gitme/")
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

	identities, err := identity.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning identities: %v\n", err)
		os.Exit(1)
	}
	cfg.UpdateIdentities(identities)
	cfg.Save()

	if len(cfg.Identities) == 0 {
		fmt.Println("No identities found.")
		fmt.Println("Add one with: gitme add \"Your Name\" \"your@email.com\"")
		return
	}

	// Get current identity for this folder
	var currentIdentity *identity.Identity
	if id, ok := cfg.GetIdentityForFolder(cwd); ok {
		currentIdentity = &id
	}

	model := ui.New(cfg.Identities, currentIdentity, cwd)
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	m := finalModel.(ui.Model)

	switch m.Action() {
	case ui.ActionDelete:
		if target := m.DeleteTarget(); target != nil {
			// Remove the identity from the list
			var newIdentities []identity.Identity
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
			fmt.Println(cmd.SuccessStyle.Render("Deleted:"), target.Name, "<"+target.Email+">")
		}

	case ui.ActionRescan:
		cmd.Scan()

	case ui.ActionSelect:
		if selected := m.Choice(); selected != nil {
			if err := cmd.ApplyIdentity(cwd, *selected); err != nil {
				fmt.Fprintf(os.Stderr, "Error applying identity: %v\n", err)
				os.Exit(1)
			}

			cfg.SetIdentityForFolder(cwd, *selected)
			if err := cfg.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
				os.Exit(1)
			}

			fmt.Println(cmd.SuccessStyle.Render("Switched to:"), selected.Name, "<"+selected.Email+">")
		}
	}
}
