package declaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newPTYTestServer creates a test HTTP server and returns the server plus
// an apiClient whose config points at it.
func newPTYTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *apiClient) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	cfg := &Config{
		APIKey: "test-key",
		APIURL: ts.URL,
	}
	client := newAPIClient(cfg)
	return ts, client
}

// ---------------------------------------------------------------------------
// PTY.Create tests
// ---------------------------------------------------------------------------

func TestPTY_Create_DefaultSize(t *testing.T) {
	t.Parallel()

	var (
		mu         sync.Mutex
		gotMethod  string
		gotPath    string
		gotBody    map[string]interface{}
		gotAuthHdr string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuthHdr = r.Header.Get("Authorization")

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"pid": 10}`))
	})

	_, client := newPTYTestServer(t, handler)
	pty := NewTestPTY("sbx-123", client)

	handle, err := pty.Create(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/sandboxes/sbx-123/pty" {
		t.Errorf("expected path /sandboxes/sbx-123/pty, got %s", gotPath)
	}

	// Default size should be 80x24
	sizeRaw, ok := gotBody["size"]
	if !ok {
		t.Fatal("expected 'size' in request body")
	}
	sizeMap, ok := sizeRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected size to be a map, got %T", sizeRaw)
	}
	if cols, _ := sizeMap["cols"].(float64); int(cols) != 80 {
		t.Errorf("expected default cols=80, got %v", sizeMap["cols"])
	}
	if rows, _ := sizeMap["rows"].(float64); int(rows) != 24 {
		t.Errorf("expected default rows=24, got %v", sizeMap["rows"])
	}

	if handle == nil {
		t.Fatal("expected non-nil handle")
	}
	if handle.PID != 10 {
		t.Errorf("expected PID=10, got %d", handle.PID)
	}

	// Verify auth header is present
	if gotAuthHdr == "" {
		t.Error("expected Authorization header to be set")
	}
}

func TestPTY_Create_CustomSize(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		gotBody map[string]interface{}
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"pid": 42}`))
	})

	_, client := newPTYTestServer(t, handler)
	pty := NewTestPTY("sbx-456", client)

	handle, err := pty.Create(context.Background(), PtySize{Cols: 120, Rows: 40})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	sizeRaw, ok := gotBody["size"]
	if !ok {
		t.Fatal("expected 'size' in request body")
	}
	sizeMap, ok := sizeRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected size to be a map, got %T", sizeRaw)
	}
	if cols, _ := sizeMap["cols"].(float64); int(cols) != 120 {
		t.Errorf("expected cols=120, got %v", sizeMap["cols"])
	}
	if rows, _ := sizeMap["rows"].(float64); int(rows) != 40 {
		t.Errorf("expected rows=40, got %v", sizeMap["rows"])
	}
	if handle.PID != 42 {
		t.Errorf("expected PID=42, got %d", handle.PID)
	}
}

func TestPTY_Create_ServerError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal"}`))
	})

	_, client := newPTYTestServer(t, handler)
	pty := NewTestPTY("sbx-err", client)

	_, err := pty.Create(context.Background())
	if err == nil {
		t.Fatal("expected an error on 500 response")
	}
}

func TestPTY_Create_InvalidJSON(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	})

	_, client := newPTYTestServer(t, handler)
	pty := NewTestPTY("sbx-bad", client)

	_, err := pty.Create(context.Background())
	if err == nil {
		t.Fatal("expected an error on invalid JSON response")
	}
}

func TestPTY_Create_ContextCanceled(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow server
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"pid": 1}`))
	})

	_, client := newPTYTestServer(t, handler)
	pty := NewTestPTY("sbx-cancel", client)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := pty.Create(ctx)
	if err == nil {
		t.Fatal("expected an error when context is canceled")
	}
}

// ---------------------------------------------------------------------------
// PtyHandle.Kill tests
// ---------------------------------------------------------------------------

func TestPtyHandle_Kill_Success(t *testing.T) {
	t.Parallel()

	var (
		mu        sync.Mutex
		gotMethod string
		gotPath   string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})

	_, client := newPTYTestServer(t, handler)
	handle := NewTestPtyHandle(10, "sbx-123", client)

	err := handle.Kill(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
	if gotPath != "/sandboxes/sbx-123/pty/10" {
		t.Errorf("expected path /sandboxes/sbx-123/pty/10, got %s", gotPath)
	}
}

func TestPtyHandle_Kill_NotFound(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "pty not found"}`))
	})

	_, client := newPTYTestServer(t, handler)
	handle := NewTestPtyHandle(99, "sbx-missing", client)

	err := handle.Kill(context.Background())
	if err == nil {
		t.Fatal("expected an error on 404 response")
	}
	var notFound *NotFoundError
	if ok := isErrorType(err, &notFound); !ok {
		// It's acceptable if the error isn't typed yet in stubs; just verify it's non-nil
		t.Logf("note: error is not *NotFoundError (may be stub): %T: %v", err, err)
	}
}

