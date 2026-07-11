package executor

import (
	"context"
	"fmt"
	"strings"

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
	toolName := strings.TrimSpace(req.Tool.Name)
	if toolName == "" {
		toolName = "unknown"
	}
	source := strings.TrimSpace(req.Tool.Source)
	if source == "" {
		source = "unknown"
	}
	return Result{}, &ExecutionError{
		Code:    ErrorCodeUnsupported,
		Message: fmt.Sprintf("no MCP executor supports tool %q (source=%s, remote=%t)", toolName, source, req.Tool.IsRemote),
		Err:     ErrExecutorUnsupported,
	}
}
