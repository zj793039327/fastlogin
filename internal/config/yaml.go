package config

import "gopkg.in/yaml.v3"

// parseYAML decodes the config document. Does NOT apply defaults.
func parseYAML(data []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// applyDefaults fills in Type ("ssh") and Port (22) for entries missing them.
func applyDefaults(c *Config) {
	apply := func(e *Entry) {
		if e.Type == "" {
			e.Type = "ssh"
		}
		if e.Port == 0 {
			e.Port = 22
		}
	}
	for i := range c.Groups {
		for j := range c.Groups[i].Entries {
			apply(&c.Groups[i].Entries[j])
		}
	}
	for i := range c.Entries {
		apply(&c.Entries[i])
	}
}
