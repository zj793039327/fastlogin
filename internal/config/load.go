package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Load reads, parses, and normalizes the config at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg, err := parseYAML(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	applyDefaults(cfg)

	if cfg.IncludeSSHConfig {
		if sshEntries, err := loadSSHConfig(); err == nil && len(sshEntries) > 0 {
			merge(cfg, sshEntries)
		}
	}

	// Loose top-level entries become a final pseudo-group with an empty name
	// (rendered without a header, not foldable).
	if len(cfg.Entries) > 0 {
		cfg.Groups = append(cfg.Groups, Group{Name: "", Entries: cfg.Entries})
		cfg.Entries = nil
	}
	return cfg, nil
}

// DefaultPath returns ~/.config/fastlogin/config.yaml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "fastlogin", "config.yaml"), nil
}
