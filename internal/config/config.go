package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/vosamoilenko/gitme/internal/identity"
)

var configDir string

func init() {
	home, _ := os.UserHomeDir()
	configDir = filepath.Join(home, ".config", "gitme")
	os.MkdirAll(configDir, 0755)
}

// ============ Identities Config ============

// Config holds identities and folder mappings
type Config struct {
	FolderIdentities map[string]identity.Identity `json:"folder_identities"`
	Identities       []identity.Identity          `json:"identities"`
}

func identitiesPath() string {
	return filepath.Join(configDir, "identities.json")
}

// Load reads the identities config from disk
func Load() (*Config, error) {
	cfg := &Config{
		FolderIdentities: make(map[string]identity.Identity),
		Identities:       []identity.Identity{},
	}

	data, err := os.ReadFile(identitiesPath())
	if err != nil {
		if os.IsNotExist(err) {
			// Try legacy config.json
			legacyPath := filepath.Join(configDir, "config.json")
			data, err = os.ReadFile(legacyPath)
			if err != nil {
				return cfg, nil
			}
			// Migrate from legacy
			if err := json.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
			// Save to new location and delete legacy
			cfg.Save()
			os.Remove(legacyPath)
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if cfg.FolderIdentities == nil {
		cfg.FolderIdentities = make(map[string]identity.Identity)
	}

	return cfg, nil
}

// Save writes the identities config to disk
func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(identitiesPath(), data, 0644)
}

// Delete removes the identities config file
func Delete() error {
	if err := os.Remove(identitiesPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SetIdentityForFolder associates an identity with a folder
func (c *Config) SetIdentityForFolder(folder string, id identity.Identity) {
	c.FolderIdentities[folder] = id
}

// GetIdentityForFolder returns the identity for a folder, if set
func (c *Config) GetIdentityForFolder(folder string) (identity.Identity, bool) {
	id, ok := c.FolderIdentities[folder]
	return id, ok
}

// UpdateIdentities merges newly discovered identities with stored ones
func (c *Config) UpdateIdentities(ids []identity.Identity) {
	seen := make(map[string]bool)
	for _, id := range c.Identities {
		seen[id.Email] = true
	}
	for _, id := range ids {
		if !seen[id.Email] {
			c.Identities = append(c.Identities, id)
			seen[id.Email] = true
		}
	}
}

// ============ Rules Config ============

// Rule maps a path pattern to an identity email
type Rule struct {
	Pattern string `json:"pattern"` // e.g., "github.com/vosamoilenko" or "~/work"
	Email   string `json:"email"`
}

// RulesConfig holds auto-switch rules
type RulesConfig struct {
	Rules []Rule `json:"rules"`
}

func rulesPath() string {
	return filepath.Join(configDir, "rules.json")
}

// LoadRules reads the rules config from disk
func LoadRules() (*RulesConfig, error) {
	cfg := &RulesConfig{Rules: []Rule{}}

	data, err := os.ReadFile(rulesPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes the rules config to disk
func (r *RulesConfig) Save() error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(rulesPath(), data, 0644)
}

// AddRule adds a new rule or updates existing one
func (r *RulesConfig) AddRule(pattern, email string) {
	for i, rule := range r.Rules {
		if rule.Pattern == pattern {
			r.Rules[i].Email = email
			return
		}
	}
	r.Rules = append(r.Rules, Rule{Pattern: pattern, Email: email})
}

// RemoveRule removes a rule by pattern
func (r *RulesConfig) RemoveRule(pattern string) bool {
	for i, rule := range r.Rules {
		if rule.Pattern == pattern {
			r.Rules = append(r.Rules[:i], r.Rules[i+1:]...)
			return true
		}
	}
	return false
}

// FindRuleForPath finds the best matching rule for a path
func (r *RulesConfig) FindRuleForPath(path string) *Rule {
	var bestMatch *Rule
	bestLen := 0
	for i, rule := range r.Rules {
		if matchesPattern(path, rule.Pattern) && len(rule.Pattern) > bestLen {
			bestMatch = &r.Rules[i]
			bestLen = len(rule.Pattern)
		}
	}
	return bestMatch
}

// matchesPattern checks if path contains the pattern
func matchesPattern(path, pattern string) bool {
	// Expand ~ in pattern
	if len(pattern) > 0 && pattern[0] == '~' {
		home, _ := os.UserHomeDir()
		pattern = home + pattern[1:]
	}
	// Simple contains match - patterns like "github.com/user" or "/full/path"
	return len(pattern) > 0 && strings.Contains(path, pattern)
}

// ============ Settings Config ============

// Settings holds user preferences
type Settings struct {
	AutoApply bool `json:"auto_apply"` // false = warn, true = auto-set identity
}

func settingsPath() string {
	return filepath.Join(configDir, "settings.json")
}

// LoadSettings reads the settings from disk
func LoadSettings() (*Settings, error) {
	s := &Settings{AutoApply: false}

	data, err := os.ReadFile(settingsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}

	return s, nil
}

// Save writes the settings to disk
func (s *Settings) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath(), data, 0644)
}
