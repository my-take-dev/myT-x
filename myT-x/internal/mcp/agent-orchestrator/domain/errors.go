package domain

// OrchestratorErrorCode identifies a caller-safe error category for MCP tools.
type OrchestratorErrorCode string

const (
	ErrCodeAccessDenied OrchestratorErrorCode = "access_denied"
	ErrCodeNotFound     OrchestratorErrorCode = "not_found"
	ErrCodeValidation   OrchestratorErrorCode = "validation"
	ErrCodeConflict     OrchestratorErrorCode = "conflict"
	ErrCodeInternal     OrchestratorErrorCode = "internal"
)

// OrchestratorError preserves a caller-safe message while retaining internal
// error details for logging and errors.Is/errors.As checks.
type OrchestratorError struct {
	Code     OrchestratorErrorCode
	Message  string
	Reason   string
	Internal error
}

// NewOrchestratorError creates a classified error for MCP tool responses.
func NewOrchestratorError(code OrchestratorErrorCode, message, reason string, internal error) *OrchestratorError {
	if reason == "" {
		reason = message
	}
	return &OrchestratorError{
		Code:     code,
		Message:  message,
		Reason:   reason,
		Internal: internal,
	}
}

func (e *OrchestratorError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *OrchestratorError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Internal
}
