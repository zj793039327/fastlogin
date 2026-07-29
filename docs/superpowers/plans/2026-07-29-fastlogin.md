# fastlogin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a terminal-based interactive command launcher that lets users pick a server/command from a grouped, foldable, searchable list and hands the terminal over to a Runner (SSH or arbitrary command).

**Architecture:** Three layers — Config (YAML + `~/.ssh/config` merge), Runner (pluggable interface with SSH + Command implementations), TUI (bubbletea). Selecting an entry sets `selected` and quits bubbletea; `main` then runs the Runner directly against the restored terminal (no PTY-inside-bubbletea complexity).

**Tech Stack:** Go 1.21+, `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`, `golang.org/x/crypto/ssh`, `golang.org/x/term`, `github.com/creack/pty`, `gopkg.in/yaml.v3`. Module name: `fastlogin`.

**Spec:** `docs/superpowers/specs/2026-07-29-fastlogin-design.md`

---

## File Map

- Create: `go.mod`, `main.go`, `.gitignore`
- Create: `internal/config/config.go` — Entry/Group/Auth/Config structs, defaults, Address(), ExpandPath(), DefaultPath()
- Create: `internal/config/yaml.go` — parseYAML, applyDefaults
- Create: `internal/config/yaml_test.go` — YAML parse + defaults tests
- Create: `internal/config/sshconfig.go` — parseSSHConfig, loadSSHConfig, merge
- Create: `internal/config/sshconfig_test.go` — ssh-config parse + merge dedup tests
- Create: `internal/config/load.go` — Load (orchestrates yaml + sshconfig + loose→pseudo-group)
- Create: `internal/runner/runner.go` — Runner interface + Registry
- Create: `internal/runner/runner_test.go` — Registry tests
- Create: `internal/runner/pty.go` — setWinsize helper (creack/pty)
- Create: `internal/runner/command.go` — CommandRunner
- Create: `internal/runner/ssh.go` — SSHRunner + authMethods
- Create: `internal/tui/model.go` — Model, New, Init, row, rows(), matches, selectedEntry, currentRow, moveCursor
- Create: `internal/tui/model_test.go` — rows/fold/search/selection tests
- Create: `internal/tui/view.go` — View + renderRow
- Create: `internal/tui/update.go` — Update
- Create: `config.example.yaml`

All Go imports of internal packages use module path `fastlogin/internal/<pkg>`.

---

## Task 1: Project Scaffold

**Files:**
- Create: `20260729_fastlogin/go.mod`
- Create: `20260729_fastlogin/.gitignore`

- [ ] **Step 1: Initialize module**

Run from `/Users/zj/playground/20260729_fastlogin`:
```bash
go mod init fastlogin
```
Expected: creates `go.mod` with `module fastlogin`.

- [ ] **Step 2: Pin Go version to 1.21 (for builtin min/max)**

Edit `go.mod` so the `go` directive reads `go 1.21`.

- [ ] **Step 3: Create .gitignore**

`.gitignore`:
```
/fastlogin
*.exe
dist/
```

- [ ] **Step 4: Init git and commit**

```bash
git init && git add -A && git commit -m "chore: init fastlogin module"
```

---

## Task 2: Config Types & Helpers

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

