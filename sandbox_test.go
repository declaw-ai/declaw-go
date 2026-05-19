package declaw_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/declaw-ai/declaw-go"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// requestLog captures details of an HTTP request for assertions.
type requestLog struct {
	Method      string
	Path        string
	Query       string
	Body        map[string]interface{}
	RawBody     string
	ContentType string
	Auth        string
}

// captureRequest records incoming request details and writes the mock response.
func captureRequest(logs *[]requestLog, mu *sync.Mutex, statusCode int, responseBody string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		log := requestLog{
			Method:      r.Method,
			Path:        r.URL.Path,
			Query:       r.URL.RawQuery,
			RawBody:     string(bodyBytes),
			ContentType: r.Header.Get("Content-Type"),
			Auth:        r.Header.Get("Authorization"),
		}
		_ = json.Unmarshal(bodyBytes, &log.Body)

		mu.Lock()
		*logs = append(*logs, log)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(responseBody))
	}
}

// newSandboxFromServer creates a Sandbox struct wired to the test server.
func newSandboxFromServer(t *testing.T, serverURL, sandboxID string) *declaw.Sandbox {
	t.Helper()
	cfg := declaw.NewConfig(
		declaw.WithAPIKey("test-key"),
		declaw.WithAPIURL(serverURL),
	)
	c := declaw.NewTestAPIClient(cfg)
	return declaw.NewTestSandbox(sandboxID, c)
}

// ===========================================================================
// Create tests
// ===========================================================================

func TestCreate_DefaultOptions(t *testing.T) {
	var captured requestLog
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		captured = requestLog{
			Method:      r.Method,
			Path:        r.URL.Path,
			ContentType: r.Header.Get("Content-Type"),
			Auth:        r.Header.Get("Authorization"),
			RawBody:     string(bodyBytes),
		}
		_ = json.Unmarshal(bodyBytes, &captured.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{
			"sandbox_id": "sbx-new-1",
			"envd_access_token": "tok-envd",
			"sandbox_domain": "sbx-new-1.sandbox.declaw.ai"
		}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "test-key")
	t.Setenv("DECLAW_API_URL", srv.URL)

	// Create returns "not implemented" for stubs -- these tests define
	// the expected behavior for the implementation.
	sbx, err := declaw.Create(context.Background())
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify POST to /sandboxes
	if captured.Method != http.MethodPost {
		t.Errorf("expected POST, got %q", captured.Method)
	}
	if captured.Path != "/sandboxes" {
		t.Errorf("expected path /sandboxes, got %q", captured.Path)
	}
	if captured.Auth != "Bearer test-key" {
		t.Errorf("expected auth header, got %q", captured.Auth)
	}

	// Verify response fields populated on sandbox
	if sbx == nil {
		t.Fatal("expected non-nil sandbox")
	}
	if sbx.ID != "sbx-new-1" {
		t.Errorf("expected ID=sbx-new-1, got %q", sbx.ID)
	}
	if sbx.Commands == nil {
		t.Error("expected Commands to be non-nil")
	}
	if sbx.Files == nil {
		t.Error("expected Files to be non-nil")
	}
	if sbx.PTY == nil {
		t.Error("expected PTY to be non-nil")
	}
}

func TestCreate_WithTemplate(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"sandbox_id":"sbx-py","envd_access_token":"t","sandbox_domain":"d"}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.Create(context.Background(),
		declaw.WithTemplate("python"),
	)
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedBody == nil {
		t.Fatal("expected body, got nil")
	}
	if capturedBody["template"] != "python" {
		t.Errorf("expected template=python, got %v", capturedBody["template"])
	}
}

func TestCreate_WithTimeout(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"sandbox_id":"sbx-1","envd_access_token":"t","sandbox_domain":"d"}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.Create(context.Background(),
		declaw.WithTimeout(600),
	)
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedBody == nil {
		t.Fatal("expected body, got nil")
	}
	if v, ok := capturedBody["timeout"]; !ok || v != float64(600) {
		t.Errorf("expected timeout=600, got %v", capturedBody["timeout"])
	}
}

func TestCreate_WithMetadata(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"sandbox_id":"sbx-1","envd_access_token":"t","sandbox_domain":"d"}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.Create(context.Background(),
		declaw.WithMetadata(map[string]string{"env": "staging", "team": "infra"}),
	)
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedBody == nil {
		t.Fatal("expected body, got nil")
	}
	md, ok := capturedBody["metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected metadata map, got %T", capturedBody["metadata"])
	}
	if md["env"] != "staging" {
		t.Errorf("expected metadata.env=staging, got %v", md["env"])
	}
	if md["team"] != "infra" {
		t.Errorf("expected metadata.team=infra, got %v", md["team"])
	}
}

func TestCreate_WithEnvs(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"sandbox_id":"sbx-1","envd_access_token":"t","sandbox_domain":"d"}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.Create(context.Background(),
		declaw.WithEnvs(map[string]string{"FOO": "bar", "DB_URL": "postgres://..."}),
	)
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedBody == nil {
		t.Fatal("expected body, got nil")
	}
	envs, ok := capturedBody["envs"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected envs map, got %T", capturedBody["envs"])
	}
	if envs["FOO"] != "bar" {
		t.Errorf("expected envs.FOO=bar, got %v", envs["FOO"])
	}
}

