package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vosamoilenko/gitme/internal/config"
	"github.com/vosamoilenko/gitme/internal/stats"
)

// Stats shows commit statistics by identity
func Stats() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	// Check if --all flag
	showAll := len(os.Args) >= 3 && (os.Args[2] == "--all" || os.Args[2] == "-a")

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Build set of known emails
	knownEmails := make(map[string]bool)
	for _, id := range cfg.Identities {
		knownEmails[strings.ToLower(id.Email)] = true
	}

	if showAll {
		statsAll(knownEmails)
	} else {
		statsSingle(cwd, knownEmails)
	}
}

func statsSingle(cwd string, knownEmails map[string]bool) {
	// Check if we're in a git repo
	gitDir := filepath.Join(cwd, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: not a git repository\n")
		os.Exit(1)
	}

	repoStats, err := stats.CollectRepoStats(cwd, knownEmails)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error collecting stats: %v\n", err)
		os.Exit(1)
	}

	if repoStats.TotalCount == 0 {
		fmt.Println("No commits found from your known identities in this repo.")
		return
	}

	printRepoStats(repoStats)
}

func statsAll(knownEmails map[string]bool) {
	home, _ := os.UserHomeDir()

	workspaceDirs := []string{
		filepath.Join(home, "Developer"),
		filepath.Join(home, "Projects"),
		filepath.Join(home, "Code"),
		filepath.Join(home, "workspace"),
		filepath.Join(home, "src"),
		filepath.Join(home, "work"),
	}

	// Aggregate stats across all repos
	aggregated := &stats.RepoStats{
		ByIdentity: make(map[string]*stats.IdentityStats),
	}

	repoCount := 0
	for _, dir := range workspaceDirs {
		if _, err := os.Stat(dir); err == nil {
			collectAllRepos(dir, 4, knownEmails, aggregated, &repoCount)
		}
	}

	if aggregated.TotalCount == 0 {
		fmt.Println("No commits found from your known identities.")
		return
	}

	fmt.Printf("%s (across %d repositories)\n\n", HeaderStyle.Render("Your commit statistics"), repoCount)
	printIdentityStats(aggregated)
	printWeekdayChart(aggregated)
}

func collectAllRepos(dir string, maxDepth int, knownEmails map[string]bool, aggregated *stats.RepoStats, repoCount *int) {
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
			// Found a repo
			repoStats, err := stats.CollectRepoStats(subdir, knownEmails)
			if err == nil && repoStats.TotalCount > 0 {
				*repoCount++
				aggregated.TotalCount += repoStats.TotalCount

				// Merge identity stats
				for email, idStats := range repoStats.ByIdentity {
					if existing, ok := aggregated.ByIdentity[email]; ok {
						existing.CommitCount += idStats.CommitCount
						if idStats.FirstCommit.Before(existing.FirstCommit) {
							existing.FirstCommit = idStats.FirstCommit
						}
						if idStats.LastCommit.After(existing.LastCommit) {
							existing.LastCommit = idStats.LastCommit
						}
						for day, count := range idStats.ByWeekday {
							existing.ByWeekday[day] += count
						}
						for hour, count := range idStats.ByHour {
							existing.ByHour[hour] += count
						}
					} else {
						// Copy the stats
						aggregated.ByIdentity[email] = &stats.IdentityStats{
							Name:        idStats.Name,
							Email:       idStats.Email,
							CommitCount: idStats.CommitCount,
							FirstCommit: idStats.FirstCommit,
							LastCommit:  idStats.LastCommit,
							ByWeekday:   make(map[time.Weekday]int),
							ByHour:      make(map[int]int),
						}
						for day, count := range idStats.ByWeekday {
							aggregated.ByIdentity[email].ByWeekday[day] = count
						}
						for hour, count := range idStats.ByHour {
							aggregated.ByIdentity[email].ByHour[hour] = count
						}
					}
				}
			}
		}

		if maxDepth > 1 {
			collectAllRepos(subdir, maxDepth-1, knownEmails, aggregated, repoCount)
		}
	}
}

func printRepoStats(repoStats *stats.RepoStats) {
	fmt.Println(HeaderStyle.Render("Commits by your identities:"))
	fmt.Println()
	printIdentityStats(repoStats)
	printWeekdayChart(repoStats)
}

func printIdentityStats(repoStats *stats.RepoStats) {
	sorted := repoStats.SortedIdentities()

	for _, idStats := range sorted {
		percentage := float64(idStats.CommitCount) / float64(repoStats.TotalCount) * 100
		fmt.Printf("  %s <%s>\n", idStats.Name, idStats.Email)
		fmt.Printf("    %s\n", DimStyle.Render(fmt.Sprintf(
			"%d commits (%.0f%%) | %s → %s",
			idStats.CommitCount,
			percentage,
			idStats.FirstCommit.Format("2006-01-02"),
			idStats.LastCommit.Format("2006-01-02"),
		)))
		fmt.Println()
	}
}

func printWeekdayChart(repoStats *stats.RepoStats) {
	weekdayStats := repoStats.AggregatedWeekdayStats()
	maxCount := stats.MaxWeekdayCount(weekdayStats)

	if maxCount == 0 {
		return
	}

	fmt.Println(HeaderStyle.Render("Activity by weekday:"))
	fmt.Println()

	days := []time.Weekday{
		time.Monday, time.Tuesday, time.Wednesday,
		time.Thursday, time.Friday, time.Saturday, time.Sunday,
	}
	dayNames := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

	maxBarWidth := 30
	for i, day := range days {
		count := weekdayStats[day]
		barLen := 0
		if maxCount > 0 {
			barLen = count * maxBarWidth / maxCount
		}
		bar := strings.Repeat("█", barLen)
		fmt.Printf("  %s %s %s\n", dayNames[i], DimStyle.Render(bar), DimStyle.Render(fmt.Sprintf("%d", count)))
	}
	fmt.Println()
}
