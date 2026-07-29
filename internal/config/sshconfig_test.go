package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseSSHConfig(t *testing.T) {
	content := `
# comment
Host web-01
  HostName 10.0.0.1
  User root
  Port 2222
  IdentityFile ~/.ssh/web01.pem

Host wildcard-*
  HostName ignored
  User x

Host db-01
  HostName 10.0.0.2
  User admin
`
	path := writeTemp(t, "config", content)
	entries, err := parseSSHConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries: %+v", len(entries), entries)
	}
	want := Entry{Name: "web-01", Host: "10.0.0.1", User: "root", Port: 2222, Type: "ssh", Auth: &Auth{PEM: expandOK(t, "~/.ssh/web01.pem")}}
	got := entries[0]
	if got.Name != want.Name || got.Host != want.Host || got.User != want.User || got.Port != want.Port {
		t.Errorf("entries[0] = %+v, want %+v", got, want)
	}
	if got.Auth == nil || got.Auth.PEM != want.Auth.PEM {
		t.Errorf("entries[0].auth = %+v", got.Auth)
	}
	if entries[1].Name != "db-01" || entries[1].Port != 22 {
		t.Errorf("entries[1] = %+v", entries[1])
	}
}

func expandOK(t *testing.T, p string) string {
	t.Helper()
	e, err := ExpandPath(p)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestMergeDedup(t *testing.T) {
	c := &Config{Groups: []Group{{
		Name: "g",
		Entries: []Entry{
			{Name: "dup", User: "root", Host: "10.0.0.1", Port: 22},
		},
	}}}
	sshEntries := []Entry{
		{Name: "dup", User: "root", Host: "10.0.0.1", Port: 22},
		{Name: "new", User: "admin", Host: "10.0.0.2", Port: 22},
	}
	merge(c, sshEntries)
	if len(c.Groups) != 2 {
		t.Fatalf("groups = %d", len(c.Groups))
	}
	sc := c.Groups[1]
	if sc.Name != "ssh-config" || len(sc.Entries) != 1 || sc.Entries[0].Name != "new" {
		t.Errorf("ssh-config group = %+v", sc)
	}
}