`internal/config/config_test.go`:
```go
package config

import "testing"

func TestApplyDefaults(t *testing.T) {
	c := &Config{
		Groups: []Group{{
			Name: "g",
			Entries: []Entry{
				{Name: "a", Host: "h", User: "u"},
				{Name: "b", Type: "command", Command: "echo hi"},
			},
		}},
		Entries: []Entry{{Name: "c", Host: "h2", User: "u2", Port: 2222}},
	}
	applyDefaults(c)

	if c.Groups[0].Entries[0].Type != "ssh" {
		t.Errorf("default type = %q, want ssh", c.Groups[0].Entries[0].Type)
	}
	if c.Groups[0].Entries[0].Port != 22 {
		t.Errorf("default port = %d, want 22", c.Groups[0].Entries[0].Port)
	}
	if c.Groups[0].Entries[1].Type != "command" {
		t.Errorf("explicit type overwritten to %q", c.Groups[0].Entries[1].Type)
	}
	if c.Entries[0].Port != 2222 {
		t.Errorf("explicit port overwritten to %d", c.Entries[0].Port)
	}
}

func TestAddress(t *testing.T) {
	e := Entry{User: "root", Host: "1.2.3.4", Port: 22}
	if got := e.Address(); got != "root@1.2.3.4:22" {
		t.Errorf("Address = %q", got)
	}
}

func TestExpandPath(t *testing.T) {
	got, err := ExpandPath("~/foo")
	if err != nil {
		t.Fatal(err)
	}
	if got[len(got)-4:] != "/foo" {
		t.Errorf("ExpandPath(~/foo) = %q", got)
	}
	got2, _ := ExpandPath("/abs/path")
	if got2 != "/abs/path" {
		t.Errorf("ExpandPath(abs) = %q", got2)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/`
Expected: FAIL — `applyDefaults`, `Address`, `ExpandPath` undefined.

- [ ] **Step 3: Write the implementation**

`internal/config/config.go`:
```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add Entry/Group/Config types and helpers"
```

---

## Task 3: YAML Parser

**Files:**
- Create: `internal/config/yaml.go`
- Test: `internal/config/yaml_test.go`

- [ ] **Step 1: Add dependency**

Run: `go get gopkg.in/yaml.v3`

- [ ] **Step 2: Write the failing test**

`internal/config/yaml_test.go`:
```go
package config

import "testing"

func TestParseYAML(t *testing.T) {
	data := []byte(`
groups:
  - name: prod
    entries:
      - name: web-01
        host: 10.0.0.1
        user: root
        auth:
          password: "secret"
        tags: [web]
      - name: cmd-1
        type: command
        command: echo hi
entries:
  - name: loose
    host: 1.2.3.4
    user: root
    auth:
      pem: ~/.ssh/id_rsa
include_ssh_config: true
`)
	cfg, err := parseYAML(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IncludeSSHConfig != true {
		t.Error("IncludeSSHConfig not parsed")
	}
	if len(cfg.Groups) != 1 || cfg.Groups[0].Name != "prod" {
		t.Errorf("groups = %+v", cfg.Groups)
	}
	if len(cfg.Groups[0].Entries) != 2 {
		t.Fatalf("group entries = %d", len(cfg.Groups[0].Entries))
	}
	web := cfg.Groups[0].Entries[0]
	if web.Auth == nil || web.Auth.Password != "secret" {
		t.Errorf("web auth = %+v", web.Auth)
	}
	if len(web.Tags) != 1 || web.Tags[0] != "web" {
		t.Errorf("web tags = %v", web.Tags)
	}
	cmd := cfg.Groups[0].Entries[1]
	if cmd.Type != "command" || cmd.Command != "echo hi" {
		t.Errorf("cmd entry = %+v", cmd)
	}
	if len(cfg.Entries) != 1 || cfg.Entries[0].Auth.PEM != "~/.ssh/id_rsa" {
		t.Errorf("loose entries = %+v", cfg.Entries)
	}
}

func TestParseYAMLInvalid(t *testing.T) {
	_, err := parseYAML([]byte("groups: [unclosed"))
	if err == nil {
		t.Fatal("expected parse error")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestParseYAML`
Expected: FAIL — `parseYAML` undefined.

- [ ] **Step 4: Write the implementation**

`internal/config/yaml.go`:
```go
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
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/`
Expected: PASS (all config tests).

- [ ] **Step 6: Commit**

```bash
git add internal/config/yaml.go internal/config/yaml_test.go go.mod go.sum
git commit -m "feat(config): add YAML parser with defaults"
```

