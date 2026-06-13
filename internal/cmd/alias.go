package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/vosamoilenko/gitme/internal/config"
	"github.com/vosamoilenko/gitme/internal/identity"
)

var sshRemoteRe = regexp.MustCompile(`^git@([^:]+):(.+)$`)

func switchSSHRemotes(cwd, alias string) error {
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return err
	}

	seen := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		remoteName := fields[0]
		url := fields[1]

		if seen[remoteName] {
			continue
		}
		seen[remoteName] = true

		m := sshRemoteRe.FindStringSubmatch(url)
		if m == nil {
			continue
		}

		host := m[1]
		path := m[2]

		var platform string
		switch {
		case strings.Contains(host, "github"):
			platform = "github"
		case strings.Contains(host, "gitlab"):
			platform = "gitlab"
		default:
			continue
		}

		newHost := alias + "-" + platform
		if host == newHost {
			continue
		}

		newURL := "git@" + newHost + ":" + path
		setCmd := exec.Command("git", "remote", "set-url", remoteName, newURL)
		setCmd.Dir = cwd
		if err := setCmd.Run(); err != nil {
			return fmt.Errorf("failed to update remote %s: %w", remoteName, err)
		}
		fmt.Printf("  Remote %s → %s\n", remoteName, newURL)
	}
	return nil
}

// Use resolves an alias and switches identity + SSH remote
func Use() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: gitme use <alias>\n")
		os.Exit(1)
	}

	name := os.Args[2]

	aliases, err := config.LoadAliases()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading aliases: %v\n", err)
		os.Exit(1)
	}

	email := aliases.ResolveAlias(name)
	if email == name {
		fmt.Fprintf(os.Stderr, "Alias not found: %s\n", name)
		fmt.Fprintf(os.Stderr, "Run 'gitme alias list' to see available aliases\n")
		os.Exit(1)
	}

	cwd, _ := os.Getwd()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	var found *identity.Identity
	for _, id := range cfg.Identities {
		if id.Email == email {
			found = &id
			break
		}
	}

	if found == nil {
		fmt.Fprintf(os.Stderr, "Identity not found for email: %s\n", email)
		os.Exit(1)
	}

	if err := ApplyIdentity(cwd, *found); err != nil {
		fmt.Fprintf(os.Stderr, "Error applying identity: %v\n", err)
		os.Exit(1)
	}

	if err := switchSSHRemotes(cwd, name); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not switch SSH remotes: %v\n", err)
	}

	cfg.SetIdentityForFolder(cwd, *found)
	cfg.Save()

	fmt.Println(SuccessStyle.Render("Switched to:"), found.Name, "<"+found.Email+">")
}

// Alias handles the alias subcommand
func Alias() {
	if len(os.Args) < 3 {
		aliasUsage()
		os.Exit(1)
	}

	switch os.Args[2] {
	case "add", "set":
		aliasAdd()
	case "list", "ls":
		aliasList()
	case "remove", "rm":
		aliasRemove()
	default:
		fmt.Fprintf(os.Stderr, "Unknown alias command: %s\n", os.Args[2])
		aliasUsage()
		os.Exit(1)
	}
}

func aliasUsage() {
	fmt.Println("Usage:")
	fmt.Println("  gitme alias add <name> <email>  Add an alias for quick switching")
	fmt.Println("  gitme alias list                List all aliases")
	fmt.Println("  gitme alias rm <name>           Remove an alias")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  gitme alias add work volodymyr@company.com")
	fmt.Println("  gitme alias add personal me@gmail.com")
	fmt.Println("  gitme use work    # Uses the alias to switch identity")
}

func aliasAdd() {
	if len(os.Args) < 5 {
		fmt.Fprintf(os.Stderr, "Usage: gitme alias add <name> <email>\n")
		os.Exit(1)
	}

	name := os.Args[3]
	email := os.Args[4]

	aliases, err := config.LoadAliases()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading aliases: %v\n", err)
		os.Exit(1)
	}

	aliases.SetAlias(name, email)

	if err := aliases.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving aliases: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(SuccessStyle.Render("Added alias:"), name, "→", email)
}

func aliasList() {
	aliases, err := config.LoadAliases()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading aliases: %v\n", err)
		os.Exit(1)
	}

	if len(aliases.Aliases) == 0 {
		fmt.Println("No aliases configured.")
		fmt.Println("Add one with: gitme alias add <name> <email>")
		return
	}

	fmt.Println(HeaderStyle.Render("Aliases:"))
	fmt.Println()
	for name, email := range aliases.Aliases {
		fmt.Printf("  %s → %s\n", name, email)
	}
}

func aliasRemove() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: gitme alias rm <name>\n")
		os.Exit(1)
	}

	name := os.Args[3]

	aliases, err := config.LoadAliases()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading aliases: %v\n", err)
		os.Exit(1)
	}

	if !aliases.RemoveAlias(name) {
		fmt.Fprintf(os.Stderr, "Alias not found: %s\n", name)
		os.Exit(1)
	}

	if err := aliases.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving aliases: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(SuccessStyle.Render("Removed alias:"), name)
}
