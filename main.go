package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"fastlogin/internal/config"
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