---

## Task 4: SSH Config Parser

**Files:**
- Create: `internal/config/sshconfig.go`
- Test: `internal/config/sshconfig_test.go`

- [ ] **Step 1: Write the failing test**

`internal/config/sshconfig_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestParseSSHConfig`
Expected: FAIL — `parseSSHConfig` undefined.

- [ ] **Step 3: Write the implementation**

`internal/config/sshconfig.go`:
```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Write merge dedup test**

Append to `internal/config/sshconfig_test.go`:
```go
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
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestMergeDedup`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/config/sshconfig.go internal/config/sshconfig_test.go
git commit -m "feat(config): parse ~/.ssh/config and merge with dedup"
```

---

## Task 5: Config Load Orchestrator

**Files:**
- Create: `internal/config/load.go`
- Create: `internal/config/load_test.go`

- [ ] **Step 1: Write the failing test**

`internal/config/load_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoad`
Expected: FAIL — `Load` undefined.

- [ ] **Step 3: Write the implementation**

`internal/config/load.go`:
```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/`
Expected: PASS (all config tests).

- [ ] **Step 5: Commit**

```bash
git add internal/config/load.go internal/config/load_test.go
git commit -m "feat(config): add Load orchestrator and DefaultPath"
```

---

## Task 6: Runner Interface & Registry

**Files:**
- Create: `internal/runner/runner.go`
- Test: `internal/runner/runner_test.go`

- [ ] **Step 1: Write the failing test**

`internal/runner/runner_test.go`:
```go
package runner

import (
	"context"
	"testing"

	"fastlogin/config"
)

type fakeRunner struct{ ran bool }

func (f *fakeRunner) Run(ctx context.Context, e config.Entry) error {
	f.ran = true
	return nil
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	ssh := &fakeRunner{}
	r.Register("ssh", ssh)

	got, err := r.Get(config.Entry{Type: "ssh"})
	if err != nil {
		t.Fatal(err)
	}
	if got != Runner(ssh) {
		t.Error("returned runner mismatch")
	}
}

func TestRegistryUnknownType(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get(config.Entry{Type: "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/`
Expected: FAIL — types undefined.

- [ ] **Step 3: Write the implementation**

`internal/runner/runner.go`:
```go
package runner

import (
	"context"
	"fmt"

	"fastlogin/config"
)

// Runner starts an interactive session for an entry and takes over the terminal.
type Runner interface {
	Run(ctx context.Context, e config.Entry) error
}

// Registry maps entry type names to Runner implementations.
type Registry struct {
	runners map[string]Runner
}

func NewRegistry() *Registry {
	return &Registry{runners: make(map[string]Runner)}
}

func (r *Registry) Register(typeName string, runner Runner) {
	r.runners[typeName] = runner
}

func (r *Registry) Get(e config.Entry) (Runner, error) {
	runner, ok := r.runners[e.Type]
	if !ok {
		return nil, fmt.Errorf("unknown entry type: %q", e.Type)
	}
	return runner, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runner/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/runner/runner.go internal/runner/runner_test.go
git commit -m "feat(runner): add Runner interface and Registry"
```

---

## Task 7: PTY Helper & CommandRunner

**Files:**
- Create: `internal/runner/pty.go`
- Create: `internal/runner/command.go`

- [ ] **Step 1: Add dependencies**

Run:
```bash
go get github.com/creack/pty
go get golang.org/x/term
```

- [ ] **Step 2: Write PTY helper**

`internal/runner/pty.go`:
```go
package runner

import (
	"os"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// attachSize copies the current terminal's size onto the PTY master.
func attachSize(ptmx *os.File) {
	fd := int(os.Stdin.Fd())
	if w, h, err := term.GetSize(fd); err == nil {
		_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(h), Cols: uint16(w)})
	}
}
```

- [ ] **Step 3: Write CommandRunner**