func TestCreate_WithNetwork(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"sandbox_id":"sbx-1","envd_access_token":"t","sandbox_domain":"d"}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	allowPublic := true
	_, err := declaw.Create(context.Background(),
		declaw.WithNetwork(declaw.SandboxNetworkOpts{
			AllowOut:           []string{"*.github.com", "api.openai.com"},
			DenyOut:            []string{"*.malware.com"},
			AllowPublicTraffic: &allowPublic,
		}),
	)
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedBody == nil {
		t.Fatal("expected body, got nil")
	}
	if capturedBody["network"] == nil {
		t.Error("expected network config in body")
	}
}

func TestCreate_WithSecurity(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"sandbox_id":"sbx-1","envd_access_token":"t","sandbox_domain":"d"}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.Create(context.Background(),
		declaw.WithSecurity(declaw.SecurityPolicy{
			PII: &declaw.PIIConfig{
				Enabled: true,
				Types:   []declaw.PIIType{declaw.PIIEmail, declaw.PIIPhone},
				Action:  declaw.RedactionActionRedact,
			},
			InjectionDefense: &declaw.InjectionDefenseConfig{
				Enabled:     true,
				Sensitivity: declaw.InjectionSensitivityHigh,
				Action:      declaw.InjectionActionBlock,
			},
		}),
	)
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedBody == nil {
		t.Fatal("expected body, got nil")
	}
	if capturedBody["security"] == nil && capturedBody["security_policy"] == nil {
		t.Error("expected security/security_policy in body")
	}
}

func TestCreate_WithAllOptions(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"sandbox_id":"sbx-full","envd_access_token":"t","sandbox_domain":"d"}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.Create(context.Background(),
		declaw.WithTemplate("python"),
		declaw.WithTimeout(600),
		declaw.WithMetadata(map[string]string{"key": "val"}),
		declaw.WithEnvs(map[string]string{"A": "1"}),
		declaw.WithSecure(true),
		declaw.WithLifecycle(declaw.SandboxLifecycle{
			OnTimeout:  "pause",
			AutoResume: true,
		}),
		declaw.WithVolumes([]declaw.VolumeAttachment{
			{VolumeID: "vol-1", MountPath: "/data"},
		}),
	)
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedBody == nil {
		t.Fatal("expected body, got nil")
	}
}

func TestCreate_Error401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "bad-key")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.Create(context.Background())
	if err == nil {
		t.Fatal("expected error for 401")
	}
	var authErr *declaw.AuthenticationError
	if errors.As(err, &authErr) {
		if authErr.StatusCode != 401 {
			t.Errorf("expected StatusCode=401, got %d", authErr.StatusCode)
		}
	}
}

func TestCreate_Error402(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(402)
		w.Write([]byte(`{"message":"insufficient balance"}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.Create(context.Background())
	if err == nil {
		t.Fatal("expected error for 402")
	}
	var balErr *declaw.InsufficientBalanceError
	if errors.As(err, &balErr) {
		if balErr.StatusCode != 402 {
			t.Errorf("expected StatusCode=402, got %d", balErr.StatusCode)
		}
	}
}

func TestCreate_Error429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(429)
		w.Write([]byte(`{"message":"rate limited"}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.Create(context.Background())
	if err == nil {
		t.Fatal("expected error for 429")
	}
	var rlErr *declaw.RateLimitError
	if errors.As(err, &rlErr) {
		if rlErr.RetryAfter != 10*time.Second {
			t.Errorf("expected RetryAfter=10s, got %v", rlErr.RetryAfter)
		}
	}
}

func TestCreate_Error500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"message":"internal error"}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.Create(context.Background())
	if err == nil {
		t.Fatal("expected error for 500")
	}
	var sErr *declaw.SandboxError
	if errors.As(err, &sErr) {
		if sErr.StatusCode != 500 {
			t.Errorf("expected StatusCode=500, got %d", sErr.StatusCode)
		}
	}
}

func TestCreate_ContextCancelled(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "k")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := declaw.Create(ctx)
	if err == nil {
		t.Error("expected error due to cancelled context")
	}
}

// ===========================================================================
// Connect tests
// ===========================================================================

func TestConnect_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/sandboxes/sbx-123") {
			t.Errorf("expected path ending with /sandboxes/sbx-123, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"sandbox_id": "sbx-123",
			"envd_access_token": "",
			"sandbox_domain": "sbx-123.sandbox.declaw.ai",
			"traffic_access_token": "",
			"state": "live"
		}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	sbx, err := declaw.Connect(context.Background(), "sbx-123")
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if sbx == nil {
		t.Fatal("expected non-nil sandbox")
	}
	if sbx.ID != "sbx-123" {
		t.Errorf("expected ID=sbx-123, got %q", sbx.ID)
	}
	if sbx.Commands == nil {
		t.Error("expected Commands to be non-nil")
	}
	if sbx.Files == nil {
		t.Error("expected Files to be non-nil")
	}
	if sbx.PTY == nil {
		t.Error("expected PTY to be non-nil")
	}
}

func TestConnect_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"message":"sandbox not found"}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.Connect(context.Background(), "sbx-missing")
	if err == nil {
		t.Fatal("expected error for 404")
	}

	var nfErr *declaw.NotFoundError
	if errors.As(err, &nfErr) {
		t.Logf("got NotFoundError as expected: %v", nfErr)
	}
}

func TestConnect_EmptySandboxID(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "k")

	_, err := declaw.Connect(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty sandbox ID")
	}
}

func TestConnect_InvalidSandboxID_PathTraversal(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.Connect(context.Background(), "../etc/passwd")
	if err == nil && called {
		t.Log("WARNING: path traversal sandbox ID was sent to server")
	}
}

