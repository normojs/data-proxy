package executor

import (
	"context"
	"errors"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

const (
	ErrorCodeNotImplemented = "NOT_IMPLEMENTED"
	ErrorCodeFailed         = "EXECUTOR_FAILED"
	ErrorCodeTimeout        = "EXECUTOR_TIMEOUT"
)

var (
	ErrExecutorNotImplemented = errors.New("mcp executor is not implemented")
	ErrExecutorTimeout        = errors.New("mcp executor timed out")
)

type Request struct {
	CallId    int64
	UserId    int
	TokenId   int
	RequestId string
	RequestIP string
	Tool      model.MCPTool
	Arguments map[string]any
}

type Result struct {
	Content         []dto.MCPContentBlock
	Metadata        map[string]any
	Summary         string
	DurationMS      int
	ResultSize      int
	BillingUnits    int
	BridgeSessionId string
	TargetClient    string
}

type Executor interface {
	Supports(tool model.MCPTool) bool
	Execute(ctx context.Context, req Request) (Result, error)
}

type ExecutionError struct {
	Code    string
	Message string
	Err     error
}

func (e *ExecutionError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return ErrorCodeFailed
}

func (e *ExecutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var execErr *ExecutionError
	if errors.As(err, &execErr) && execErr.Code != "" {
		return execErr.Code
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrExecutorTimeout) {
		return ErrorCodeTimeout
	}
	if errors.Is(err, ErrExecutorNotImplemented) {
		return ErrorCodeNotImplemented
	}
	return ErrorCodeFailed
}