`internal/runner/command.go`:
```go
package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"fastlogin/config"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// CommandRunner execs an arbitrary shell command under a PTY.
type CommandRunner struct{}

func (r *CommandRunner) Run(ctx context.Context, e config.Entry) error {
	if e.Command == "" {
		return fmt.Errorf("entry %q has empty command", e.Name)
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", e.Command)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("start command %q: %w", e.Command, err)
	}
	defer ptmx.Close()
	attachSize(ptmx)

	rawRestore := makeRawStdin()
	if rawRestore != nil {
		defer rawRestore()
	}

	go io.Copy(ptmx, os.Stdin)
	go io.Copy(os.Stdout, ptmx)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("command %q exited: %w", e.Command, err)
	}
	return nil
}

// makeRawStdin puts the terminal in raw mode and returns a restore func, or nil
// if stdin is not a terminal.
func makeRawStdin() func() {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return nil
	}
	return func() { _ = term.Restore(fd, old) }
}
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./internal/runner/`
Expected: no output (success).

- [ ] **Step 5: Manual smoke test**

Create `/tmp/cmdtest.sh` later via the runner — for now just confirm build. Run `go vet ./internal/runner/`.
Expected: no issues.

- [ ] **Step 6: Commit**

```bash
git add internal/runner/pty.go internal/runner/command.go go.mod go.sum
git commit -m "feat(runner): add CommandRunner with PTY"
```

---

## Task 8: SSHRunner

**Files:**
- Create: `internal/runner/ssh.go`

- [ ] **Step 1: Add dependency**

Run: `go get golang.org/x/crypto/ssh`

- [ ] **Step 2: Write SSHRunner**

`internal/runner/ssh.go`:
```go
package runner

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"fastlogin/config"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// SSHRunner connects via the ssh library and attaches the local terminal.
type SSHRunner struct{}

func (r *SSHRunner) Run(ctx context.Context, e config.Entry) error {
	methods, err := authMethods(e)
	if err != nil {
		return err
	}
	cfg := &ssh.ClientConfig{
		User:            e.User,
		Auth:            methods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	addr := net.JoinHostPort(e.Host, fmt.Sprintf("%d", e.Port))

	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("stdin is not a terminal")
	}
	if restore := makeRawStdin(); restore != nil {
		defer restore()
	}

	w, h, _ := term.GetSize(fd)
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", h, w, modes); err != nil {
		return fmt.Errorf("request pty: %w", err)
	}
	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	if err := session.Shell(); err != nil {
		return fmt.Errorf("start shell: %w", err)
	}
	return session.Wait()
}

// authMethods builds ssh.AuthMethod list from the entry's Auth.
func authMethods(e config.Entry) ([]ssh.AuthMethod, error) {
	if e.Auth == nil {
		return nil, fmt.Errorf("entry %q has no auth", e.Name)
	}
	var methods []ssh.AuthMethod
	if e.Auth.Password != "" {
		methods = append(methods, ssh.Password(e.Auth.Password))
	}
	if e.Auth.PEM != "" {
		path, err := config.ExpandPath(e.Auth.PEM)
		if err != nil {
			return nil, fmt.Errorf("expand pem path: %w", err)
		}
		key, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read pem %s: %w", path, err)
		}
		var signer ssh.Signer
		if e.Auth.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(e.Auth.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(key)
		}
		if err != nil {
			return nil, fmt.Errorf("parse pem: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("entry %q auth has neither password nor pem", e.Name)
	}
	return methods, nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/runner/`
Expected: no output.

Run: `go vet ./internal/runner/`
Expected: no issues.

- [ ] **Step 4: Commit**

```bash
git add internal/runner/ssh.go go.mod go.sum
git commit -m "feat(runner): add SSHRunner with password/pem auth"
```

---

## Task 9: TUI Model (rows, fold, search, selection)

**Files:**
- Create: `internal/tui/model.go`
- Test: `internal/tui/model_test.go`

