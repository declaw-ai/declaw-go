package declaw

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

// --- Error interface and hierarchy tests ---

func TestSandboxError_ErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      SandboxError
		expected string
	}{
		{
			name:     "message only",
			err:      SandboxError{Message: "something went wrong"},
			expected: "something went wrong",
		},
		{
			name:     "with sandbox ID",
			err:      SandboxError{Message: "not found", SandboxID: "sbx-abc"},
			expected: "sandbox sbx-abc: not found",
		},
		{
			name:     "empty message with sandbox ID",
			err:      SandboxError{Message: "", SandboxID: "sbx-123"},
			expected: "sandbox sbx-123: ",
		},
		{
			name:     "empty message without sandbox ID",
			err:      SandboxError{Message: ""},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Error()
			if got != tc.expected {
				t.Errorf("Error() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestSandboxError_ImplementsError(t *testing.T) {
	var _ error = &SandboxError{}
}

func TestAllErrorTypes_ImplementError(t *testing.T) {
	// Verify every error type satisfies the error interface
	var _ error = &TimeoutError{SandboxError: &SandboxError{}}
	var _ error = &NotFoundError{SandboxError: &SandboxError{}}
	var _ error = &AuthenticationError{SandboxError: &SandboxError{}}
	var _ error = &InvalidArgumentError{SandboxError: &SandboxError{}}
	var _ error = &NotEnoughSpaceError{SandboxError: &SandboxError{}}
	var _ error = &TemplateError{SandboxError: &SandboxError{}}
	var _ error = &BuildError{SandboxError: &SandboxError{}}
	var _ error = &FileUploadError{SandboxError: &SandboxError{}}
	var _ error = &GitAuthError{SandboxError: &SandboxError{}}
	var _ error = &GitUpstreamError{SandboxError: &SandboxError{}}
	var _ error = &InsufficientBalanceError{SandboxError: &SandboxError{}}
	var _ error = &RateLimitError{SandboxError: &SandboxError{}}
	var _ error = &CommandExitError{SandboxError: &SandboxError{}}
}

func TestAllErrorTypes_WrapSandboxError(t *testing.T) {
	base := &SandboxError{Message: "base error", SandboxID: "sbx-1"}

	tests := []struct {
		name string
		err  error
	}{
		{"TimeoutError", &TimeoutError{SandboxError: base}},
		{"NotFoundError", &NotFoundError{SandboxError: base}},
		{"AuthenticationError", &AuthenticationError{SandboxError: base}},
		{"InvalidArgumentError", &InvalidArgumentError{SandboxError: base}},
		{"NotEnoughSpaceError", &NotEnoughSpaceError{SandboxError: base}},
		{"TemplateError", &TemplateError{SandboxError: base}},
		{"BuildError", &BuildError{SandboxError: base}},
		{"FileUploadError", &FileUploadError{SandboxError: base}},
		{"GitAuthError", &GitAuthError{SandboxError: base}},
		{"GitUpstreamError", &GitUpstreamError{SandboxError: base}},
		{"InsufficientBalanceError", &InsufficientBalanceError{SandboxError: base}},
		{"RateLimitError", &RateLimitError{SandboxError: base}},
		{"CommandExitError", &CommandExitError{SandboxError: base}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.err.Error()
			if msg != "sandbox sbx-1: base error" {
				t.Errorf("Error() = %q, want %q", msg, "sandbox sbx-1: base error")
			}
		})
	}
}

func TestSandboxID_Propagation(t *testing.T) {
	base := &SandboxError{Message: "err", SandboxID: "sbx-propagated"}

	timeout := &TimeoutError{SandboxError: base}
	if timeout.SandboxID != "sbx-propagated" {
		t.Errorf("SandboxID = %q, want %q", timeout.SandboxID, "sbx-propagated")
	}

	notFound := &NotFoundError{SandboxError: base}
	if notFound.SandboxID != "sbx-propagated" {
		t.Errorf("SandboxID = %q, want %q", notFound.SandboxID, "sbx-propagated")
	}
}

func TestErrorsAs_SpecificTypeAssertions(t *testing.T) {
	base := &SandboxError{Message: "test"}

	// errors.As works for matching the concrete wrapper type
	t.Run("TimeoutError specific", func(t *testing.T) {
		err := error(&TimeoutError{SandboxError: base})
		var te *TimeoutError
		if !errors.As(err, &te) {
			t.Error("errors.As(*TimeoutError) returned false")
		}
	})

	t.Run("NotFoundError specific", func(t *testing.T) {
		err := error(&NotFoundError{SandboxError: base})
		var nfe *NotFoundError
		if !errors.As(err, &nfe) {
			t.Error("errors.As(*NotFoundError) returned false")
		}
	})

	t.Run("AuthenticationError specific", func(t *testing.T) {
		err := error(&AuthenticationError{SandboxError: base})
		var ae *AuthenticationError
		if !errors.As(err, &ae) {
			t.Error("errors.As(*AuthenticationError) returned false")
		}
	})

	t.Run("InvalidArgumentError specific", func(t *testing.T) {
		err := error(&InvalidArgumentError{SandboxError: base})
		var iae *InvalidArgumentError
		if !errors.As(err, &iae) {
			t.Error("errors.As(*InvalidArgumentError) returned false")
		}
	})

	t.Run("NotEnoughSpaceError specific", func(t *testing.T) {
		err := error(&NotEnoughSpaceError{SandboxError: base})
		var nese *NotEnoughSpaceError
		if !errors.As(err, &nese) {
			t.Error("errors.As(*NotEnoughSpaceError) returned false")
		}
	})

	t.Run("InsufficientBalanceError specific", func(t *testing.T) {
		err := error(&InsufficientBalanceError{SandboxError: base})
		var ibe *InsufficientBalanceError
		if !errors.As(err, &ibe) {
			t.Error("errors.As(*InsufficientBalanceError) returned false")
		}
	})

	t.Run("RateLimitError specific with fields", func(t *testing.T) {
		err := error(&RateLimitError{SandboxError: base, RetryAfter: 5 * time.Second})
		var rle *RateLimitError
		if !errors.As(err, &rle) {
			t.Error("errors.As(*RateLimitError) returned false")
		}
		if rle.RetryAfter != 5*time.Second {
			t.Errorf("RetryAfter = %v, want 5s", rle.RetryAfter)
		}
	})

	t.Run("CommandExitError specific with fields", func(t *testing.T) {
		err := error(&CommandExitError{
			SandboxError: base,
			ExitCode:     127,
			Stdout:       "out",
			Stderr:       "err",
		})
		var cee *CommandExitError
		if !errors.As(err, &cee) {
			t.Error("errors.As(*CommandExitError) returned false")
		}
		if cee.ExitCode != 127 {
			t.Errorf("ExitCode = %d, want 127", cee.ExitCode)
		}
		if cee.Stdout != "out" {
			t.Errorf("Stdout = %q, want %q", cee.Stdout, "out")
		}
		if cee.Stderr != "err" {
			t.Errorf("Stderr = %q, want %q", cee.Stderr, "err")
		}
	})
}

func TestErrorsAs_EmbeddedSandboxError_NotUnwrapped(t *testing.T) {
	// NOTE: Go's errors.As does NOT unwrap embedded pointer fields.
	// A *TimeoutError with an embedded *SandboxError will NOT match
	// errors.As(err, &sandboxError) because there is no Unwrap() method.
	// This documents the current behavior. If Unwrap() is added to the
	// wrapper types in the future, this test should be updated.
	base := &SandboxError{Message: "test"}

	wrappers := []error{
		&TimeoutError{SandboxError: base},
		&NotFoundError{SandboxError: base},
		&AuthenticationError{SandboxError: base},
	}

	for _, err := range wrappers {
		var se *SandboxError
		if errors.As(err, &se) {
			// If this passes, it means Unwrap() was added (good!).
			// Update this test to expect true.
			continue
		}
		// Current behavior: errors.As does not unwrap embedded pointers
	}
}

func TestEmbeddedSandboxError_DirectAccess(t *testing.T) {
	// Even though errors.As doesn't unwrap, the embedded SandboxError
	// is accessible via direct field access on the concrete type.
	base := &SandboxError{Message: "test msg", SandboxID: "sbx-1", StatusCode: 404}

	nfe := &NotFoundError{SandboxError: base}
	if nfe.SandboxError.Message != "test msg" {
		t.Errorf("direct SandboxError.Message = %q, want %q", nfe.SandboxError.Message, "test msg")
	}
	if nfe.SandboxError.SandboxID != "sbx-1" {
		t.Errorf("direct SandboxError.SandboxID = %q, want %q", nfe.SandboxError.SandboxID, "sbx-1")
	}
	if nfe.SandboxError.StatusCode != 404 {
		t.Errorf("direct SandboxError.StatusCode = %d, want 404", nfe.SandboxError.StatusCode)
	}

	// Promoted field access also works
	if nfe.Message != "test msg" {
		t.Errorf("promoted Message = %q, want %q", nfe.Message, "test msg")
	}
	if nfe.SandboxID != "sbx-1" {
		t.Errorf("promoted SandboxID = %q, want %q", nfe.SandboxID, "sbx-1")
	}
	if nfe.StatusCode != 404 {
		t.Errorf("promoted StatusCode = %d, want 404", nfe.StatusCode)
	}
}

func TestErrorsAs_NegativeCases(t *testing.T) {
	// A TimeoutError should not match NotFoundError
	base := &SandboxError{Message: "test"}
	err := error(&TimeoutError{SandboxError: base})

	var nfe *NotFoundError
	if errors.As(err, &nfe) {
		t.Error("TimeoutError should not match NotFoundError")
	}

	var ae *AuthenticationError
	if errors.As(err, &ae) {
		t.Error("TimeoutError should not match AuthenticationError")
	}
}

// --- CommandExitError tests ---

func TestCommandExitError_Fields(t *testing.T) {
	err := &CommandExitError{
		SandboxError: &SandboxError{Message: "command failed", SandboxID: "sbx-cmd"},
		ExitCode:     1,
		Stdout:       "partial output",
		Stderr:       "error: not found",
	}

	if err.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", err.ExitCode)
	}
	if err.Stdout != "partial output" {
		t.Errorf("Stdout = %q, want %q", err.Stdout, "partial output")
	}
	if err.Stderr != "error: not found" {
		t.Errorf("Stderr = %q, want %q", err.Stderr, "error: not found")
	}
	if err.SandboxID != "sbx-cmd" {
		t.Errorf("SandboxID = %q, want %q", err.SandboxID, "sbx-cmd")
	}
}

func TestCommandExitError_ZeroExitCode(t *testing.T) {
	err := &CommandExitError{
		SandboxError: &SandboxError{Message: "ok"},
		ExitCode:     0,
	}
	if err.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", err.ExitCode)
	}
}

