package hermes

import (
	"errors"
	"fmt"
	"strings"
)

// HTTPError describes an unsuccessful response without retaining its body.
// Hermes error pages may contain configuration or authentication details, so
// callers can inspect the status while the client deliberately discards the
// response payload.
type HTTPError struct {
	Method     string
	Path       string
	StatusCode int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("hermes request %s %s failed with HTTP %d", e.Method, e.Path, e.StatusCode)
}

// IsNotFound reports whether an error represents a missing Hermes object.
func IsNotFound(err error) bool {
	var httpError *HTTPError
	return errors.As(err, &httpError) && httpError.StatusCode == 404
}

// OperationError describes an application-level failure returned in an
// otherwise successful JSON response. Hermes uses this shape for operations
// such as model assignment when a confirmation or another user action is
// required. The error intentionally contains only server-provided summary
// fields; response bodies are never retained wholesale.
type OperationError struct {
	Operation       string
	Message         string
	ConfirmRequired bool
}

func (e *OperationError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "Hermes rejected the operation"
	}
	if e.ConfirmRequired {
		return fmt.Sprintf("hermes %s requires confirmation: %s", e.Operation, message)
	}
	return fmt.Sprintf("hermes %s failed: %s", e.Operation, message)
}