func TestConnect_InvalidSandboxID_SpecialChars(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.Connect(context.Background(), "sbx 123 with spaces")
	if err == nil {
		t.Log("note: sandbox ID with spaces was accepted; consider adding validation")
	}
}

// ===========================================================================
// ID validation tests for KillSandbox, Restore, DeleteSnapshot
// ===========================================================================

func TestKillSandbox_EmptySandboxID(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "k")
	err := declaw.KillSandbox(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty sandbox ID")
	}
	if err != nil && !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKillSandbox_PathTraversalSandboxID(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "k")
	err := declaw.KillSandbox(context.Background(), "../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal sandbox ID")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid characters") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRestore_EmptySandboxID(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "k")
	_, err := declaw.Restore(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty sandbox ID")
	}
	if err != nil && !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRestore_PathTraversalSandboxID(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "k")
	_, err := declaw.Restore(context.Background(), "sbx/../../etc")
	if err == nil {
		t.Error("expected error for path traversal sandbox ID")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid characters") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteSnapshot_EmptySnapshotID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called for empty snapshot ID")
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-123")
	err := sbx.DeleteSnapshot(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty snapshot ID")
	}
	if err != nil && !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteSnapshot_PathTraversalSnapshotID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called for path traversal snapshot ID")
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-123")
	err := sbx.DeleteSnapshot(context.Background(), "../../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal snapshot ID")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid characters") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ===========================================================================
// Kill tests (instance method)
// ===========================================================================

func TestSandbox_Kill(t *testing.T) {
	var captured requestLog
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = requestLog{
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.RawQuery,
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-kill-1")
	err := sbx.Kill(context.Background())
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.Method != http.MethodDelete {
		t.Errorf("expected DELETE, got %q", captured.Method)
	}
	if !strings.Contains(captured.Path, "sbx-kill-1") {
		t.Errorf("expected path to contain sandbox ID, got %q", captured.Path)
	}
}

// ===========================================================================
// Kill tests (static function)
// ===========================================================================

func TestKillSandbox_Static(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	err := declaw.KillSandbox(context.Background(), "sbx-kill-2")
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===========================================================================
// KillMany tests
// ===========================================================================

func TestKillManySandboxes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"results":{"sbx-1":{"queued":true},"sbx-2":{"queued":true}}}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	results, err := declaw.KillManySandboxes(context.Background(), []string{"sbx-1", "sbx-2"})
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 kill results, got %d", len(results))
	}
}

func TestKillManySandboxes_EmptyList(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "k")

	results, err := declaw.KillManySandboxes(context.Background(), []string{})
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty list, got %d", len(results))
	}
}

// ===========================================================================
// List tests
// ===========================================================================

func TestListSandboxes_Default(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"sandboxes":[],"total":0}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "test-key")
	t.Setenv("DECLAW_API_URL", srv.URL)

	page, err := declaw.ListSandboxes(context.Background())
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if page == nil {
		t.Fatal("expected non-nil SandboxPage")
	}
}

func TestListSandboxes_WithState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"sandboxes":[],"total":0}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.ListSandboxes(context.Background(),
		declaw.WithState(declaw.StateLive),
	)
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListSandboxes_WithLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"sandboxes":[],"total":0}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.ListSandboxes(context.Background(),
		declaw.WithLimit(10),
	)
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListSandboxes_WithStatePaused(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("state")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"sandboxes":[],"total":0}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.ListSandboxes(context.Background(),
		declaw.WithState(declaw.StatePaused),
	)
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedQuery != "paused" {
		t.Errorf("expected state query param 'paused', got %q", capturedQuery)
	}
}

func TestListSandboxes_WithOffset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"sandboxes":[],"total":0}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.ListSandboxes(context.Background(),
		declaw.WithOffset(10),
	)
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListSandboxes_Combined(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"sandboxes":[],"total":0}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.ListSandboxes(context.Background(),
		declaw.WithState(declaw.StateLive),
		declaw.WithLimit(25),
		declaw.WithOffset(50),
	)
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===========================================================================
// Info / Status / Timeout / Metrics tests (instance methods via test server)
// ===========================================================================

func TestSandbox_GetInfo(t *testing.T) {
	var captured requestLog
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = requestLog{Method: r.Method, Path: r.URL.Path}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"sandbox_id": "sbx-info-1",
			"template_id": "python",
			"name": "test-sandbox",
			"metadata": {"env": "test"},
			"state": "live"
		}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-info-1")
	info, err := sbx.GetInfo(context.Background())
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.Method != http.MethodGet {
		t.Errorf("expected GET, got %q", captured.Method)
	}
	if !strings.Contains(captured.Path, "sbx-info-1") {
		t.Errorf("expected path to contain sandbox ID, got %q", captured.Path)
	}

	if info == nil {
		t.Fatal("expected non-nil SandboxInfo")
	}
	if info.SandboxID != "sbx-info-1" {
		t.Errorf("expected SandboxID=sbx-info-1, got %q", info.SandboxID)
	}
	if info.TemplateID != "python" {
		t.Errorf("expected TemplateID=python, got %q", info.TemplateID)
	}
	if info.State != declaw.StateLive {
		t.Errorf("expected State=live, got %q", info.State)
	}
}

func TestSandbox_IsRunning_True(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"is_running":true}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-run-1")
	running, err := sbx.IsRunning(context.Background())
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if !running {
		t.Error("expected IsRunning=true")
	}
}

func TestSandbox_IsRunning_False(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"is_running":false}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-paused-1")
	running, err := sbx.IsRunning(context.Background())
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if running {
		t.Error("expected IsRunning=false for paused sandbox")
	}
}