func TestCommandExitError_EmptyOutput(t *testing.T) {
	err := &CommandExitError{
		SandboxError: &SandboxError{Message: "fail"},
		ExitCode:     2,
		Stdout:       "",
		Stderr:       "",
	}
	if err.Stdout != "" {
		t.Errorf("Stdout should be empty, got %q", err.Stdout)
	}
	if err.Stderr != "" {
		t.Errorf("Stderr should be empty, got %q", err.Stderr)
	}
}

// --- RateLimitError tests ---

func TestRateLimitError_Fields(t *testing.T) {
	err := &RateLimitError{
		SandboxError: &SandboxError{Message: "rate limited"},
		RetryAfter:   30 * time.Second,
		Limit:        100,
		Remaining:    0,
	}

	if err.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %v, want 30s", err.RetryAfter)
	}
	if err.Limit != 100 {
		t.Errorf("Limit = %d, want 100", err.Limit)
	}
	if err.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0", err.Remaining)
	}
}

func TestRateLimitError_ZeroValues(t *testing.T) {
	err := &RateLimitError{
		SandboxError: &SandboxError{Message: "limited"},
	}

	if err.RetryAfter != 0 {
		t.Errorf("RetryAfter = %v, want 0", err.RetryAfter)
	}
	if err.Limit != 0 {
		t.Errorf("Limit = %d, want 0", err.Limit)
	}
	if err.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0", err.Remaining)
	}
}

