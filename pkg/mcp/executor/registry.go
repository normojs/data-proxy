package executor

import "github.com/QuantumNous/new-api/model"

type Registry struct {
	executors []Executor
	fallback  Executor
}

func NewRegistry(executors ...Executor) *Registry {
	registry := &Registry{
		fallback: NewNoopExecutor(),
	}
	for _, executor := range executors {
		registry.Register(executor)
	}
	return registry
}

func (r *Registry) Register(executor Executor) {
	if r == nil || executor == nil {
		return
	}
	r.executors = append(r.executors, executor)
}

func (r *Registry) Resolve(tool model.MCPTool) Executor {
	if r == nil {
		return NewNoopExecutor()
	}
	for _, executor := range r.executors {
		if executor != nil && executor.Supports(tool) {
			return executor
		}
	}
	if r.fallback == nil {
		return NewNoopExecutor()
	}
	return r.fallback
}
