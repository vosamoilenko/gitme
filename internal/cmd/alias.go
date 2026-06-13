package cmd

import (
	"fmt"
	"os"

	"github.com/vosamoilenko/gitme/internal/config"
)

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