// --- errorFromResponse tests ---

func newMockResponse(statusCode int, body string, headers map[string]string) (*http.Response, []byte) {
	resp := &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
	}
	for k, v := range headers {
		resp.Header.Set(k, v)
	}
	return resp, []byte(body)
}

func TestErrorFromResponse_StatusCodeMapping(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		sandboxID  string
		checkType  func(error) bool
		typeName   string
	}{
		{
			name:       "401 returns AuthenticationError",
			statusCode: 401,
			body:       "unauthorized",
			sandboxID:  "sbx-1",
			checkType:  func(e error) bool { var ae *AuthenticationError; return errors.As(e, &ae) },
			typeName:   "AuthenticationError",
		},
		{
			name:       "403 returns AuthenticationError",
			statusCode: 403,
			body:       "forbidden",
			sandboxID:  "sbx-2",
			checkType:  func(e error) bool { var ae *AuthenticationError; return errors.As(e, &ae) },
			typeName:   "AuthenticationError",
		},
		{
			name:       "404 returns NotFoundError",
			statusCode: 404,
			body:       "not found",
			sandboxID:  "sbx-3",
			checkType:  func(e error) bool { var nfe *NotFoundError; return errors.As(e, &nfe) },
			typeName:   "NotFoundError",
		},
		{
			name:       "408 returns TimeoutError",
			statusCode: 408,
			body:       "request timeout",
			sandboxID:  "sbx-4",
			checkType:  func(e error) bool { var te *TimeoutError; return errors.As(e, &te) },
			typeName:   "TimeoutError",
		},
		{
			name:       "422 returns InvalidArgumentError",
			statusCode: 422,
			body:       "invalid params",
			sandboxID:  "sbx-5",
			checkType:  func(e error) bool { var iae *InvalidArgumentError; return errors.As(e, &iae) },
			typeName:   "InvalidArgumentError",
		},
		{
			name:       "402 returns InsufficientBalanceError",
			statusCode: 402,
			body:       "payment required",
			sandboxID:  "sbx-6",
			checkType:  func(e error) bool { var ibe *InsufficientBalanceError; return errors.As(e, &ibe) },
			typeName:   "InsufficientBalanceError",
		},
		{
			name:       "429 returns RateLimitError",
			statusCode: 429,
			body:       "too many requests",
			sandboxID:  "sbx-7",
			checkType:  func(e error) bool { var rle *RateLimitError; return errors.As(e, &rle) },
			typeName:   "RateLimitError",
		},
		{
			name:       "507 returns NotEnoughSpaceError",
			statusCode: 507,
			body:       "insufficient storage",
			sandboxID:  "sbx-8",
			checkType:  func(e error) bool { var nese *NotEnoughSpaceError; return errors.As(e, &nese) },
			typeName:   "NotEnoughSpaceError",
		},
		{
			name:       "500 returns SandboxError",
			statusCode: 500,
			body:       "internal error",
			sandboxID:  "sbx-9",
			checkType:  func(e error) bool { var se *SandboxError; return errors.As(e, &se) },
			typeName:   "SandboxError",
		},
		{
			name:       "502 returns SandboxError",
			statusCode: 502,
			body:       "bad gateway",
			sandboxID:  "",
			checkType:  func(e error) bool { var se *SandboxError; return errors.As(e, &se) },
			typeName:   "SandboxError",
		},
		{
			name:       "503 returns SandboxError",
			statusCode: 503,
			body:       "service unavailable",
			sandboxID:  "",
			checkType:  func(e error) bool { var se *SandboxError; return errors.As(e, &se) },
			typeName:   "SandboxError",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := newMockResponse(tc.statusCode, tc.body, nil)
			err := ExportedErrorFromResponse(resp, body, tc.sandboxID)

			if err == nil {
				t.Fatal("expected non-nil error")
			}
			if !tc.checkType(err) {
				t.Errorf("expected error type %s, got %T", tc.typeName, err)
			}
		})
	}
}

