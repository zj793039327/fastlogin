package runner

import (
	"context"
	"testing"

	"fastlogin/internal/config"
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
