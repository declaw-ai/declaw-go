package declaw

import (
	"encoding/json"
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

	// Code is the API's machine-readable error code (the "code" field of the
	// error body), empty when the response carried none.
	//
	// Branch on this, never on Message. Messages are prose and change; codes are
	// contract. It matters most where one status means several unrelated things:
	// 409 on POST /sandboxes is either "idempotency_in_progress" (the original
	// create is still running — retry the identical request) or
	// "template_not_ready" (rebuild the template — retrying is pointless).
	Code string
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

	// BuildID, TemplateID and Logs identify the failed build and its template
	// and hold the build's full output, when the error comes from a build that
	// ran (BuildTemplate, RebuildTemplate). TemplateID is what RebuildTemplate
	// takes to retry it.
	BuildID    string
	TemplateID string
	Logs       []string
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

// ConflictError is returned when a request conflicts with the current state of
// the resource (HTTP 409). For volume writes guarded by if_version it indicates
// a CAS version mismatch; for locks it indicates the lock is held by another
// holder.
type ConflictError struct {
	*SandboxError
}

func (e *ConflictError) Unwrap() error { return e.SandboxError }

// VersionMismatchError is returned when an optimistic-concurrency (CAS) volume
// write fails because the on-disk version no longer matches the supplied
// if_version token (HTTP 409). It wraps ConflictError so callers may match
// either type.
type VersionMismatchError struct {
	*ConflictError
}

func (e *VersionMismatchError) Unwrap() error { return e.ConflictError }

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

	var parsed struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Message != "" {
		msg = parsed.Message
	}

	base := &SandboxError{
		Message:    msg,
		SandboxID:  sandboxID,
		StatusCode: resp.StatusCode,
		Code:       parsed.Code,
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

	case http.StatusConflict:
		return &ConflictError{SandboxError: base}

	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return &InvalidArgumentError{SandboxError: base}

	case http.StatusRequestEntityTooLarge:
		return &NotEnoughSpaceError{SandboxError: base}

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
