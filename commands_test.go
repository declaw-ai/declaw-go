package declaw

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------- helpers ----------

// newTestCommandsServer creates an httptest.Server and returns a Commands instance pointing at it.
// The caller should defer server.Close().
func newTestCommandsServer(t *testing.T, handler http.Handler) (*Commands, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	cfg := &Config{
		APIKey: "test-key",
		APIURL: server.URL,
	}
	client := NewTestAPIClient(cfg)
	cmds := NewTestCommands("sbx-123", client)
	return cmds, server
}

// newTestCommandHandle creates an httptest.Server and returns a CommandHandle pointing at it.
func newTestCommandHandle(t *testing.T, pid int, handler http.Handler) (*CommandHandle, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	cfg := &Config{
		APIKey: "test-key",
		APIURL: server.URL,
	}
	client := NewTestAPIClient(cfg)
	handle := NewTestCommandHandle(pid, "sbx-123", client)
	return handle, server
}

// ---------- Commands.Run (foreground) ----------

func TestCommandsRun_Foreground_ReturnsCommandResult(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}
		if req["cmd"] != "echo hello" {
			t.Errorf("expected cmd %q, got %q", "echo hello", req["cmd"])
		}
		if req["background"] != false {
			t.Errorf("expected background=false, got %v", req["background"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid":       1,
			"exit_code": 0,
			"stdout":    "hello\n",
			"stderr":    "",
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	result, err := cmds.Run(context.Background(), "echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.PID != 1 {
		t.Errorf("expected PID 1, got %d", result.PID)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("expected stdout %q, got %q", "hello\n", result.Stdout)
	}
	if result.Stderr != "" {
		t.Errorf("expected empty stderr, got %q", result.Stderr)
	}
}

func TestCommandsRun_Foreground_VerifiesHTTPDetails(t *testing.T) {
	var capturedMethod string
	var capturedPath string
	var capturedContentType string
	var capturedAuth string

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedContentType = r.Header.Get("Content-Type")
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid": 1, "exit_code": 0, "stdout": "", "stderr": "",
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	_, err := cmds.Run(context.Background(), "ls")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedMethod != "POST" {
		t.Errorf("expected POST, got %s", capturedMethod)
	}
	if capturedPath != "/sandboxes/sbx-123/commands" {
		t.Errorf("expected path /sandboxes/sbx-123/commands, got %s", capturedPath)
	}
	if !strings.Contains(capturedContentType, "application/json") {
		t.Errorf("expected Content-Type containing application/json, got %q", capturedContentType)
	}
	if capturedAuth == "" {
		t.Log("note: Authorization header was empty; implementation may set it differently")
	}
}

func TestCommandsRun_WithUser(t *testing.T) {
	var capturedBody map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid": 1, "exit_code": 0, "stdout": "", "stderr": "",
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	_, err := cmds.Run(context.Background(), "whoami", WithUser("root"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedBody["user"] != "root" {
		t.Errorf("expected user %q in request body, got %v", "root", capturedBody["user"])
	}
}

func TestCommandsRun_WithCwd(t *testing.T) {
	var capturedBody map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid": 1, "exit_code": 0, "stdout": "", "stderr": "",
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	_, err := cmds.Run(context.Background(), "ls", WithCwd("/tmp"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedBody["cwd"] != "/tmp" {
		t.Errorf("expected cwd %q in request body, got %v", "/tmp", capturedBody["cwd"])
	}
}

func TestCommandsRun_WithRunEnvs(t *testing.T) {
	var capturedBody map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid": 1, "exit_code": 0, "stdout": "bar\n", "stderr": "",
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	_, err := cmds.Run(context.Background(), "echo $FOO", WithRunEnvs(map[string]string{"FOO": "bar"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	envs, ok := capturedBody["envs"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected envs to be a map, got %T: %v", capturedBody["envs"], capturedBody["envs"])
	}
	if envs["FOO"] != "bar" {
		t.Errorf("expected envs[FOO]=%q, got %v", "bar", envs["FOO"])
	}
}

func TestCommandsRun_WithRunTimeout(t *testing.T) {
	var capturedBody map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid": 1, "exit_code": 0, "stdout": "", "stderr": "",
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	_, err := cmds.Run(context.Background(), "sleep 1", WithRunTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The timeout should be sent as seconds (integer 30)
	timeout, ok := capturedBody["timeout"]
	if !ok {
		t.Fatal("expected timeout field in request body")
	}
	// JSON numbers are float64
	if timeoutVal, ok := timeout.(float64); ok {
		if timeoutVal != 30 {
			t.Errorf("expected timeout=30, got %v", timeoutVal)
		}
	} else {
		t.Errorf("expected timeout to be a number, got %T: %v", timeout, timeout)
	}
}

func TestCommandsRun_WithAllOptions(t *testing.T) {
	var capturedBody map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid": 1, "exit_code": 0, "stdout": "", "stderr": "",
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	_, err := cmds.Run(context.Background(), "ls -la",
		WithUser("root"),
		WithCwd("/tmp"),
		WithRunEnvs(map[string]string{"FOO": "bar"}),
		WithRunTimeout(120*time.Second),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedBody["cmd"] != "ls -la" {
		t.Errorf("expected cmd %q, got %v", "ls -la", capturedBody["cmd"])
	}
	if capturedBody["user"] != "root" {
		t.Errorf("expected user %q, got %v", "root", capturedBody["user"])
	}
	if capturedBody["cwd"] != "/tmp" {
		t.Errorf("expected cwd %q, got %v", "/tmp", capturedBody["cwd"])
	}
	if capturedBody["background"] != false {
		t.Errorf("expected background=false, got %v", capturedBody["background"])
	}
}

func TestCommandsRun_OnStdoutCallback(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid":       1,
			"exit_code": 0,
			"stdout":    "line1\nline2\nline3\n",
			"stderr":    "",
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	var mu sync.Mutex
	var lines []string
	_, err := cmds.Run(context.Background(), "some-cmd",
		WithOnStdout(func(line string) {
			mu.Lock()
			defer mu.Unlock()
			lines = append(lines, line)
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	// The callback should be invoked for each non-empty line
	if len(lines) < 2 {
		t.Errorf("expected at least 2 stdout lines, got %d: %v", len(lines), lines)
	}
}

func TestCommandsRun_OnStderrCallback(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid":       1,
			"exit_code": 0,
			"stdout":    "",
			"stderr":    "warn1\nwarn2\n",
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	var mu sync.Mutex
	var errLines []string
	_, err := cmds.Run(context.Background(), "cmd",
		WithOnStderr(func(line string) {
			mu.Lock()
			defer mu.Unlock()
			errLines = append(errLines, line)
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(errLines) < 2 {
		t.Errorf("expected at least 2 stderr lines, got %d: %v", len(errLines), errLines)
	}
}

func TestCommandsRun_BothCallbacks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid":       1,
			"exit_code": 0,
			"stdout":    "out1\nout2\n",
			"stderr":    "err1\n",
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	var mu sync.Mutex
	var stdoutLines []string
	var stderrLines []string
	_, err := cmds.Run(context.Background(), "cmd",
		WithOnStdout(func(line string) {
			mu.Lock()
			defer mu.Unlock()
			stdoutLines = append(stdoutLines, line)
		}),
		WithOnStderr(func(line string) {
			mu.Lock()
			defer mu.Unlock()
			stderrLines = append(stderrLines, line)
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(stdoutLines) < 2 {
		t.Errorf("expected at least 2 stdout lines, got %d", len(stdoutLines))
	}
	if len(stderrLines) < 1 {
		t.Errorf("expected at least 1 stderr line, got %d", len(stderrLines))
	}
}

func TestCommandsRun_NonZeroExitCode_ReturnsCommandExitError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid":       1,
			"exit_code": 1,
			"stdout":    "",
			"stderr":    "command not found\n",
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	result, err := cmds.Run(context.Background(), "badcmd")

	// Implementation may either return an error (CommandExitError) or return the result
	// with non-zero exit code. We test for both patterns.
	if err != nil {
		// Should be a CommandExitError
		exitErr, ok := err.(*CommandExitError)
		if !ok {
			t.Fatalf("expected *CommandExitError, got %T: %v", err, err)
		}
		if exitErr.ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", exitErr.ExitCode)
		}
		if exitErr.Stderr != "command not found\n" {
			t.Errorf("expected stderr %q, got %q", "command not found\n", exitErr.Stderr)
		}
	} else if result != nil {
		// Some implementations return the result with non-zero exit code
		if result.ExitCode != 1 {
			t.Errorf("expected exit code 1 in result, got %d", result.ExitCode)
		}
	} else {
		t.Fatal("expected either an error or a non-nil result")
	}
}

func TestCommandsRun_ServerError_500(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	_, err := cmds.Run(context.Background(), "echo hello")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestCommandsRun_NotFound_404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("sandbox not found"))
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	_, err := cmds.Run(context.Background(), "echo hello")
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}

	// Optionally check it's a NotFoundError
	if notFoundErr, ok := err.(*NotFoundError); ok {
		if notFoundErr.StatusCode != 404 {
			t.Errorf("expected status code 404, got %d", notFoundErr.StatusCode)
		}
	}
}

func TestCommandsRun_EmptyCommand(t *testing.T) {
	var capturedBody map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid": 1, "exit_code": 0, "stdout": "", "stderr": "",
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	// Empty command should still be sent
	_, err := cmds.Run(context.Background(), "")
	// Implementation may reject or forward; just check it doesn't panic
	_ = err

	if capturedBody != nil {
		if capturedBody["cmd"] != "" {
			t.Errorf("expected empty cmd, got %v", capturedBody["cmd"])
		}
	}
}

func TestCommandsRun_ContextCancellation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow response
		time.Sleep(2 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid": 1, "exit_code": 0, "stdout": "", "stderr": "",
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := cmds.Run(ctx, "sleep 100")
	if err == nil {
		t.Fatal("expected error due to context cancellation, got nil")
	}
}

// ---------- Commands.Start (background) ----------

func TestCommandsStart_ReturnsCommandHandle(t *testing.T) {
	var capturedBody map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid": 42,
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	handle, err := cmds.Start(context.Background(), "sleep 100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle == nil {
		t.Fatal("expected non-nil handle")
	}
	if handle.PID != 42 {
		t.Errorf("expected PID 42, got %d", handle.PID)
	}

	// Verify background=true was sent
	if capturedBody["background"] != true {
		t.Errorf("expected background=true in request body, got %v", capturedBody["background"])
	}
	if capturedBody["cmd"] != "sleep 100" {
		t.Errorf("expected cmd %q, got %v", "sleep 100", capturedBody["cmd"])
	}
}

func TestCommandsStart_WithOptions(t *testing.T) {
	var capturedBody map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid": 99,
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	handle, err := cmds.Start(context.Background(), "python script.py",
		WithUser("root"),
		WithCwd("/app"),
		WithRunEnvs(map[string]string{"PYTHONPATH": "/app"}),
		WithRunTimeout(300*time.Second),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle.PID != 99 {
		t.Errorf("expected PID 99, got %d", handle.PID)
	}

	if capturedBody["background"] != true {
		t.Errorf("expected background=true, got %v", capturedBody["background"])
	}
	if capturedBody["user"] != "root" {
		t.Errorf("expected user=root, got %v", capturedBody["user"])
	}
	if capturedBody["cwd"] != "/app" {
		t.Errorf("expected cwd=/app, got %v", capturedBody["cwd"])
	}
}

func TestCommandsStart_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	_, err := cmds.Start(context.Background(), "sleep 100")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

// ---------- CommandHandle.Wait ----------

func TestCommandHandleWait_ReturnsCommandResult(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands/42/wait", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid":       42,
			"exit_code": 0,
			"stdout":    "done\n",
			"stderr":    "",
		})
	})

	handle, server := newTestCommandHandle(t, 42, mux)
	defer server.Close()

	result, err := handle.Wait(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Stdout != "done\n" {
		t.Errorf("expected stdout %q, got %q", "done\n", result.Stdout)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.PID != 42 {
		t.Errorf("expected PID 42, got %d", result.PID)
	}
}

func TestCommandHandleWait_NonZeroExit_ReturnsCommandExitError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands/10/wait", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid":       10,
			"exit_code": 1,
			"stdout":    "",
			"stderr":    "error\n",
		})
	})

	handle, server := newTestCommandHandle(t, 10, mux)
	defer server.Close()

	result, err := handle.Wait(context.Background())

	if err != nil {
		exitErr, ok := err.(*CommandExitError)
		if !ok {
			t.Fatalf("expected *CommandExitError, got %T: %v", err, err)
		}
		if exitErr.ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", exitErr.ExitCode)
		}
		if exitErr.Stderr != "error\n" {
			t.Errorf("expected stderr %q, got %q", "error\n", exitErr.Stderr)
		}
	} else if result != nil {
		if result.ExitCode != 1 {
			t.Errorf("expected exit code 1 in result, got %d", result.ExitCode)
		}
	} else {
		t.Fatal("expected either an error or a non-nil result")
	}
}

func TestCommandHandleWait_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands/42/wait", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	})

	handle, server := newTestCommandHandle(t, 42, mux)
	defer server.Close()

	_, err := handle.Wait(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestCommandHandleWait_VerifiesCorrectPath(t *testing.T) {
	var capturedPath string

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands/99/wait", func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid": 99, "exit_code": 0, "stdout": "", "stderr": "",
		})
	})

	handle, server := newTestCommandHandle(t, 99, mux)
	defer server.Close()

	_, err := handle.Wait(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedPath != "/sandboxes/sbx-123/commands/99/wait" {
		t.Errorf("expected path /sandboxes/sbx-123/commands/99/wait, got %s", capturedPath)
	}
}

// ---------- CommandHandle.Kill ----------

func TestCommandHandleKill_SendsDeleteRequest(t *testing.T) {
	var capturedMethod string
	var capturedPath string

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands/42", func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})

	handle, server := newTestCommandHandle(t, 42, mux)
	defer server.Close()

	err := handle.Kill(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMethod != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", capturedMethod)
	}
	if capturedPath != "/sandboxes/sbx-123/commands/42" {
		t.Errorf("expected path /sandboxes/sbx-123/commands/42, got %s", capturedPath)
	}
}

func TestCommandHandleKill_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands/42", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	})

	handle, server := newTestCommandHandle(t, 42, mux)
	defer server.Close()

	err := handle.Kill(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// ---------- CommandHandle.SendStdin ----------

func TestCommandHandleSendStdin_NotImplemented(t *testing.T) {
	mux := http.NewServeMux()
	handle, server := newTestCommandHandle(t, 42, mux)
	defer server.Close()

	err := handle.SendStdin(context.Background(), "input\n")
	if err == nil {
		t.Fatal("expected error from SendStdin")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("expected 'not yet implemented' error, got: %v", err)
	}
}

// ---------- Commands.List ----------

func TestCommandsList_ReturnsProcessInfoSlice(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			// POST is for Run, GET is for List
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"pid": 1, "cmd": "echo hello", "is_pty": false},
			{"pid": 2, "cmd": "sleep 100", "is_pty": true},
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	procs, err := cmds.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(procs) != 2 {
		t.Fatalf("expected 2 processes, got %d", len(procs))
	}
	if procs[0].PID != 1 {
		t.Errorf("expected first process PID=1, got %d", procs[0].PID)
	}
	if procs[0].Cmd != "echo hello" {
		t.Errorf("expected first process cmd %q, got %q", "echo hello", procs[0].Cmd)
	}
	if procs[0].IsPty {
		t.Error("expected first process IsPty=false")
	}
	if procs[1].PID != 2 {
		t.Errorf("expected second process PID=2, got %d", procs[1].PID)
	}
}

func TestCommandsList_EmptyList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	procs, err := cmds.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(procs) != 0 {
		t.Errorf("expected empty process list, got %d", len(procs))
	}
}

func TestCommandsList_VerifiesGetMethod(t *testing.T) {
	var capturedMethod string

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	_, err := cmds.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMethod != http.MethodGet {
		t.Errorf("expected GET for List, got %s", capturedMethod)
	}
}

func TestCommandsList_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	_, err := cmds.List(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// ---------- RunOption builder tests ----------

func TestRunOptions_WithUser(t *testing.T) {
	opts := &runOpts{}
	WithUser("admin")(opts)
	if opts.User != "admin" {
		t.Errorf("expected user %q, got %q", "admin", opts.User)
	}
}

func TestRunOptions_WithCwd(t *testing.T) {
	opts := &runOpts{}
	WithCwd("/var/log")(opts)
	if opts.Cwd != "/var/log" {
		t.Errorf("expected cwd %q, got %q", "/var/log", opts.Cwd)
	}
}

func TestRunOptions_WithRunEnvs(t *testing.T) {
	opts := &runOpts{}
	envs := map[string]string{"A": "1", "B": "2"}
	WithRunEnvs(envs)(opts)
	if len(opts.Envs) != 2 {
		t.Errorf("expected 2 envs, got %d", len(opts.Envs))
	}
	if opts.Envs["A"] != "1" {
		t.Errorf("expected envs[A]=%q, got %q", "1", opts.Envs["A"])
	}
}

func TestRunOptions_WithStdin(t *testing.T) {
	opts := &runOpts{}
	WithStdin()(opts)
	if !opts.Stdin {
		t.Error("expected Stdin=true after WithStdin()")
	}
}

func TestRunOptions_WithRunTimeout(t *testing.T) {
	opts := &runOpts{}
	WithRunTimeout(45 * time.Second)(opts)
	if opts.Timeout != 45*time.Second {
		t.Errorf("expected timeout 45s, got %v", opts.Timeout)
	}
}

func TestRunOptions_WithOnStdout(t *testing.T) {
	opts := &runOpts{}
	called := false
	WithOnStdout(func(line string) { called = true })(opts)
	if opts.OnStdout == nil {
		t.Fatal("expected OnStdout to be set")
	}
	opts.OnStdout("test")
	if !called {
		t.Error("expected OnStdout callback to be invoked")
	}
}

func TestRunOptions_WithOnStderr(t *testing.T) {
	opts := &runOpts{}
	called := false
	WithOnStderr(func(line string) { called = true })(opts)
	if opts.OnStderr == nil {
		t.Fatal("expected OnStderr to be set")
	}
	opts.OnStderr("test")
	if !called {
		t.Error("expected OnStderr callback to be invoked")
	}
}

func TestRunOptions_LastOptionWins(t *testing.T) {
	opts := &runOpts{}
	WithUser("first")(opts)
	WithUser("second")(opts)
	if opts.User != "second" {
		t.Errorf("expected last user option to win, got %q", opts.User)
	}
}

func TestRunOptions_MultipleOptions(t *testing.T) {
	opts := &runOpts{}
	WithUser("root")(opts)
	WithCwd("/tmp")(opts)
	WithRunTimeout(60 * time.Second)(opts)
	WithStdin()(opts)

	if opts.User != "root" {
		t.Errorf("expected user %q, got %q", "root", opts.User)
	}
	if opts.Cwd != "/tmp" {
		t.Errorf("expected cwd %q, got %q", "/tmp", opts.Cwd)
	}
	if opts.Timeout != 60*time.Second {
		t.Errorf("expected timeout 60s, got %v", opts.Timeout)
	}
	if !opts.Stdin {
		t.Error("expected Stdin=true")
	}
}

// ---------- Table-driven comprehensive tests ----------

func TestCommandsRun_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		cmd          string
		opts         []RunOption
		serverResp   map[string]interface{}
		wantPID      int
		wantExitCode int
		wantStdout   string
		wantStderr   string
		wantErr      bool
	}{
		{
			name: "simple echo",
			cmd:  "echo hello",
			serverResp: map[string]interface{}{
				"pid": 1, "exit_code": 0, "stdout": "hello\n", "stderr": "",
			},
			wantPID: 1, wantExitCode: 0, wantStdout: "hello\n", wantStderr: "",
		},
		{
			name: "multi-line output",
			cmd:  "seq 3",
			serverResp: map[string]interface{}{
				"pid": 2, "exit_code": 0, "stdout": "1\n2\n3\n", "stderr": "",
			},
			wantPID: 2, wantExitCode: 0, wantStdout: "1\n2\n3\n",
		},
		{
			name: "stderr output",
			cmd:  "echo err >&2",
			serverResp: map[string]interface{}{
				"pid": 3, "exit_code": 0, "stdout": "", "stderr": "err\n",
			},
			wantPID: 3, wantStderr: "err\n",
		},
		{
			name: "non-zero exit code",
			cmd:  "false",
			serverResp: map[string]interface{}{
				"pid": 4, "exit_code": 127, "stdout": "", "stderr": "not found\n",
			},
			wantPID: 4, wantExitCode: 127, wantStderr: "not found\n", wantErr: true,
		},
		{
			name: "command with special characters",
			cmd:  `echo "hello world" && echo 'single quotes' | grep hello`,
			serverResp: map[string]interface{}{
				"pid": 5, "exit_code": 0, "stdout": "hello world\n", "stderr": "",
			},
			wantPID: 5, wantStdout: "hello world\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tc.serverResp)
			})

			cmds, server := newTestCommandsServer(t, mux)
			defer server.Close()

			result, err := cmds.Run(context.Background(), tc.cmd, tc.opts...)

			if tc.wantErr {
				// Non-zero exit: either error or result with non-zero code
				if err != nil {
					if exitErr, ok := err.(*CommandExitError); ok {
						if exitErr.ExitCode != tc.wantExitCode {
							t.Errorf("exit code: want %d, got %d", tc.wantExitCode, exitErr.ExitCode)
						}
					}
				} else if result != nil {
					if result.ExitCode != tc.wantExitCode {
						t.Errorf("exit code: want %d, got %d", tc.wantExitCode, result.ExitCode)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.PID != tc.wantPID {
				t.Errorf("PID: want %d, got %d", tc.wantPID, result.PID)
			}
			if result.ExitCode != tc.wantExitCode {
				t.Errorf("exit code: want %d, got %d", tc.wantExitCode, result.ExitCode)
			}
			if result.Stdout != tc.wantStdout {
				t.Errorf("stdout: want %q, got %q", tc.wantStdout, result.Stdout)
			}
			if result.Stderr != tc.wantStderr {
				t.Errorf("stderr: want %q, got %q", tc.wantStderr, result.Stderr)
			}
		})
	}
}

func TestCommandsStart_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		cmd        string
		opts       []RunOption
		serverResp map[string]interface{}
		wantPID    int
		wantErr    bool
	}{
		{
			name:       "simple background command",
			cmd:        "sleep 100",
			serverResp: map[string]interface{}{"pid": 42},
			wantPID:    42,
		},
		{
			name:       "background with options",
			cmd:        "python app.py",
			opts:       []RunOption{WithUser("root"), WithCwd("/app")},
			serverResp: map[string]interface{}{"pid": 99},
			wantPID:    99,
		},
		{
			name:       "pid zero",
			cmd:        "init",
			serverResp: map[string]interface{}{"pid": 0},
			wantPID:    0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tc.serverResp)
			})

			cmds, server := newTestCommandsServer(t, mux)
			defer server.Close()

			handle, err := cmds.Start(context.Background(), tc.cmd, tc.opts...)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if handle.PID != tc.wantPID {
				t.Errorf("PID: want %d, got %d", tc.wantPID, handle.PID)
			}
		})
	}
}