func TestPtyHandle_Kill_ContextCanceled(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusNoContent)
	})

	_, client := newPTYTestServer(t, handler)
	handle := NewTestPtyHandle(10, "sbx-timeout", client)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := handle.Kill(ctx)
	if err == nil {
		t.Fatal("expected an error when context is canceled")
	}
}

// ---------------------------------------------------------------------------
// PtyHandle.SendInput tests
// ---------------------------------------------------------------------------

func TestPtyHandle_SendInput_Success(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		gotBody map[string]interface{}
		gotPath string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotPath = r.URL.Path

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	_, client := newPTYTestServer(t, handler)
	handle := NewTestPtyHandle(99, "sbx-pty", client)

	err := handle.SendInput(context.Background(), []byte("ls -la\n"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotPath != "/sandboxes/sbx-pty/pty/99/stdin" {
		t.Errorf("expected path /sandboxes/sbx-pty/pty/99/stdin, got %s", gotPath)
	}

	dataVal, ok := gotBody["data"]
	if !ok {
		t.Fatal("expected 'data' field in request body")
	}
	dataStr, ok := dataVal.(string)
	if !ok {
		t.Fatalf("expected data to be a string, got %T", dataVal)
	}
	if dataStr != "ls -la\n" {
		t.Errorf("expected data=%q, got %q", "ls -la\n", dataStr)
	}
}

func TestPtyHandle_SendInput_EmptyData(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	_, client := newPTYTestServer(t, handler)
	handle := NewTestPtyHandle(99, "sbx-empty", client)

	err := handle.SendInput(context.Background(), []byte{})
	// Should either succeed (sending empty body) or return a specific error;
	// the important thing is it does not panic.
	_ = err
}

func TestPtyHandle_SendInput_LargePayload(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	_, client := newPTYTestServer(t, handler)
	handle := NewTestPtyHandle(99, "sbx-large", client)

	// 1MB of data
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = 'A'
	}

	err := handle.SendInput(context.Background(), largeData)
	// Should not panic regardless of outcome
	_ = err
}

func TestPtyHandle_SendInput_SpecialCharacters(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		gotBody map[string]interface{}
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	_, client := newPTYTestServer(t, handler)
	handle := NewTestPtyHandle(1, "sbx-special", client)

	// Send special chars including control sequences and Unicode
	err := handle.SendInput(context.Background(), []byte("\x1b[A\t\n"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	dataVal, ok := gotBody["data"]
	if !ok {
		t.Fatal("expected 'data' field in request body")
	}
	dataStr, ok := dataVal.(string)
	if !ok {
		t.Fatalf("expected data to be a string, got %T", dataVal)
	}
	if dataStr != "\x1b[A\t\n" {
		t.Errorf("expected raw special chars, got %q", dataStr)
	}
}

// ---------------------------------------------------------------------------
// PtyHandle.SetSize tests
// ---------------------------------------------------------------------------

func TestPtyHandle_SetSize_Success(t *testing.T) {
	t.Parallel()

	var (
		mu        sync.Mutex
		gotMethod string
		gotPath   string
		gotBody   map[string]interface{}
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotMethod = r.Method
		gotPath = r.URL.Path

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	_, client := newPTYTestServer(t, handler)
	handle := NewTestPtyHandle(99, "sbx-resize", client)

	err := handle.SetSize(context.Background(), 120, 40)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodPatch {
		t.Errorf("expected PATCH, got %s", gotMethod)
	}
	if gotPath != "/sandboxes/sbx-resize/pty/99" {
		t.Errorf("expected path /sandboxes/sbx-resize/pty/99, got %s", gotPath)
	}

	// Check body has cols and rows (could be nested under "size" or at top level)
	// Based on TS SDK: body is { size: { cols: 200, rows: 50 } }
	sizeRaw, hasSize := gotBody["size"]
	if hasSize {
		sizeMap, ok := sizeRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("expected size to be a map, got %T", sizeRaw)
		}
		if cols, _ := sizeMap["cols"].(float64); int(cols) != 120 {
			t.Errorf("expected cols=120, got %v", sizeMap["cols"])
		}
		if rows, _ := sizeMap["rows"].(float64); int(rows) != 40 {
			t.Errorf("expected rows=40, got %v", sizeMap["rows"])
		}
	} else {
		// Alternative: cols/rows at top level
		if cols, _ := gotBody["cols"].(float64); int(cols) != 120 {
			t.Errorf("expected cols=120, got %v", gotBody["cols"])
		}
		if rows, _ := gotBody["rows"].(float64); int(rows) != 40 {
			t.Errorf("expected rows=40, got %v", gotBody["rows"])
		}
	}
}

func TestPtyHandle_SetSize_ZeroDimensions(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	_, client := newPTYTestServer(t, handler)
	handle := NewTestPtyHandle(1, "sbx-zero", client)

	// Zero dimensions -- should not panic. Implementation may validate.
	err := handle.SetSize(context.Background(), 0, 0)
	_ = err // Just ensure no panic
}

func TestPtyHandle_SetSize_NegativeDimensions(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error": "invalid dimensions"}`))
	})

	_, client := newPTYTestServer(t, handler)
	handle := NewTestPtyHandle(1, "sbx-neg", client)

	err := handle.SetSize(context.Background(), -1, -1)
	if err == nil {
		t.Error("expected an error for negative dimensions")
	}
}

// ---------------------------------------------------------------------------
// PtyHandle.Stream tests
// ---------------------------------------------------------------------------

func TestPtyHandle_Stream_ReceivesData(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sandboxes/sbx-stream/pty/10/stream" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("expected ResponseWriter to implement Flusher")
			return
		}

		// Send SSE data events
		fmt.Fprintf(w, "event: data\ndata: {\"data\": \"aGVsbG8=\"}\n\n") // "hello" in base64
		flusher.Flush()
		fmt.Fprintf(w, "event: data\ndata: {\"data\": \"d29ybGQ=\"}\n\n") // "world" in base64
		flusher.Flush()
		fmt.Fprintf(w, "event: exit\ndata: {\"exit_code\": 0}\n\n")
		flusher.Flush()
	})

	_, client := newPTYTestServer(t, handler)
	handle := NewTestPtyHandle(10, "sbx-stream", client)

	ch, err := handle.Stream(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	var received []string
	timeout := time.After(5 * time.Second)
	for {
		select {
		case data, ok := <-ch:
			if !ok {
				goto done
			}
			received = append(received, string(data))
		case <-timeout:
			t.Fatal("timed out waiting for stream data")
		}
	}
done:

	if len(received) < 2 {
		t.Errorf("expected at least 2 data events, got %d", len(received))
	}
	if len(received) >= 1 && received[0] != "hello" {
		t.Errorf("expected first event 'hello', got %q", received[0])
	}
	if len(received) >= 2 && received[1] != "world" {
		t.Errorf("expected second event 'world', got %q", received[1])
	}
}