- [ ] **Step 1: Add dependencies**

Run:
```bash
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/lipgloss
```

- [ ] **Step 2: Write the failing test**

`internal/tui/model_test.go`:
```go
package tui

import (
	"testing"

	"fastlogin/config"
)

func sampleConfig() *config.Config {
	return &config.Config{
		Groups: []config.Group{
			{Name: "prod", Entries: []config.Entry{
				{Name: "web", User: "root", Host: "10.0.0.1", Port: 22, Type: "ssh", Tags: []string{"web"}},
				{Name: "db", User: "admin", Host: "10.0.0.2", Port: 22, Type: "ssh"},
			}},
			{Name: "", Entries: []config.Entry{
				{Name: "loose", User: "u", Host: "1.2.3.4", Port: 22, Type: "ssh"},
			}},
		},
	}
}

func TestRowsExpanded(t *testing.T) {
	m := New(sampleConfig())
	rows := m.rows()
	// prod-header, web, db, loose = 4
	if len(rows) != 4 {
		t.Fatalf("rows = %d, %+v", len(rows), rows)
	}
	if rows[0].Kind != RowGroup || rows[0].GroupIdx != 0 {
		t.Errorf("rows[0] = %+v", rows[0])
	}
	if rows[3].EntryIdx != 0 || rows[3].GroupIdx != 1 {
		t.Errorf("loose row = %+v", rows[3])
	}
}

func TestRowsCollapsedHidesEntries(t *testing.T) {
	m := New(sampleConfig())
	m.expanded[0] = false
	rows := m.rows()
	// prod-header only, loose-header-less + loose entry = 2
	if len(rows) != 2 {
		t.Fatalf("rows = %d, %+v", len(rows), rows)
	}
	if rows[0].Kind != RowGroup {
		t.Errorf("rows[0] should be group header")
	}
}

func TestRowsSearchFilter(t *testing.T) {
	m := New(sampleConfig())
	m.searching = true
	m.search = "web"
	rows := m.rows()
	// prod-header + web (db hidden, empty-name group hidden because no match)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, %+v", len(rows), rows)
	}
	if rows[1].EntryIdx != 0 {
		t.Errorf("searched entry row = %+v", rows[1])
	}
}

func TestSearchHidesEmptyNamedGroups(t *testing.T) {
	m := New(sampleConfig())
	m.searching = true
	m.search = "nomatch"
	rows := m.rows()
	if len(rows) != 0 {
		t.Fatalf("expected no rows, got %+v", rows)
	}
}

func TestMoveCursorWraps(t *testing.T) {
	m := New(sampleConfig())
	m.moveCursor(1)
	if m.cursor != 1 {
		t.Errorf("cursor = %d", m.cursor)
	}
	m.cursor = 3
	m.moveCursor(1)
	if m.cursor != 0 {
		t.Errorf("wrap to top failed, cursor = %d", m.cursor)
	}
	m.moveCursor(-1)
	if m.cursor != 3 {
		t.Errorf("wrap to bottom failed, cursor = %d", m.cursor)
	}
}

func TestSelectedEntry(t *testing.T) {
	m := New(sampleConfig())
	m.cursor = 1 // web
	e := m.selectedEntry()
	if e == nil || e.Name != "web" {
		t.Errorf("selected = %+v", e)
	}
	m.cursor = 0 // group header
	if m.selectedEntry() != nil {
		t.Error("group header should not select an entry")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/tui/`
Expected: FAIL — `New`, `rows`, types undefined.

- [ ] **Step 4: Write the implementation**

