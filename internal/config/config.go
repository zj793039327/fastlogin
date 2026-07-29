package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Auth holds credentials for an SSH entry. Either Password or PEM is used.
type Auth struct {
	Password   string `yaml:"password,omitempty"`
	PEM        string `yaml:"pem,omitempty"`
	Passphrase string `yaml:"passphrase,omitempty"`
}

// Entry is one launchable item. All types share these fields.
type Entry struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type,omitempty"`
	Host    string   `yaml:"host,omitempty"`
	User    string   `yaml:"user,omitempty"`
	Port    int      `yaml:"port,omitempty"`
	Auth    *Auth    `yaml:"auth,omitempty"`
	Tags    []string `yaml:"tags,omitempty"`
	Command string   `yaml:"command,omitempty"`
}

// Group is a named, foldable collection of entries.
type Group struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description,omitempty"`
	Entries     []Entry `yaml:"entries"`
}

// Config is the top-level configuration document.
type Config struct {
	Groups           []Group `yaml:"groups"`
	Entries          []Entry `yaml:"entries"`
	IncludeSSHConfig bool    `yaml:"include_ssh_config"`
}

// Address returns the dedup key "user@host:port".
func (e Entry) Address() string {
	return fmt.Sprintf("%s@%s:%d", e.User, e.Host, e.Port)
}

// ExpandPath expands a leading ~ to the user's home directory.
func ExpandPath(p string) (string, error) {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p, err
		}
		return filepath.Join(home, p[1:]), nil
	}
	return p, nil
}

// applyDefaults fills in default Type ("ssh") and Port (22) for any entry
// that leaves them unset. Explicit values are preserved.
func applyDefaults(c *Config) {
	for i := range c.Groups {
		for j := range c.Groups[i].Entries {
			if c.Groups[i].Entries[j].Type == "" {
				c.Groups[i].Entries[j].Type = "ssh"
			}
			if c.Groups[i].Entries[j].Port == 0 {
				c.Groups[i].Entries[j].Port = 22
			}
		}
	}
	for i := range c.Entries {
		if c.Entries[i].Type == "" {
			c.Entries[i].Type = "ssh"
		}
		if c.Entries[i].Port == 0 {
			c.Entries[i].Port = 22
		}
	}
}
