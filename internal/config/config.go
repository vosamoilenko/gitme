package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/vosamoilenko/gitme/internal/identity"
)

// Config holds the application configuration
type Config struct {
	// FolderIdentities maps folder paths to their selected identity
	FolderIdentities map[string]identity.Identity `json:"folder_identities"`
	// Identities stores all known identities
	Identities []identity.Identity `json:"identities"`
}

var configPath string

func init() {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "gitme")
	os.MkdirAll(configDir, 0755)
	configPath = filepath.Join(configDir, "config.json")
}

// Load reads the config from disk
func Load() (*Config, error) {
	cfg := &Config{
		FolderIdentities: make(map[string]identity.Identity),
		Identities:       []identity.Identity{},
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
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

// Save writes the config to disk
func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
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