`internal/tui/model.go`:
```go
package tui

import (
	"strings"

	"github.com/charmbracelet/bubbletea"

	"fastlogin/config"
)

// RowKind distinguishes group-header rows from entry rows in the flattened list.
type RowKind int

const (
	RowGroup RowKind = iota
	RowEntry
)

type row struct {
	Kind     RowKind
	GroupIdx int
	EntryIdx int
}

// Model is the bubbletea model. It is a value type: Update returns a copy.
type Model struct {
	cfg       *config.Config
	cursor    int
	expanded  map[int]bool
	searching bool
	search    string
	width     int
	height    int
	selected  *config.Entry
	err       string
}

// New builds a Model with all groups expanded by default.
func New(cfg *config.Config) Model {
	expanded := make(map[int]bool, len(cfg.Groups))
	for i := range cfg.Groups {
		expanded[i] = true
	}
	return Model{cfg: cfg, expanded: expanded}
}

func (m Model) Init() tea.Cmd { return nil }

// Selected returns the entry chosen via Enter, or nil.
func (m Model) Selected() *config.Entry { return m.selected }

// rows flattens the visible structure honoring fold + search state.
func (m Model) rows() []row {
	var out []row
	q := strings.ToLower(m.search)
	searching := m.searching && m.search != ""
	for gi, g := range m.cfg.Groups {
		var idxs []int
		for ei, e := range g.Entries {
			if searching && !matches(e, q) {
				continue
			}
			idxs = append(idxs, ei)
		}
		// Named group: emit header; hide entirely if a search yields no hits.
		if g.Name != "" {
			if searching && len(idxs) == 0 {
				continue
			}
			out = append(out, row{Kind: RowGroup, GroupIdx: gi})
			if !m.expanded[gi] {
				continue
			}
		}
		for _, ei := range idxs {
			out = append(out, row{Kind: RowEntry, GroupIdx: gi, EntryIdx: ei})
		}
	}
	return out
}

func matches(e config.Entry, q string) bool {
	if strings.Contains(strings.ToLower(e.Name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Host), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.User), q) {
		return true
	}
	for _, t := range e.Tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

func (m Model) currentRow() *row {
	rows := m.rows()
	if m.cursor < 0 || m.cursor >= len(rows) {
		return nil
	}
	r := rows[m.cursor]
	return &r
}

func (m Model) selectedEntry() *config.Entry {
	r := m.currentRow()
	if r == nil || r.Kind != RowEntry {
		return nil
	}
	e := m.cfg.Groups[r.GroupIdx].Entries[r.EntryIdx]
	return &e
}

// moveCursor advances by delta, wrapping around the visible rows.
func (m *Model) moveCursor(delta int) {
	rows := m.rows()
	n := len(rows)
	if n == 0 {
		m.cursor = 0
		return
	}
	m.cursor = ((m.cursor+delta)%n + n) % n
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/tui/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/model.go internal/tui/model_test.go go.mod go.sum
git commit -m "feat(tui): add Model with flatten/fold/search/selection"
```

---

## Task 10: TUI View

**Files:**
- Create: `internal/tui/view.go`

- [ ] **Step 1: Write the view**

`internal/tui/view.go`:
```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	groupStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("fastlogin"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", max(m.width, 50)))
	b.WriteString("\n")

	rows := m.rows()
	if len(rows) == 0 {
		b.WriteString(dimStyle.Render("  (no matching entries)"))
		b.WriteString("\n")
	} else {
		for i, r := range rows {
			line := m.renderRow(r)
			marker := "  "
			if i == m.cursor && !m.searching {
				line = cursorStyle.Render(line)
				marker = "▶ "
			}
			b.WriteString(marker + line + "\n")
		}
	}

	b.WriteString(strings.Repeat("─", max(m.width, 50)))
	b.WriteString("\n")
	if m.searching {
		b.WriteString("search: " + m.search)
	} else {
		b.WriteString(dimStyle.Render("↑↓ navigate · →/← fold · ⏎ connect · / search · q quit"))
	}
	if m.err != "" {
		b.WriteString("\n" + errStyle.Render(m.err))
	}
	return b.String()
}

func (m Model) renderRow(r row) string {
	if r.Kind == RowGroup {
		g := m.cfg.Groups[r.GroupIdx]
		marker := "▼"
		if !m.expanded[r.GroupIdx] {
			marker = "▶"
		}
		return groupStyle.Render(fmt.Sprintf("%s %s", marker, g.Name)) +
			dimStyle.Render(fmt.Sprintf("  (%d)", len(g.Entries)))
	}
	g := m.cfg.Groups[r.GroupIdx]
	e := g.Entries[r.EntryIdx]
	indent := "    "
	if g.Name == "" {
		indent = ""
	}
	return fmt.Sprintf("%s%-14s %s@%s [%s]", indent, e.Name, e.User, e.Host, e.Type)
}
```

