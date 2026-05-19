package declaw

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// SandboxError is the base error type for all Declaw sandbox errors.
type SandboxError struct {
	// Message is a human-readable description of the error.
	Message string

	// SandboxID is the ID of the sandbox that caused the error, if applicable.
	SandboxID string

	// StatusCode is the HTTP status code from the API response, if applicable.
	StatusCode int
}

// Error implements the error interface.
func (e *SandboxError) Error() string {
	if e.SandboxID != "" {
		return fmt.Sprintf("sandbox %s: %s", e.SandboxID, e.Message)
	}
	return e.Message
}

// TimeoutError is returned when an operation exceeds its timeout.
type TimeoutError struct {
	*SandboxError
}

func (e *TimeoutError) Unwrap() error { return e.SandboxError }

// NotFoundError is returned when a sandbox or resource is not found.
type NotFoundError struct {
	*SandboxError
}

func (e *NotFoundError) Unwrap() error { return e.SandboxError }

// AuthenticationError is returned when the API key or access token is invalid.
type AuthenticationError struct {
	*SandboxError
}

func (e *AuthenticationError) Unwrap() error { return e.SandboxError }

// InvalidArgumentError is returned when invalid arguments are passed to an API call.
type InvalidArgumentError struct {
	*SandboxError
}

func (e *InvalidArgumentError) Unwrap() error { return e.SandboxError }

// NotEnoughSpaceError is returned when the sandbox runs out of disk space.
type NotEnoughSpaceError struct {
	*SandboxError
}

func (e *NotEnoughSpaceError) Unwrap() error { return e.SandboxError }

// TemplateError is returned on template build or retrieval errors.
type TemplateError struct {
	*SandboxError
}

func (e *TemplateError) Unwrap() error { return e.SandboxError }

// BuildError is returned when a template build fails.
type BuildError struct {
	*SandboxError
}

func (e *BuildError) Unwrap() error { return e.SandboxError }

// FileUploadError is returned when a file upload to the sandbox fails.
type FileUploadError struct {
	*SandboxError
}

func (e *FileUploadError) Unwrap() error { return e.SandboxError }

// GitAuthError is returned on git authentication errors inside the sandbox.
type GitAuthError struct {
	*SandboxError
}

func (e *GitAuthError) Unwrap() error { return e.SandboxError }

// GitUpstreamError is returned on git upstream errors inside the sandbox.
type GitUpstreamError struct {
	*SandboxError
}

func (e *GitUpstreamError) Unwrap() error { return e.SandboxError }

// InsufficientBalanceError is returned when the account has insufficient balance (HTTP 402).
type InsufficientBalanceError struct {
	*SandboxError
}

func (e *InsufficientBalanceError) Unwrap() error { return e.SandboxError }

// RateLimitError is returned when the API rate limit is exceeded (HTTP 429).
type RateLimitError struct {
	*SandboxError

	// RetryAfter is the duration to wait before retrying.
	RetryAfter time.Duration

	// Limit is the rate limit ceiling.
	Limit int

	// Remaining is the number of requests remaining in the current window.
	Remaining int
}

func (e *RateLimitError) Unwrap() error { return e.SandboxError }

// CommandExitError is returned when a command exits with a non-zero exit code.
type CommandExitError struct {
	*SandboxError

	// ExitCode is the process exit code.
	ExitCode int

	// Stdout is the captured standard output.
	Stdout string

	// Stderr is the captured standard error.
	Stderr string
}

func (e *CommandExitError) Unwrap() error { return e.SandboxError }

// errorFromResponse creates a typed error from an HTTP response.
// It maps HTTP status codes to the appropriate error types:
//   - 401, 403: AuthenticationError
//   - 402: InsufficientBalanceError
//   - 404: NotFoundError
//   - 408: TimeoutError
//   - 422: InvalidArgumentError
//   - 429: RateLimitError (parses Retry-After, X-RateLimit-Limit, X-RateLimit-Remaining headers)
//   - 507: NotEnoughSpaceError
//   - 5xx: SandboxError
func errorFromResponse(resp *http.Response, body []byte, sandboxID string) error {
	msg := string(body)
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}

	base := &SandboxError{
		Message:    msg,
		SandboxID:  sandboxID,
		StatusCode: resp.StatusCode,
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &AuthenticationError{SandboxError: base}

	case http.StatusPaymentRequired:
		return &InsufficientBalanceError{SandboxError: base}

	case http.StatusNotFound:
		return &NotFoundError{SandboxError: base}

	case http.StatusRequestTimeout:
		return &TimeoutError{SandboxError: base}

	case http.StatusUnprocessableEntity:
		return &InvalidArgumentError{SandboxError: base}

	case http.StatusTooManyRequests:
		rle := &RateLimitError{SandboxError: base}

		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				rle.RetryAfter = time.Duration(secs) * time.Second
			}
		}
		if lim := resp.Header.Get("X-RateLimit-Limit"); lim != "" {
			if n, err := strconv.Atoi(lim); err == nil {
				rle.Limit = n
			}
		}
		if rem := resp.Header.Get("X-RateLimit-Remaining"); rem != "" {
			if n, err := strconv.Atoi(rem); err == nil {
				rle.Remaining = n
			}
		}
		return rle

	case http.StatusInsufficientStorage:
		return &NotEnoughSpaceError{SandboxError: base}

	default:
		if resp.StatusCode >= 500 {
			return base
		}
		return base
	}
}