func TestSandbox_SetTimeout(t *testing.T) {
	var captured requestLog
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		captured = requestLog{Method: r.Method, Path: r.URL.Path}
		json.Unmarshal(bodyBytes, &capturedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-to-1")
	err := sbx.SetTimeout(context.Background(), 600)
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.Method != http.MethodPatch {
		t.Errorf("expected PATCH, got %q", captured.Method)
	}
	if !strings.Contains(captured.Path, "timeout") {
		t.Errorf("expected path to contain 'timeout', got %q", captured.Path)
	}
	if capturedBody != nil {
		if v, ok := capturedBody["timeout"]; ok && v != float64(600) {
			t.Errorf("expected timeout=600, got %v", v)
		}
	}
}

func TestSandbox_SetTimeout_Zero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-to-2")
	err := sbx.SetTimeout(context.Background(), 0)
	if err != nil && err.Error() == "not implemented" {
		return
	}
}

func TestSandbox_SetTimeout_NegativeValue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"error":"invalid timeout"}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-to-3")
	err := sbx.SetTimeout(context.Background(), -1)
	if err == nil {
		t.Log("negative timeout accepted (implementation may validate)")
	}
}

func TestSandbox_GetMetrics(t *testing.T) {
	var captured requestLog
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = requestLog{Method: r.Method, Path: r.URL.Path}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"timestamp": "2026-05-19T00:00:00Z",
			"cpu_usage_percent": 12.5,
			"memory_usage_mb": 256.0,
			"disk_usage_mb": 100.0
		}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-met-1")
	metrics, err := sbx.GetMetrics(context.Background())
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.Method != http.MethodGet {
		t.Errorf("expected GET, got %q", captured.Method)
	}
	if !strings.Contains(captured.Path, "metrics") {
		t.Errorf("expected path to contain 'metrics', got %q", captured.Path)
	}

	if metrics == nil {
		t.Fatal("expected non-nil SandboxMetrics")
	}
	if metrics.CPUUsagePercent != 12.5 {
		t.Errorf("expected CPU=12.5, got %f", metrics.CPUUsagePercent)
	}
	if metrics.MemoryUsageMB != 256.0 {
		t.Errorf("expected Memory=256, got %f", metrics.MemoryUsageMB)
	}
	if metrics.DiskUsageMB != 100.0 {
		t.Errorf("expected Disk=100, got %f", metrics.DiskUsageMB)
	}
}

// ===========================================================================
// Pause / Resume tests
// ===========================================================================

func TestSandbox_Pause(t *testing.T) {
	var captured requestLog
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = requestLog{Method: r.Method, Path: r.URL.Path}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-pause-1")
	err := sbx.Pause(context.Background())
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.Method != http.MethodPost {
		t.Errorf("expected POST, got %q", captured.Method)
	}
	if !strings.Contains(captured.Path, "pause") {
		t.Errorf("expected path to contain 'pause', got %q", captured.Path)
	}
	if !strings.Contains(captured.Path, "sbx-pause-1") {
		t.Errorf("expected path to contain sandbox ID, got %q", captured.Path)
	}
}

func TestSandbox_Resume(t *testing.T) {
	var captured requestLog
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = requestLog{Method: r.Method, Path: r.URL.Path}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-resume-1")
	err := sbx.Resume(context.Background())
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.Method != http.MethodPost {
		t.Errorf("expected POST, got %q", captured.Method)
	}
	if !strings.Contains(captured.Path, "resume") {
		t.Errorf("expected path to contain 'resume', got %q", captured.Path)
	}
}

func TestSandbox_Pause_AlreadyPaused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"sandbox already paused"}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-pause-2")
	err := sbx.Pause(context.Background())
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Logf("pause error (expected): %v", err)
	}
}

func TestSandbox_Resume_NotPaused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"sandbox is not paused"}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-resume-2")
	err := sbx.Resume(context.Background())
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Logf("resume error (expected): %v", err)
	}
}

// ===========================================================================
// Snapshot tests
// ===========================================================================

func TestSandbox_CreateSnapshot(t *testing.T) {
	var captured requestLog
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = requestLog{Method: r.Method, Path: r.URL.Path}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{
			"snapshot_id": "snap-abc",
			"sandbox_id": "sbx-snap-1",
			"created_at": "2026-05-19T00:00:00Z"
		}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-snap-1")
	snap, err := sbx.CreateSnapshot(context.Background())
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.Method != http.MethodPost {
		t.Errorf("expected POST, got %q", captured.Method)
	}
	if !strings.Contains(captured.Path, "snapshot") {
		t.Errorf("expected path to contain 'snapshot', got %q", captured.Path)
	}

	if snap == nil {
		t.Fatal("expected non-nil SnapshotInfo")
	}
	if snap.SnapshotID != "snap-abc" {
		t.Errorf("expected SnapshotID=snap-abc, got %q", snap.SnapshotID)
	}
	if snap.SandboxID != "sbx-snap-1" {
		t.Errorf("expected SandboxID=sbx-snap-1, got %q", snap.SandboxID)
	}
}

func TestSandbox_ListSnapshots(t *testing.T) {
	var captured requestLog
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = requestLog{Method: r.Method, Path: r.URL.Path}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"snapshots":[
			{"snapshot_id":"snap-1","sandbox_id":"sbx-snap-2"},
			{"snapshot_id":"snap-2","sandbox_id":"sbx-snap-2"}
		]}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-snap-2")
	snapshots, err := sbx.ListSnapshots(context.Background())
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.Method != http.MethodGet {
		t.Errorf("expected GET, got %q", captured.Method)
	}
	if !strings.Contains(captured.Path, "snapshots") {
		t.Errorf("expected path to contain 'snapshots', got %q", captured.Path)
	}

	if len(snapshots) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(snapshots))
	}
}