Note: `config` is not imported in view.go — entry fields are reached via the Model's `cfg` field, no direct type reference.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/tui/`
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/view.go
git commit -m "feat(tui): add View rendering"
```

---

## Task 11: TUI Update

**Files:**
- Create: `internal/tui/update.go`

- [ ] **Step 1: Write the update**

`internal/tui/update.go`:
```go
package tui

import "github.com/charmbracelet/bubbletea"

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "q":
			if m.searching {
				m.search += "q"
				m.cursor = 0
			} else {
				return m, tea.Quit
			}

		case "esc":
			m.searching = false
			m.search = ""
			m.cursor = 0

		case "/":
			m.searching = true
			m.search = ""
			m.cursor = 0

		case "enter":
			if e := m.selectedEntry(); e != nil {
				m.selected = e
				return m, tea.Quit
			}

		case "up":
			if m.searching {
				break
			}
			m.moveCursor(-1)

		case "down":
			if m.searching {
				break
			}
			m.moveCursor(1)

		case "right", "left", "tab":
			if r := m.currentRow(); r != nil && r.Kind == RowGroup {
				m.expanded[r.GroupIdx] = !m.expanded[r.GroupIdx]
			}

		case "backspace":
			if m.searching && len(m.search) > 0 {
				m.search = m.search[:len(m.search)-1]
				m.cursor = 0
			}

		default:
			if m.searching {
				ch := msg.String()
				if len(ch) == 1 && ch >= " " {
					m.search += ch
					m.cursor = 0
				}
			}
		}
	}
	return m, nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/tui/`
Expected: no output.

Run: `go vet ./internal/tui/`
Expected: no issues.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/update.go
git commit -m "feat(tui): add keyboard handling (nav/fold/search/connect)"
```

---

## Task 12: main.go Assembly

**Files:**
- Create: `main.go`

- [ ] **Step 1: Write main.go**

`main.go`:
```go
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"fastlogin/config"
	"fastlogin/internal/runner"
	"fastlogin/internal/tui"
)

func main() {
	cfgPath, err := config.DefaultPath()
	if err != nil {
		fail("locate config: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fail("config not found at %s\nCreate the directory and a config.yaml, see config.example.yaml.", cfgPath)
		}
		fail("%v", err)
	}

	total := 0
	for _, g := range cfg.Groups {
		total += len(g.Entries)
	}
	if total == 0 {
		fail("no entries configured in %s", cfgPath)
	}

	p := tea.NewProgram(tui.New(cfg), tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		fail("tui: %v", err)
	}

	model, ok := final.(tui.Model)
	if !ok || model.Selected() == nil {
		return // user quit without selecting
	}
	entry := *model.Selected()

	registry := runner.NewRegistry()
	registry.Register("ssh", &runner.SSHRunner{})
	registry.Register("command", &runner.CommandRunner{})

	r, err := registry.Get(entry)
	if err != nil {
		fail("%v", err)
	}
	if err := r.Run(context.Background(), entry); err != nil {
		fail("%v", err)
	}
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "fastlogin: "+format+"\n", a...)
	os.Exit(1)
}
```

- [ ] **Step 2: Build the binary**

