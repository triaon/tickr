package exchange

import (
	"context"
	"fmt"

	"github.com/etz/tickr/internal/model"
)

type Adapter interface {
	Name() string
	Fetch(ctx context.Context, req model.FetchRequest) ([]model.Symbol, []model.Warning, error)
}

type Registry struct {
	adapters map[string]Adapter
}

func NewRegistry() *Registry {
	return &Registry{adapters: map[string]Adapter{}}
}

func (r *Registry) Register(a Adapter) {
	r.adapters[a.Name()] = a
}

func (r *Registry) Get(name string) (Adapter, error) {
	a, ok := r.adapters[name]
	if !ok {
		return nil, fmt.Errorf("unknown exchange %q", name)
	}
	return a, nil
}

func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.adapters))
	for k := range r.adapters {
		out = append(out, k)
	}
	return out
}