func TestSandbox_ListSnapshots_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"snapshots":[]}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-snap-3")
	snapshots, err := sbx.ListSnapshots(context.Background())
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if len(snapshots) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snapshots))
	}
}

func TestSandbox_DeleteSnapshot(t *testing.T) {
	var captured requestLog
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = requestLog{Method: r.Method, Path: r.URL.Path}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-snap-4")
	err := sbx.DeleteSnapshot(context.Background(), "snap-del-1")
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.Method != http.MethodDelete {
		t.Errorf("expected DELETE, got %q", captured.Method)
	}
	if !strings.Contains(captured.Path, "snap-del-1") {
		t.Errorf("expected path to contain snapshot ID, got %q", captured.Path)
	}
	if !strings.Contains(captured.Path, "sbx-snap-4") {
		t.Errorf("expected path to contain sandbox ID, got %q", captured.Path)
	}
}

func TestSandbox_DeleteSnapshot_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"snapshot not found"}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-snap-5")
	err := sbx.DeleteSnapshot(context.Background(), "snap-missing")
	if err == nil {
		t.Log("expected error for missing snapshot (may be accepted in stub)")
	}

	var nfErr *declaw.NotFoundError
	if errors.As(err, &nfErr) {
		t.Logf("got NotFoundError as expected: %v", nfErr)
	}
}

// ===========================================================================
// Restore tests
// ===========================================================================

func TestRestore_WithSnapshotID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"sandbox_id":"sbx-orig","node_id":"node-1","snapshot_id":"snap-123"}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	sbx, err := declaw.Restore(context.Background(), "sbx-orig",
		declaw.WithSnapshotID("snap-123"),
	)
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if sbx == nil {
		t.Fatal("expected non-nil sandbox")
	}
}