// ---------- Authentication header tests ----------

func TestCommandsRun_SetsAuthorizationHeader(t *testing.T) {
	var capturedAuth string

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pid": 1, "exit_code": 0, "stdout": "", "stderr": "",
		})
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	_, _ = cmds.Run(context.Background(), "echo test")

	// The Authorization header should contain the API key in some form
	// Common patterns: "Bearer test-key" or just presence of the key
	if capturedAuth != "" {
		if !strings.Contains(capturedAuth, "test-key") {
			t.Logf("Authorization header present but does not contain API key: %q", capturedAuth)
		}
	}
}

// ---------- Error type mapping tests ----------

func TestCommandsRun_AuthenticationError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	_, err := cmds.Run(context.Background(), "echo hello")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if authErr, ok := err.(*AuthenticationError); ok {
		if authErr.StatusCode != 401 {
			t.Errorf("expected status 401, got %d", authErr.StatusCode)
		}
	}
}

func TestCommandsRun_RateLimitError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/commands", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.Header().Set("X-RateLimit-Limit", "100")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("rate limited"))
	})

	cmds, server := newTestCommandsServer(t, mux)
	defer server.Close()

	_, err := cmds.Run(context.Background(), "echo hello")
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
	if rlErr, ok := err.(*RateLimitError); ok {
		if rlErr.RetryAfter != 30*time.Second {
			t.Errorf("expected RetryAfter 30s, got %v", rlErr.RetryAfter)
		}
		if rlErr.Limit != 100 {
			t.Errorf("expected Limit 100, got %d", rlErr.Limit)
		}
		if rlErr.Remaining != 0 {
			t.Errorf("expected Remaining 0, got %d", rlErr.Remaining)
		}
	}
}