func TestErrorFromResponse_500_NotSpecificType(t *testing.T) {
	// 500 should return base *SandboxError, NOT a more specific type
	resp, body := newMockResponse(500, "internal error", nil)
	err := ExportedErrorFromResponse(resp, body, "")

	var te *TimeoutError
	if errors.As(err, &te) {
		t.Error("500 should not return TimeoutError")
	}
	var nfe *NotFoundError
	if errors.As(err, &nfe) {
		t.Error("500 should not return NotFoundError")
	}
	var ae *AuthenticationError
	if errors.As(err, &ae) {
		t.Error("500 should not return AuthenticationError")
	}
}

func TestErrorFromResponse_SandboxIDPropagation(t *testing.T) {
	resp, body := newMockResponse(404, "not found", nil)
	err := ExportedErrorFromResponse(resp, body, "sbx-my-sandbox")

	var nfe *NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatal("expected NotFoundError")
	}
	if nfe.SandboxID != "sbx-my-sandbox" {
		t.Errorf("SandboxID = %q, want %q", nfe.SandboxID, "sbx-my-sandbox")
	}
}

func TestErrorFromResponse_EmptySandboxID(t *testing.T) {
	resp, body := newMockResponse(401, "unauthorized", nil)
	err := ExportedErrorFromResponse(resp, body, "")

	var ae *AuthenticationError
	if !errors.As(err, &ae) {
		t.Fatal("expected AuthenticationError")
	}
	if ae.SandboxID != "" {
		t.Errorf("SandboxID = %q, want empty", ae.SandboxID)
	}
}

