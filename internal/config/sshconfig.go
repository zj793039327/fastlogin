package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// parseSSHConfig reads an OpenSSH config file and converts each non-wildcard
// Host block into an Entry. Hosts without a User are dropped (no way to log in).
func parseSSHConfig(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		entries []Entry
		cur     *Entry
	)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.ToLower(parts[0])
		val := parts[1]
		switch key {
		case "host":
			if cur != nil {
				entries = append(entries, *cur)
			}
			cur = &Entry{Name: val, Host: val, User: "", Type: "ssh", Port: 22}
		case "hostname":
			if cur != nil {
				cur.Host = val
			}
		case "user":
			if cur != nil {
				cur.User = val
			}
		case "port":
			if cur != nil {
				if p, err := strconv.Atoi(val); err == nil {
					cur.Port = p
				}
			}
		case "identityfile":
			if cur != nil {
				if expanded, err := ExpandPath(val); err == nil {
					cur.Auth = &Auth{PEM: expanded}
				}
			}
		}
	}
	if cur != nil {
		entries = append(entries, *cur)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var out []Entry
	for _, e := range entries {
		if strings.ContainsAny(e.Name, "*?") {
			continue
		}
		if e.User == "" {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// loadSSHConfig reads ~/.ssh/config into entries. Returns nil,nil if missing.
func loadSSHConfig() ([]Entry, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".ssh", "config")
	if _, err := os.Stat(path); err != nil {
		return nil, nil
	}
	return parseSSHConfig(path)
}

// merge adds ssh-config entries to the config as a virtual "ssh-config" group,
// skipping any whose Address() already exists.
func merge(c *Config, sshEntries []Entry) {
	existing := map[string]bool{}
	for _, g := range c.Groups {
		for _, e := range g.Entries {
			existing[e.Address()] = true
		}
	}
	var added []Entry
	for _, e := range sshEntries {
		if !existing[e.Address()] {
			added = append(added, e)
		}
	}
	if len(added) > 0 {
		c.Groups = append(c.Groups, Group{Name: "ssh-config", Entries: added})
	}
}
