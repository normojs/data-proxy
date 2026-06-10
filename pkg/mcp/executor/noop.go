package executor

import (
	"context"

	"github.com/QuantumNous/new-api/model"
)

type NoopExecutor struct{}

func NewNoopExecutor() NoopExecutor {
	return NoopExecutor{}
}

func (NoopExecutor) Supports(tool model.MCPTool) bool {
	return true
}

func (NoopExecutor) Execute(ctx context.Context, req Request) (Result, error) {
	return Result{}, &ExecutionError{
		Code:    ErrorCodeNotImplemented,
		Message: "Tool execution is not implemented yet",
		Err:     ErrExecutorNotImplemented,
	}
}