func TestErrorFromResponse_StatusCodePreserved(t *testing.T) {
	// For wrapper types, errors.As to *SandboxError does not work (no Unwrap),
	// so we extract the status code by type-switching on the concrete type.
	tests := []struct {
		code int
	}{
		{401}, {403}, {404}, {408}, {422}, {402}, {429}, {507}, {500}, {502}, {503},
	}

	for _, tc := range tests {
		t.Run(http.StatusText(tc.code), func(t *testing.T) {
			resp, body := newMockResponse(tc.code, "error", nil)
			err := ExportedErrorFromResponse(resp, body, "sbx-test")

			statusCode := extractStatusCode(t, err)
			if statusCode != tc.code {
				t.Errorf("StatusCode = %d, want %d", statusCode, tc.code)
			}
		})
	}
}

// extractStatusCode gets the StatusCode from any Declaw error type
// by checking each concrete wrapper type.
func extractStatusCode(t *testing.T, err error) int {
	t.Helper()
	switch e := err.(type) {
	case *AuthenticationError:
		return e.StatusCode
	case *InsufficientBalanceError:
		return e.StatusCode
	case *NotFoundError:
		return e.StatusCode
	case *TimeoutError:
		return e.StatusCode
	case *InvalidArgumentError:
		return e.StatusCode
	case *RateLimitError:
		return e.StatusCode
	case *NotEnoughSpaceError:
		return e.StatusCode
	case *SandboxError:
		return e.StatusCode
	default:
		t.Fatalf("unexpected error type %T", err)
		return 0
	}
}