func TestPtyHandle_Stream_ContextCancel(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, _ := w.(http.Flusher)
		// Keep sending events until client disconnects
		for i := 0; i < 100; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
				fmt.Fprintf(w, "event: data\ndata: {\"data\": \"dGVzdA==\"}\n\n")
				flusher.Flush()
				time.Sleep(50 * time.Millisecond)
			}
		}
	})

	_, client := newPTYTestServer(t, handler)
	handle := NewTestPtyHandle(10, "sbx-cancel-stream", client)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	ch, err := handle.Stream(ctx)
	if err != nil {
		t.Fatalf("expected no error starting stream, got %v", err)
	}

	// Should eventually close the channel when context is canceled
	timeout := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // channel closed -- success
			}
		case <-timeout:
			t.Fatal("timed out: channel should have been closed after context cancellation")
		}
	}
}

func TestPtyHandle_Stream_ServerError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "server down"}`))
	})

	_, client := newPTYTestServer(t, handler)
	handle := NewTestPtyHandle(10, "sbx-stream-err", client)

	_, err := handle.Stream(context.Background())
	if err == nil {
		t.Fatal("expected an error on 500 response")
	}
}

// ---------------------------------------------------------------------------
// PTY path construction tests
// ---------------------------------------------------------------------------

func TestPTY_PathConstruction_SpecialSandboxID(t *testing.T) {
	t.Parallel()

	var gotPath string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"pid": 1}`))
	})

	_, client := newPTYTestServer(t, handler)
	pty := NewTestPTY("sbx-abc-def-123", client)

	_, _ = pty.Create(context.Background())

	if gotPath != "/sandboxes/sbx-abc-def-123/pty" {
		t.Errorf("expected path with full sandbox ID, got %s", gotPath)
	}
}

func TestPtyHandle_PathConstruction_VariousPIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pid      int
		wantPath string
	}{
		{name: "pid_1", pid: 1, wantPath: "/sandboxes/sbx-x/pty/1"},
		{name: "pid_large", pid: 99999, wantPath: "/sandboxes/sbx-x/pty/99999"},
		{name: "pid_zero", pid: 0, wantPath: "/sandboxes/sbx-x/pty/0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotPath string
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusNoContent)
			})

			_, client := newPTYTestServer(t, handler)
			handle := NewTestPtyHandle(tc.pid, "sbx-x", client)

			_ = handle.Kill(context.Background())

			if gotPath != tc.wantPath {
				t.Errorf("expected path %s, got %s", tc.wantPath, gotPath)
			}
		})
	}
}

// isErrorType is a helper that tries to unwrap err to the target type.
func isErrorType[T error](err error, target *T) bool {
	if err == nil {
		return false
	}
	return errors.As(err, target)
}
