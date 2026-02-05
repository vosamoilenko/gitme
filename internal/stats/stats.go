package stats

import (
	"os/exec"
	"sort"
	"strings"
	"time"
)

// CommitInfo holds info about a single commit
type CommitInfo struct {
	Hash   string
	Author string
	Email  string
	Date   time.Time
}

// IdentityStats holds statistics for one identity
type IdentityStats struct {
	Name        string
	Email       string
	CommitCount int
	FirstCommit time.Time
	LastCommit  time.Time
	ByWeekday   map[time.Weekday]int
	ByHour      map[int]int
}

// RepoStats holds all statistics for a repository
type RepoStats struct {
	RepoPath   string
	TotalCount int
	ByIdentity map[string]*IdentityStats // keyed by email
}

// CollectRepoStats gathers commit statistics for a repository
func CollectRepoStats(repoPath string, knownEmails map[string]bool) (*RepoStats, error) {
	// Get all commits with author info and date
	cmd := exec.Command("git", "-C", repoPath, "log", "--format=%H|%an|%ae|%aI")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	stats := &RepoStats{
		RepoPath:   repoPath,
		ByIdentity: make(map[string]*IdentityStats),
	}

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			continue
		}

		// hash := parts[0]
		name := parts[1]
		email := strings.ToLower(parts[2])
		dateStr := parts[3]

		// Only count known identities if filter provided
		if knownEmails != nil && !knownEmails[email] {
			continue
		}

		date, _ := time.Parse(time.RFC3339, dateStr)

		// Get or create identity stats
		idStats, ok := stats.ByIdentity[email]
		if !ok {
			idStats = &IdentityStats{
				Name:        name,
				Email:       parts[2], // preserve original case
				ByWeekday:   make(map[time.Weekday]int),
				ByHour:      make(map[int]int),
				FirstCommit: date,
				LastCommit:  date,
			}
			stats.ByIdentity[email] = idStats
		}

		idStats.CommitCount++
		stats.TotalCount++

		// Update date range
		if date.Before(idStats.FirstCommit) {
			idStats.FirstCommit = date
		}
		if date.After(idStats.LastCommit) {
			idStats.LastCommit = date
		}

		// Track by weekday and hour
		idStats.ByWeekday[date.Weekday()]++
		idStats.ByHour[date.Hour()]++
	}

	return stats, nil
}

// SortedIdentities returns identity stats sorted by commit count (descending)
func (r *RepoStats) SortedIdentities() []*IdentityStats {
	var result []*IdentityStats
	for _, s := range r.ByIdentity {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CommitCount > result[j].CommitCount
	})
	return result
}

// AggregatedWeekdayStats returns combined weekday stats for all identities
func (r *RepoStats) AggregatedWeekdayStats() map[time.Weekday]int {
	result := make(map[time.Weekday]int)
	for _, idStats := range r.ByIdentity {
		for day, count := range idStats.ByWeekday {
			result[day] += count
		}
	}
	return result
}

// MaxWeekdayCount returns the maximum count for any weekday (for scaling bars)
func MaxWeekdayCount(weekdayStats map[time.Weekday]int) int {
	max := 0
	for _, count := range weekdayStats {
		if count > max {
			max = count
		}
	}
	return max
}