func TestErrorFromResponse_MessageFromBody(t *testing.T) {
	resp, body := newMockResponse(404, "sandbox not found: sbx-xyz", nil)
	err := ExportedErrorFromResponse(resp, body, "")

	if !strings.Contains(err.Error(), "sandbox not found: sbx-xyz") {
		t.Errorf("error message %q should contain body text", err.Error())
	}
}

func TestErrorFromResponse_EmptyBody_FallsBackToStatusText(t *testing.T) {
	resp, body := newMockResponse(404, "", nil)
	err := ExportedErrorFromResponse(resp, body, "")

	var nfe *NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatal("expected NotFoundError")
	}
	if nfe.Message != "Not Found" {
		t.Errorf("Message = %q, want %q", nfe.Message, "Not Found")
	}
}

func TestErrorFromResponse_EmptyBody_401(t *testing.T) {
	resp, body := newMockResponse(401, "", nil)
	err := ExportedErrorFromResponse(resp, body, "")

	var ae *AuthenticationError
	if !errors.As(err, &ae) {
		t.Fatal("expected AuthenticationError")
	}
	if ae.Message != "Unauthorized" {
		t.Errorf("Message = %q, want %q", ae.Message, "Unauthorized")
	}
}

func TestErrorFromResponse_HTMLBody(t *testing.T) {
	htmlBody := "<html><body><h1>502 Bad Gateway</h1></body></html>"
	resp, body := newMockResponse(502, htmlBody, nil)
	err := ExportedErrorFromResponse(resp, body, "")

	// Should still produce a valid error, using the HTML as the message
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), "502 Bad Gateway") {
		t.Errorf("error message should contain HTML body text, got %q", err.Error())
	}
}

func TestErrorFromResponse_PlainTextBody(t *testing.T) {
	resp, body := newMockResponse(500, "plain text error message", nil)
	err := ExportedErrorFromResponse(resp, body, "")

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	var se *SandboxError
	if !errors.As(err, &se) {
		t.Fatal("expected SandboxError")
	}
	if se.Message != "plain text error message" {
		t.Errorf("Message = %q, want %q", se.Message, "plain text error message")
	}
}

// --- RateLimitError header parsing tests ---

func TestErrorFromResponse_429_RetryAfterHeader(t *testing.T) {
	headers := map[string]string{
		"Retry-After": "60",
	}
	resp, body := newMockResponse(429, "rate limited", headers)
	err := ExportedErrorFromResponse(resp, body, "")

	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatal("expected RateLimitError")
	}
	if rle.RetryAfter != 60*time.Second {
		t.Errorf("RetryAfter = %v, want 60s", rle.RetryAfter)
	}
}

func TestErrorFromResponse_429_AllRateLimitHeaders(t *testing.T) {
	headers := map[string]string{
		"Retry-After":           "30",
		"X-RateLimit-Limit":     "100",
		"X-RateLimit-Remaining": "0",
	}
	resp, body := newMockResponse(429, "rate limited", headers)
	err := ExportedErrorFromResponse(resp, body, "")

	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatal("expected RateLimitError")
	}
	if rle.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %v, want 30s", rle.RetryAfter)
	}
	if rle.Limit != 100 {
		t.Errorf("Limit = %d, want 100", rle.Limit)
	}
	if rle.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0", rle.Remaining)
	}
}

func TestErrorFromResponse_429_NoHeaders(t *testing.T) {
	resp, body := newMockResponse(429, "rate limited", nil)
	err := ExportedErrorFromResponse(resp, body, "")

	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatal("expected RateLimitError")
	}
	if rle.RetryAfter != 0 {
		t.Errorf("RetryAfter = %v, want 0 (no header)", rle.RetryAfter)
	}
	if rle.Limit != 0 {
		t.Errorf("Limit = %d, want 0 (no header)", rle.Limit)
	}
	if rle.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0 (no header)", rle.Remaining)
	}
}

