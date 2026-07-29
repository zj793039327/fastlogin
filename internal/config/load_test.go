package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLooseEntriesBecomePseudoGroup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`
entries:
  - name: loose
    host: 1.2.3.4
    user: root
    auth:
      password: x
`), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Groups) != 1 {
		t.Fatalf("groups = %+v", cfg.Groups)
	}
	if cfg.Groups[0].Name != "" {
		t.Errorf("pseudo group name = %q", cfg.Groups[0].Name)
	}
	if len(cfg.Groups[0].Entries) != 1 {
		t.Errorf("pseudo entries = %d", len(cfg.Groups[0].Entries))
	}
}

func TestLoadDefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`
groups:
  - name: g
    entries:
      - name: a
        host: h
        user: u
        auth: {password: p}
`), 0o644)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	e := cfg.Groups[0].Entries[0]
	if e.Type != "ssh" || e.Port != 22 {
		t.Errorf("defaults not applied: %+v", e)
	}
}
