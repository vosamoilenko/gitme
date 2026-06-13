package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/atotto/clipboard"
)

type worktreeConfig struct {
	Projects map[string]string `json:"projects"`
}

func worktreeConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gitme", "worktrees.json")
}

func loadWorktreeConfig() *worktreeConfig {
	cfg := &worktreeConfig{Projects: make(map[string]string)}
	data, err := os.ReadFile(worktreeConfigPath())
	if err != nil {
		return cfg
	}
	json.Unmarshal(data, cfg)
	if cfg.Projects == nil {
		cfg.Projects = make(map[string]string)
	}
	return cfg
}

func (c *worktreeConfig) save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(worktreeConfigPath(), data, 0644)
}

func getWorktreesPath(gitRoot string) string {
	cfg := loadWorktreeConfig()
	if p, ok := cfg.Projects[gitRoot]; ok {
		return p
	}
	parentDir := filepath.Dir(gitRoot)
	dirName := filepath.Base(gitRoot)
	return filepath.Join(parentDir, dirName+"-worktrees")
}

func requireGitRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot get working directory")
		os.Exit(1)
	}
	root, err := RepoRoot(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Not inside a git repository")
		os.Exit(1)
	}
	return root
}

func branchExists(branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

func treePath(args []string) {
	gitRoot := requireGitRoot()

	if len(args) < 1 {
		current := getWorktreesPath(gitRoot)
		fmt.Println(current)
		return
	}

	resolved, _ := filepath.Abs(args[0])
	cfg := loadWorktreeConfig()
	cfg.Projects[gitRoot] = resolved
	if err := cfg.save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(SuccessStyle.Render("Worktrees path set to:"), resolved)
}

func wtCb(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gitme tree cb <branch-name>")
		os.Exit(1)
	}
	branchName := args[0]

	gitRoot := requireGitRoot()
	worktreesDir := getWorktreesPath(gitRoot)

	os.MkdirAll(worktreesDir, 0755)

	wtPath := filepath.Join(worktreesDir, branchName)
	if _, err := os.Stat(wtPath); err == nil {
		fmt.Fprintf(os.Stderr, "Path already exists: %s\n", wtPath)
		os.Exit(1)
	}

	var cmd *exec.Cmd
	if branchExists(branchName) {
		cmd = exec.Command("git", "worktree", "add", wtPath, branchName)
	} else {
		cmd = exec.Command("git", "worktree", "add", wtPath, "-b", branchName)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}

	clipboard.WriteAll(wtPath)
	fmt.Println()
	fmt.Println(SuccessStyle.Render("Worktree created:"), wtPath)
	fmt.Println(DimStyle.Render("(path copied to clipboard)"))
}

func wtLs() {
	_ = requireGitRoot()
	cmd := exec.Command("git", "worktree", "list")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func wtRm(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gitme tree rm <branch-name|path>")
		os.Exit(1)
	}
	target := args[0]
	gitRoot := requireGitRoot()

	if !filepath.IsAbs(target) {
		if _, err := os.Stat(target); err != nil {
			candidate := filepath.Join(getWorktreesPath(gitRoot), target)
			if _, err := os.Stat(candidate); err == nil {
				target = candidate
			}
		}
	}

	resolved, _ := filepath.Abs(target)

	cmd := exec.Command("git", "worktree", "remove", resolved)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
	fmt.Println(SuccessStyle.Render("Removed worktree:"), resolved)
}

// Tree dispatches worktree subcommands: gitme tree <subcmd> [args...]
func Tree() {
	args := os.Args[2:]
	subcmd := ""
	if len(args) > 0 {
		subcmd = args[0]
		args = args[1:]
	}

	switch subcmd {
	case "path":
		treePath(args)
	case "cb":
		wtCb(args)
	case "ls":
		wtLs()
	case "rm":
		wtRm(args)
	case "", "help", "-h", "--help":
		treeHelp()
	default:
		fmt.Fprintf(os.Stderr, "Unknown tree command: %s\n", subcmd)
		treeHelp()
		os.Exit(1)
	}
}

func treeHelp() {
	fmt.Println(HeaderStyle.Render("gitme tree") + " - worktree manager")
	fmt.Println()
	fmt.Println("  gitme tree path [<path>]   Show or set worktrees path for this project")
	fmt.Println("  gitme tree cb <branch>     Create a worktree branch (copies path to clipboard)")
	fmt.Println("  gitme tree ls              List all worktrees")
	fmt.Println("  gitme tree rm <name|path>  Remove a worktree")
}
