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

// Auto detects and applies identity based on rules or path derivation
func Auto() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	gitDir := filepath.Join(cwd, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		// Not a git repo, silently exit (for shell hook usage)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	rules, err := config.LoadRules()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading rules: %v\n", err)
		os.Exit(1)
	}

	settings, err := config.LoadSettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading settings: %v\n", err)
		os.Exit(1)
	}

	var currentEmail string
	cmd := exec.Command("git", "config", "user.email")
	cmd.Dir = cwd
	if out, err := cmd.Output(); err == nil {
		currentEmail = strings.TrimSpace(string(out))
	}

	var expectedIdentity *identity.Identity
	var matchSource string

	// 1. Check explicit rules first
	if rule := rules.FindRuleForPath(cwd); rule != nil {
		for _, id := range cfg.Identities {
			if strings.EqualFold(id.Email, rule.Email) {
				expectedIdentity = &id
				matchSource = "rule: " + rule.Pattern
				break
			}
		}
	}

	// 2. If no rule, try to derive from path (ghq-style)
	if expectedIdentity == nil {
		expectedIdentity, matchSource = deriveIdentityFromPath(cwd, cfg.Identities)
	}

	if expectedIdentity == nil {
		return
	}

	if strings.EqualFold(currentEmail, expectedIdentity.Email) {
		return // All good
	}

	// Mismatch detected
	if settings.AutoApply {
		if err := ApplyIdentity(cwd, *expectedIdentity); err != nil {
			fmt.Fprintf(os.Stderr, "Error applying identity: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s Auto-switched to: %s <%s> (%s)\n",
			SuccessStyle.Render("✓"),
			expectedIdentity.Name, expectedIdentity.Email, matchSource)
	} else {
		fmt.Printf("%s Identity mismatch!\n", WarnStyle.Render("⚠"))
		fmt.Printf("  Current:  %s\n", currentEmail)
		fmt.Printf("  Expected: %s <%s>\n", expectedIdentity.Name, expectedIdentity.Email)
		fmt.Printf("  Source:   %s\n", DimStyle.Render(matchSource))
		fmt.Println()
		fmt.Println(DimStyle.Render("Run 'gitme set " + expectedIdentity.Email + "' to switch"))
		fmt.Println(DimStyle.Render("Or 'gitme config auto_apply on' to auto-switch"))
	}
}

func deriveIdentityFromPath(path string, identities []identity.Identity) (*identity.Identity, string) {
	for _, id := range identities {
		switch id.Platform {
		case identity.PlatformGitHub:
			if strings.Contains(path, "github.com") {
				return &id, "derived: github.com in path"
			}
		case identity.PlatformGitLab:
			if strings.Contains(path, "gitlab.com") {
				return &id, "derived: gitlab.com in path"
			}
		case identity.PlatformBitbucket:
			if strings.Contains(path, "bitbucket.org") {
				return &id, "derived: bitbucket.org in path"
			}
		}
	}
	return nil, ""
}

// Rule manages auto-switch rules
func Rule() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: gitme rule <add|list|rm> [args]\n")
		os.Exit(1)
	}

	subCmd := os.Args[2]

	rules, err := config.LoadRules()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading rules: %v\n", err)
		os.Exit(1)
	}

	switch subCmd {
	case "add":
		if len(os.Args) < 5 {
			fmt.Fprintf(os.Stderr, "Usage: gitme rule add <pattern> <email>\n")
			fmt.Fprintf(os.Stderr, "Example: gitme rule add github.com/myuser me@example.com\n")
			os.Exit(1)
		}
		pattern := os.Args[3]
		email := os.Args[4]

		cfg, _ := config.Load()
		found := false
		for _, id := range cfg.Identities {
			if strings.EqualFold(id.Email, email) {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Warning: %s is not a known identity\n", email)
		}

		rules.AddRule(pattern, email)
		if err := rules.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving rules: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s Added rule: %s → %s\n", SuccessStyle.Render("✓"), pattern, email)

	case "list", "ls":
		if len(rules.Rules) == 0 {
			fmt.Println("No rules configured.")
			fmt.Println(DimStyle.Render("Add one with: gitme rule add <pattern> <email>"))
			return
		}
		fmt.Println(HeaderStyle.Render("Auto-switch rules:"))
		fmt.Println()
		for _, r := range rules.Rules {
			fmt.Printf("  %s → %s\n", r.Pattern, r.Email)
		}

	case "rm", "remove":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: gitme rule rm <pattern>\n")
			os.Exit(1)
		}
		pattern := os.Args[3]
		if rules.RemoveRule(pattern) {
			if err := rules.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving rules: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("%s Removed rule: %s\n", SuccessStyle.Render("✓"), pattern)
		} else {
			fmt.Fprintf(os.Stderr, "Rule not found: %s\n", pattern)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown rule command: %s\n", subCmd)
		fmt.Fprintf(os.Stderr, "Usage: gitme rule <add|list|rm> [args]\n")
		os.Exit(1)
	}
}

// Config manages settings
func Config() {
	if len(os.Args) < 3 {
		settings, err := config.LoadSettings()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading settings: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(HeaderStyle.Render("Settings:"))
		fmt.Println()
		autoApplyStr := "off"
		if settings.AutoApply {
			autoApplyStr = "on"
		}
		fmt.Printf("  auto_apply: %s\n", autoApplyStr)
		return
	}

	key := os.Args[2]
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: gitme config <key> <value>\n")
		os.Exit(1)
	}
	value := os.Args[3]

	settings, err := config.LoadSettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading settings: %v\n", err)
		os.Exit(1)
	}

	switch key {
	case "auto_apply":
		switch strings.ToLower(value) {
		case "on", "true", "1", "yes":
			settings.AutoApply = true
		case "off", "false", "0", "no":
			settings.AutoApply = false
		default:
			fmt.Fprintf(os.Stderr, "Invalid value: %s (use on/off)\n", value)
			os.Exit(1)
		}
		if err := settings.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving settings: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s Set auto_apply = %s\n", SuccessStyle.Render("✓"), value)
	default:
		fmt.Fprintf(os.Stderr, "Unknown setting: %s\n", key)
		os.Exit(1)
	}
}