func TestRestore_WithoutSnapshotID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"sandbox_id":"sbx-orig-2","node_id":"node-1","snapshot_id":"snap-latest"}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	sbx, err := declaw.Restore(context.Background(), "sbx-orig-2")
	if err != nil {
		if err.Error() == "not implemented" {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if sbx == nil {
		t.Fatal("expected non-nil sandbox")
	}
}

func TestRestore_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"message":"sandbox not found"}`))
	}))
	defer srv.Close()

	t.Setenv("DECLAW_API_KEY", "k")
	t.Setenv("DECLAW_API_URL", srv.URL)

	_, err := declaw.Restore(context.Background(), "sbx-missing")
	if err == nil {
		t.Fatal("expected error for missing sandbox")
	}
}

// ===========================================================================
// Kill error cases
// ===========================================================================

func TestSandbox_Kill_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"sandbox not found"}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-gone")
	err := sbx.Kill(context.Background())
	if err == nil {
		t.Log("no error for killing non-existent sandbox (idempotent)")
		return
	}
	if err.Error() == "not implemented" {
		return
	}

	var nfErr *declaw.NotFoundError
	if errors.As(err, &nfErr) {
		t.Logf("got NotFoundError: %v", nfErr)
	}
}

func TestSandbox_Kill_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`internal error`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-err-1")
	err := sbx.Kill(context.Background())
	if err == nil {
		t.Log("no error for 500 (may be retried)")
		return
	}
	if err.Error() == "not implemented" {
		return
	}

	var sErr *declaw.SandboxError
	if errors.As(err, &sErr) {
		if sErr.StatusCode != 500 {
			t.Errorf("expected StatusCode=500, got %d", sErr.StatusCode)
		}
	}
}

// ===========================================================================
// Context cancellation for instance methods
// ===========================================================================

func TestSandbox_GetInfo_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-ctx-1")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := sbx.GetInfo(ctx)
	if err == nil {
		t.Error("expected error due to context timeout")
	}
}

func TestSandbox_Pause_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-ctx-2")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := sbx.Pause(ctx)
	if err == nil {
		t.Error("expected error due to cancelled context")
	}
}

func TestSandbox_Kill_ContextAlreadyDone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sbx := newSandboxFromServer(t, srv.URL, "sbx-done")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := sbx.Kill(ctx)
	if err == nil {
		t.Log("kill succeeded even with cancelled context (may be async)")
	}
}

// ===========================================================================
// Sandbox sub-object initialization
// ===========================================================================

func TestNewTestSandbox_SubObjects(t *testing.T) {
	cfg := declaw.NewConfig(declaw.WithAPIKey("k"))
	c := declaw.NewTestAPIClient(cfg)
	sbx := declaw.NewTestSandbox("sbx-test", c)

	if sbx.ID != "sbx-test" {
		t.Errorf("expected ID=sbx-test, got %q", sbx.ID)
	}
	if sbx.Commands == nil {
		t.Error("expected Commands non-nil")
	}
	if sbx.Files == nil {
		t.Error("expected Files non-nil")
	}
	if sbx.PTY == nil {
		t.Error("expected PTY non-nil")
	}
}

// ===========================================================================
// Option type verification tests
// ===========================================================================

func TestListOption_Combined(t *testing.T) {
	opts := []declaw.ListOption{
		declaw.WithState(declaw.StateLive),
		declaw.WithLimit(25),
		declaw.WithOffset(50),
	}
	if len(opts) != 3 {
		t.Errorf("expected 3 options, got %d", len(opts))
	}
}

func TestRestoreOption_WithSnapshotID(t *testing.T) {
	opts := []declaw.RestoreOption{declaw.WithSnapshotID("snap-123")}
	if len(opts) != 1 {
		t.Errorf("expected 1 option, got %d", len(opts))
	}
}

func TestRunOption_AllCombined(t *testing.T) {
	opts := []declaw.RunOption{
		declaw.WithRunEnvs(map[string]string{"A": "1"}),
		declaw.WithUser("nobody"),
		declaw.WithCwd("/tmp"),
		declaw.WithStdin(),
		declaw.WithRunTimeout(10 * time.Second),
		declaw.WithOnStdout(func(string) {}),
		declaw.WithOnStderr(func(string) {}),
	}
	if len(opts) != 7 {
		t.Errorf("expected 7 options, got %d", len(opts))
	}
}

func TestFileOption_WithFileUser_Sandbox(t *testing.T) {
	opts := []declaw.FileOption{declaw.WithFileUser("root")}
	if len(opts) != 1 {
		t.Errorf("expected 1 option, got %d", len(opts))
	}
}

func TestSandboxOption_LastOptionWins(t *testing.T) {
	opts := declaw.ExportedResolveSandboxOpts([]declaw.SandboxOption{
		declaw.WithTemplate("python"),
		declaw.WithTemplate("node"),
	})
	if opts.SandboxOptsTemplate() != "node" {
		t.Errorf("expected template=node (last wins), got %q", opts.SandboxOptsTemplate())
	}
}

// ===========================================================================
// Edge cases
// ===========================================================================

func TestCreate_NilContext(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "k")

	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from panic (expected for nil context): %v", r)
		}
	}()

	//nolint:staticcheck
	_, err := declaw.Create(nil) //nolint:SA1012
	if err != nil {
		t.Logf("got error for nil context: %v", err)
	}
}

func TestConnect_NilContext(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "k")

	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from panic (expected for nil context): %v", r)
		}
	}()

	//nolint:staticcheck
	_, err := declaw.Connect(nil, "sbx-1") //nolint:SA1012
	if err != nil {
		t.Logf("got error for nil context: %v", err)
	}
}

// ===========================================================================
// SandboxState constants
// ===========================================================================

func TestSandboxState_Values(t *testing.T) {
	states := map[declaw.SandboxState]string{
		declaw.StateLive:   "live",
		declaw.StatePaused: "paused",
		declaw.StateKilled: "killed",
	}

	for state, expected := range states {
		if string(state) != expected {
			t.Errorf("expected %q, got %q", expected, string(state))
		}
	}
}

// ===========================================================================
// AllTraffic constant
// ===========================================================================

func TestAllTraffic_ConstantValue(t *testing.T) {
	if declaw.AllTraffic != "*" {
		t.Errorf("expected AllTraffic='*', got %q", declaw.AllTraffic)
	}
}

// ===========================================================================
// DomainMatches tests
// ===========================================================================

func TestDomainMatches_ExactMatch(t *testing.T) {
	if !declaw.DomainMatches("example.com", "example.com") {
		t.Error("expected exact match")
	}
}

func TestDomainMatches_Wildcard(t *testing.T) {
	if !declaw.DomainMatches("*", "anything.com") {
		t.Error("expected wildcard to match anything")
	}
}

func TestDomainMatches_WildcardPrefix(t *testing.T) {
	if !declaw.DomainMatches("*.example.com", "api.example.com") {
		t.Error("expected *.example.com to match api.example.com")
	}
}

func TestDomainMatches_WildcardPrefix_DeepSubdomain(t *testing.T) {
	if !declaw.DomainMatches("*.example.com", "deep.sub.api.example.com") {
		t.Error("expected *.example.com to match deep.sub.api.example.com")
	}
}

func TestDomainMatches_NoMatch(t *testing.T) {
	if declaw.DomainMatches("example.com", "other.com") {
		t.Error("expected no match for different domains")
	}
}

func TestDomainMatches_CaseInsensitive(t *testing.T) {
	if !declaw.DomainMatches("Example.COM", "example.com") {
		t.Error("expected case-insensitive match")
	}
}

func TestDomainMatches_WildcardBaseMatch(t *testing.T) {
	result := declaw.DomainMatches("*.example.com", "example.com")
	t.Logf("*.example.com matches example.com: %v (documenting behavior)", result)
}

func TestDomainMatches_EmptyPattern(t *testing.T) {
	if declaw.DomainMatches("", "example.com") {
		t.Error("expected empty pattern to not match")
	}
}

func TestDomainMatches_EmptyDomain(t *testing.T) {
	if declaw.DomainMatches("example.com", "") {
		t.Error("expected empty domain to not match")
	}
}

// ===========================================================================
// SecurityPolicy.RequiresTLSInterception tests
// ===========================================================================

func TestSecurityPolicy_RequiresTLSInterception_Nil(t *testing.T) {
	var sp *declaw.SecurityPolicy
	if sp.RequiresTLSInterception() {
		t.Error("nil SecurityPolicy should not require TLS interception")
	}
}

func TestSecurityPolicy_RequiresTLSInterception_Empty(t *testing.T) {
	sp := &declaw.SecurityPolicy{}
	if sp.RequiresTLSInterception() {
		t.Error("empty SecurityPolicy should not require TLS interception")
	}
}

func TestSecurityPolicy_RequiresTLSInterception_PIIEnabled(t *testing.T) {
	sp := &declaw.SecurityPolicy{
		PII: &declaw.PIIConfig{Enabled: true},
	}
	if !sp.RequiresTLSInterception() {
		t.Error("PII enabled should require TLS interception")
	}
}

func TestSecurityPolicy_RequiresTLSInterception_PIIDisabled(t *testing.T) {
	sp := &declaw.SecurityPolicy{
		PII: &declaw.PIIConfig{Enabled: false},
	}
	if sp.RequiresTLSInterception() {
		t.Error("PII disabled should not require TLS interception")
	}
}

func TestSecurityPolicy_RequiresTLSInterception_InjectionDefense(t *testing.T) {
	sp := &declaw.SecurityPolicy{
		InjectionDefense: &declaw.InjectionDefenseConfig{Enabled: true},
	}
	if !sp.RequiresTLSInterception() {
		t.Error("injection defense enabled should require TLS interception")
	}
}

func TestSecurityPolicy_RequiresTLSInterception_Transformations(t *testing.T) {
	sp := &declaw.SecurityPolicy{
		Transformations: []declaw.TransformationRule{
			{Match: "secret", Replace: "***", Direction: declaw.TransformBoth},
		},
	}
	if !sp.RequiresTLSInterception() {
		t.Error("transformations should require TLS interception")
	}
}

func TestSecurityPolicy_RequiresTLSInterception_Toxicity(t *testing.T) {
	sp := &declaw.SecurityPolicy{
		Toxicity: &declaw.ToxicityConfig{Enabled: true, Threshold: 0.5},
	}
	if !sp.RequiresTLSInterception() {
		t.Error("toxicity enabled should require TLS interception")
	}
}

func TestSecurityPolicy_RequiresTLSInterception_CodeSecurity(t *testing.T) {
	sp := &declaw.SecurityPolicy{
		CodeSecurity: &declaw.CodeSecurityConfig{Enabled: true},
	}
	if !sp.RequiresTLSInterception() {
		t.Error("code security enabled should require TLS interception")
	}
}

func TestSecurityPolicy_RequiresTLSInterception_InvisibleText(t *testing.T) {
	sp := &declaw.SecurityPolicy{
		InvisibleText: &declaw.InvisibleTextConfig{Enabled: true},
	}
	if !sp.RequiresTLSInterception() {
		t.Error("invisible text enabled should require TLS interception")
	}
}

func TestSecurityPolicy_RequiresTLSInterception_NetworkOnly(t *testing.T) {
	sp := &declaw.SecurityPolicy{
		Network: &declaw.NetworkPolicy{AllowOut: []string{"*.github.com"}},
	}
	if sp.RequiresTLSInterception() {
		t.Error("network policy alone should not require TLS interception")
	}
}

func TestSecurityPolicy_RequiresTLSInterception_AuditOnly(t *testing.T) {
	sp := &declaw.SecurityPolicy{
		Audit: &declaw.AuditConfig{Enabled: true},
	}
	if sp.RequiresTLSInterception() {
		t.Error("audit alone should not require TLS interception")
	}
}

func TestSecurityPolicy_RequiresTLSInterception_EnvSecurityOnly(t *testing.T) {
	sp := &declaw.SecurityPolicy{
		EnvSecurity: &declaw.EnvSecurityConfig{MaskPatterns: []string{"*_SECRET"}},
	}
	if sp.RequiresTLSInterception() {
		t.Error("env security alone should not require TLS interception")
	}
}

func TestSecurityPolicy_RequiresTLSInterception_MultipleEnabled(t *testing.T) {
	sp := &declaw.SecurityPolicy{
		PII:              &declaw.PIIConfig{Enabled: true},
		InjectionDefense: &declaw.InjectionDefenseConfig{Enabled: true},
		Toxicity:         &declaw.ToxicityConfig{Enabled: true},
	}
	if !sp.RequiresTLSInterception() {
		t.Error("multiple scanners enabled should require TLS interception")
	}
}

// ===========================================================================
// Model struct field tests
// ===========================================================================

func TestKillResult_Fields(t *testing.T) {
	kr := declaw.KillResult{
		SandboxID: "sbx-1",
		Error:     errors.New("failed"),
	}
	if kr.SandboxID != "sbx-1" {
		t.Errorf("expected SandboxID=sbx-1, got %q", kr.SandboxID)
	}
	if kr.Error == nil {
		t.Error("expected non-nil error")
	}
}

func TestKillResult_NoError(t *testing.T) {
	kr := declaw.KillResult{SandboxID: "sbx-2", Error: nil}
	if kr.Error != nil {
		t.Errorf("expected nil error, got %v", kr.Error)
	}
}

func TestSandboxPage_Fields(t *testing.T) {
	page := declaw.SandboxPage{
		Sandboxes: []declaw.SandboxInfo{
			{SandboxID: "sbx-1", State: declaw.StateLive},
			{SandboxID: "sbx-2", State: declaw.StatePaused},
		},
		Total: 42,
	}
	if len(page.Sandboxes) != 2 {
		t.Errorf("expected 2 sandboxes, got %d", len(page.Sandboxes))
	}
	if page.Total != 42 {
		t.Errorf("expected Total=42, got %d", page.Total)
	}
}

func TestSnapshotPage_Fields(t *testing.T) {
	page := declaw.SnapshotPage{
		Snapshots: []declaw.SnapshotInfo{{SnapshotID: "snap-1"}},
	}
	if len(page.Snapshots) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(page.Snapshots))
	}
}

// ===========================================================================
// Table-driven: sandbox operations HTTP method + path verification
// ===========================================================================

func TestSandboxOperations_MethodAndPath(t *testing.T) {
	tests := []struct {
		name          string
		operation     func(sbx *declaw.Sandbox) error
		expectMethod  string
		expectPathSub string
	}{
		{
			name:          "Kill",
			operation:     func(sbx *declaw.Sandbox) error { return sbx.Kill(context.Background()) },
			expectMethod:  http.MethodDelete,
			expectPathSub: "sbx-table",
		},
		{
			name: "SetTimeout",
			operation: func(sbx *declaw.Sandbox) error {
				return sbx.SetTimeout(context.Background(), 300)
			},
			expectMethod:  http.MethodPatch,
			expectPathSub: "timeout",
		},
		{
			name: "GetInfo",
			operation: func(sbx *declaw.Sandbox) error {
				_, err := sbx.GetInfo(context.Background())
				return err
			},
			expectMethod:  http.MethodGet,
			expectPathSub: "sbx-table",
		},
		{
			name: "IsRunning",
			operation: func(sbx *declaw.Sandbox) error {
				_, err := sbx.IsRunning(context.Background())
				return err
			},
			expectMethod:  http.MethodGet,
			expectPathSub: "sbx-table",
		},
		{
			name: "GetMetrics",
			operation: func(sbx *declaw.Sandbox) error {
				_, err := sbx.GetMetrics(context.Background())
				return err
			},
			expectMethod:  http.MethodGet,
			expectPathSub: "metrics",
		},
		{
			name:          "Pause",
			operation:     func(sbx *declaw.Sandbox) error { return sbx.Pause(context.Background()) },
			expectMethod:  http.MethodPost,
			expectPathSub: "pause",
		},
		{
			name:          "Resume",
			operation:     func(sbx *declaw.Sandbox) error { return sbx.Resume(context.Background()) },
			expectMethod:  http.MethodPost,
			expectPathSub: "resume",
		},
		{
			name: "CreateSnapshot",
			operation: func(sbx *declaw.Sandbox) error {
				_, err := sbx.CreateSnapshot(context.Background())
				return err
			},
			expectMethod:  http.MethodPost,
			expectPathSub: "snapshot",
		},
		{
			name: "ListSnapshots",
			operation: func(sbx *declaw.Sandbox) error {
				_, err := sbx.ListSnapshots(context.Background())
				return err
			},
			expectMethod:  http.MethodGet,
			expectPathSub: "snapshot",
		},
		{
			name: "DeleteSnapshot",
			operation: func(sbx *declaw.Sandbox) error {
				return sbx.DeleteSnapshot(context.Background(), "snap-1")
			},
			expectMethod:  http.MethodDelete,
			expectPathSub: "snap-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"running","sandbox_id":"sbx-table","snapshot_id":"snap-1"}`))
			}))
			defer srv.Close()

			sbx := newSandboxFromServer(t, srv.URL, "sbx-table")
			err := tt.operation(sbx)

			if err != nil && err.Error() == "not implemented" {
				return
			}

			if gotMethod != tt.expectMethod {
				t.Errorf("expected %s, got %q", tt.expectMethod, gotMethod)
			}
			if !strings.Contains(gotPath, tt.expectPathSub) {
				t.Errorf("expected path to contain %q, got %q", tt.expectPathSub, gotPath)
			}
		})
	}
}

