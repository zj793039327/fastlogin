package runner

import (
	"context"
	"fmt"

	"fastlogin/internal/config"
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
