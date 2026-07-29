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