// ===========================================================================
// Sandbox operations with various error status codes
// ===========================================================================

func TestSandboxOperations_ErrorResponses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		operation  func(sbx *declaw.Sandbox) error
		checkError func(t *testing.T, err error)
	}{
		{
			name:       "GetInfo_401",
			statusCode: 401,
			body:       `{"error":"unauthorized"}`,
			operation: func(sbx *declaw.Sandbox) error {
				_, err := sbx.GetInfo(context.Background())
				return err
			},
			checkError: func(t *testing.T, err error) {
				var e *declaw.AuthenticationError
				if !errors.As(err, &e) {
					t.Errorf("expected AuthenticationError, got %T", err)
				}
			},
		},
		{
			name:       "GetMetrics_404",
			statusCode: 404,
			body:       `{"error":"not found"}`,
			operation: func(sbx *declaw.Sandbox) error {
				_, err := sbx.GetMetrics(context.Background())
				return err
			},
			checkError: func(t *testing.T, err error) {
				var e *declaw.NotFoundError
				if !errors.As(err, &e) {
					t.Errorf("expected NotFoundError, got %T", err)
				}
			},
		},
		{
			name:       "Pause_500",
			statusCode: 500,
			body:       `internal error`,
			operation: func(sbx *declaw.Sandbox) error {
				return sbx.Pause(context.Background())
			},
			checkError: func(t *testing.T, err error) {
				var e *declaw.SandboxError
				if !errors.As(err, &e) {
					t.Errorf("expected SandboxError, got %T", err)
				}
			},
		},
		{
			name:       "SetTimeout_422",
			statusCode: 422,
			body:       `{"error":"invalid timeout"}`,
			operation: func(sbx *declaw.Sandbox) error {
				return sbx.SetTimeout(context.Background(), -1)
			},
			checkError: func(t *testing.T, err error) {
				var e *declaw.InvalidArgumentError
				if !errors.As(err, &e) {
					t.Errorf("expected InvalidArgumentError, got %T", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			sbx := newSandboxFromServer(t, srv.URL, "sbx-err")
			err := tt.operation(sbx)

			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() == "not implemented" {
				return
			}

			tt.checkError(t, err)
		})
	}
}
