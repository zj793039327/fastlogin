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