Run: `go build -o fastlogin .`
Expected: creates `./fastlogin`.

- [ ] **Step 3: Commit**

```bash
git add main.go
git commit -m "feat: wire main (config → tui → runner)"
```

---

## Task 13: Example Config & End-to-End Verification

**Files:**
- Create: `config.example.yaml`

- [ ] **Step 1: Write example config**

`config.example.yaml`:
```yaml
# fastlogin example config. Copy to ~/.config/fastlogin/config.yaml

groups:
  - name: 生产环境
    description: 线上机器，谨慎操作
    entries:
      - name: web-01
        type: ssh
        host: 10.0.1.10
        user: root
        port: 22
        auth:
          password: "secret"
        tags: [web, prod]

      - name: db-01
        type: ssh
        host: 10.0.1.20
        user: admin
        auth:
          pem: ~/.ssh/db01.pem

  - name: 开发环境
    entries:
      - name: local-dev
        type: ssh
        host: 127.0.0.1
        user: dev
        auth:
          password: "dev123"

      - name: mysql-repl
        type: command
        command: mysql -h 10.0.1.20 -u admin -p

entries:
  - name: quick-ssh
    type: ssh
    host: 1.2.3.4
    user: root
    auth:
      pem: ~/.ssh/id_rsa

# Merge non-wildcard Hosts from ~/.ssh/config into a virtual "ssh-config" group.
include_ssh_config: true
```

- [ ] **Step 2: Run the full test suite**

Run: `go test ./...`
Expected: all packages PASS.

- [ ] **Step 3: Run go vet across the module**

Run: `go vet ./...`
Expected: no issues.

- [ ] **Step 4: Manual end-to-end smoke test (TUI only)**

Create a throwaway config and launch, verify the list renders, folding/search/quit work:
```bash
mkdir -p /tmp/fltest
cat > /tmp/fltest/config.yaml <<'EOF'
groups:
  - name: demo
    entries:
      - name: echo-hello
        type: command
        command: echo hello from fastlogin
      - name: nested-ssh
        type: ssh
        host: 127.0.0.1
        user: nobody
        port: 22
        auth: {password: x}
EOF
FASTLOGIN_CONFIG=/tmp/fltest/config.yaml ./fastlogin
```
Note: env override is NOT implemented (MVP reads DefaultPath only). For the smoke test, temporarily symlink: `mkdir -p ~/.config/fastlogin && cp /tmp/fltest/config.yaml ~/.config/fastlogin/config.yaml`, run `./fastlogin`, select `echo-hello`, confirm "hello from fastlogin" prints then program exits. Restore by deleting `~/.config/fastlogin/config.yaml`.

Expected behavior:
- Arrow keys move the `▶` cursor.
- `→`/`←` on "demo" folds/expands it.
- `/echo` filters to the echo entry; `Esc` clears.
- Enter on `echo-hello` runs it and exits.
- `q` quits without running.

- [ ] **Step 5: Commit**

```bash
git add config.example.yaml
git commit -m "docs: add example config"
```

---

## Verification Summary

After all tasks:
- `go build ./...` — compiles
- `go vet ./...` — clean
- `go test ./...` — config + runner + tui unit tests pass
- `./fastlogin` against a real config — TUI works; SSH/command sessions take over the terminal and exit on session end

## Notes for the Implementer

- The `selected` field on `Model` plus `tea.Quit` is the key simplification: bubbletea restores the terminal on quit, then `main` runs the Runner against a clean terminal. No PTY plumbing through bubbletea.
- `ssh.InsecureIgnoreHostKey` is acceptable for an MVP local launcher; note it in future work if hardening matters.
- Empty group name (`""`) is the sentinel for "no header, not foldable, always expanded" — used for top-level loose entries.
- Env-var config path override is intentionally out of scope (spec says DefaultPath). If the smoke test needs it, use the symlink trick in Task 13 Step 4.