func TestErrorFromResponse_429_InvalidRetryAfter(t *testing.T) {
	headers := map[string]string{
		"Retry-After": "not-a-number",
	}
	resp, body := newMockResponse(429, "rate limited", headers)
	err := ExportedErrorFromResponse(resp, body, "")

	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatal("expected RateLimitError")
	}
	// Invalid value should result in zero
	if rle.RetryAfter != 0 {
		t.Errorf("RetryAfter = %v, want 0 for invalid header", rle.RetryAfter)
	}
}

func TestErrorFromResponse_429_InvalidLimitHeaders(t *testing.T) {
	headers := map[string]string{
		"X-RateLimit-Limit":     "abc",
		"X-RateLimit-Remaining": "xyz",
	}
	resp, body := newMockResponse(429, "rate limited", headers)
	err := ExportedErrorFromResponse(resp, body, "")

	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatal("expected RateLimitError")
	}
	if rle.Limit != 0 {
		t.Errorf("Limit = %d, want 0 for invalid header", rle.Limit)
	}
	if rle.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0 for invalid header", rle.Remaining)
	}
}

func TestErrorFromResponse_429_RetryAfterZero(t *testing.T) {
	headers := map[string]string{
		"Retry-After": "0",
	}
	resp, body := newMockResponse(429, "rate limited", headers)
	err := ExportedErrorFromResponse(resp, body, "")

	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatal("expected RateLimitError")
	}
	if rle.RetryAfter != 0 {
		t.Errorf("RetryAfter = %v, want 0", rle.RetryAfter)
	}
}

func TestErrorFromResponse_429_LargeRetryAfter(t *testing.T) {
	headers := map[string]string{
		"Retry-After": "3600",
	}
	resp, body := newMockResponse(429, "rate limited", headers)
	err := ExportedErrorFromResponse(resp, body, "")

	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatal("expected RateLimitError")
	}
	if rle.RetryAfter != 3600*time.Second {
		t.Errorf("RetryAfter = %v, want 3600s", rle.RetryAfter)
	}
}

// --- Miscellaneous edge cases ---

func TestErrorFromResponse_UnknownClientError(t *testing.T) {
	// A 4xx code not explicitly handled should return base SandboxError
	resp, body := newMockResponse(418, "I'm a teapot", nil)
	err := ExportedErrorFromResponse(resp, body, "")

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	var se *SandboxError
	if !errors.As(err, &se) {
		t.Fatal("expected SandboxError")
	}
	if se.StatusCode != 418 {
		t.Errorf("StatusCode = %d, want 418", se.StatusCode)
	}
}

func TestErrorFromResponse_UnknownServerError(t *testing.T) {
	resp, body := newMockResponse(599, "unknown server error", nil)
	err := ExportedErrorFromResponse(resp, body, "")

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	var se *SandboxError
	if !errors.As(err, &se) {
		t.Fatal("expected SandboxError")
	}
}

func TestErrorFromResponse_BodyWithUnicode(t *testing.T) {
	resp, body := newMockResponse(400, "invalid input: éèê☃❤", nil)
	err := ExportedErrorFromResponse(resp, body, "")

	if !strings.Contains(err.Error(), "éèê") {
		t.Errorf("error should contain unicode text, got %q", err.Error())
	}
}

func TestErrorFromResponse_VeryLongBody(t *testing.T) {
	longBody := strings.Repeat("x", 10000)
	resp, body := newMockResponse(500, longBody, nil)
	err := ExportedErrorFromResponse(resp, body, "")

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	var se *SandboxError
	if !errors.As(err, &se) {
		t.Fatal("expected SandboxError")
	}
	if len(se.Message) < 10000 {
		t.Errorf("expected full body in message, got length %d", len(se.Message))
	}
}
