package declaw

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// ---------- helpers ----------

// newTestFilesystemServer creates an httptest.Server and returns a Filesystem instance pointing at it.
// The caller should defer server.Close().
func newTestFilesystemServer(t *testing.T, handler http.Handler) (*Filesystem, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	cfg := &Config{
		APIKey: "test-key",
		APIURL: server.URL,
	}
	client := NewTestAPIClient(cfg)
	fs := NewTestFilesystem("sbx-123", client)
	return fs, server
}

// ---------- Filesystem.Read ----------

func TestFilesystemRead_ReturnsContent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Query().Get("path")
		if path != "/tmp/file.txt" {
			t.Errorf("expected path query param %q, got %q", "/tmp/file.txt", path)
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("hello world"))
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	content, err := fs.Read(context.Background(), "/tmp/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "hello world" {
		t.Errorf("expected content %q, got %q", "hello world", content)
	}
}

func TestFilesystemRead_WithFileUser(t *testing.T) {
	var capturedUsername string

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		capturedUsername = r.URL.Query().Get("username")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("data"))
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	_, err := fs.Read(context.Background(), "/tmp/file.txt", WithFileUser("root"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedUsername != "root" {
		t.Errorf("expected username %q, got %q", "root", capturedUsername)
	}
}

func TestFilesystemRead_VerifiesGETMethod(t *testing.T) {
	var capturedMethod string

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte(""))
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	_, err := fs.Read(context.Background(), "/tmp/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", capturedMethod)
	}
}

func TestFilesystemRead_PathURLEncoded(t *testing.T) {
	var capturedPath string

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Query().Get("path")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("data"))
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	// Path with special characters that need URL encoding
	_, err := fs.Read(context.Background(), "/tmp/file with spaces.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedPath != "/tmp/file with spaces.txt" {
		t.Errorf("expected path %q, got %q", "/tmp/file with spaces.txt", capturedPath)
	}
}

func TestFilesystemRead_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("file not found"))
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	_, err := fs.Read(context.Background(), "/tmp/missing.txt")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestFilesystemRead_EmptyContent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		// Empty body
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	content, err := fs.Read(context.Background(), "/tmp/empty.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
}

// ---------- Filesystem.ReadBytes ----------

func TestFilesystemReadBytes_ReturnsRawBytes(t *testing.T) {
	expected := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/raw", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		path := r.URL.Query().Get("path")
		if path != "/tmp/file.bin" {
			t.Errorf("expected path %q, got %q", "/tmp/file.bin", path)
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(expected)
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	data, err := fs.ReadBytes(context.Background(), "/tmp/file.bin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(data, expected) {
		t.Errorf("expected bytes %v, got %v", expected, data)
	}
}

func TestFilesystemReadBytes_NonUTF8Bytes(t *testing.T) {
	// Non-UTF8 bytes must survive round-trip without corruption
	expected := []byte{0xFF, 0xFE, 0x00, 0x01, 0x80, 0x81, 0xC0, 0xC1}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/raw", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(expected)
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	data, err := fs.ReadBytes(context.Background(), "/tmp/nonutf8.bin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(data, expected) {
		t.Errorf("non-UTF8 bytes corrupted: expected %v, got %v", expected, data)
	}
}

func TestFilesystemReadBytes_PNGMagicBytes(t *testing.T) {
	pngMagic := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/raw", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(pngMagic)
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	data, err := fs.ReadBytes(context.Background(), "/tmp/image.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(data, pngMagic) {
		t.Errorf("PNG magic bytes corrupted: expected %v, got %v", pngMagic, data)
	}
}

func TestFilesystemReadBytes_Random4KB(t *testing.T) {
	expected := make([]byte, 4096)
	if _, err := rand.Read(expected); err != nil {
		t.Fatalf("failed to generate random bytes: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/raw", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(expected)
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	data, err := fs.ReadBytes(context.Background(), "/tmp/random.bin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(data, expected) {
		t.Errorf("4KB random bytes mismatch: lengths %d vs %d", len(expected), len(data))
	}
}

func TestFilesystemReadBytes_EmptyFile(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/raw", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		// Write nothing
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	data, err := fs.ReadBytes(context.Background(), "/tmp/empty.bin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty bytes, got %d bytes", len(data))
	}
}

// ---------- Filesystem.Write (string) ----------

func TestFilesystemWrite_SendsPostWithJSONBody(t *testing.T) {
	var capturedMethod string
	var capturedPath string
	var capturedContentType string
	var capturedBody map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			// Allow GET for Read tests
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":        "/tmp/file.txt",
			"size":        5,
					})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	info, err := fs.Write(context.Background(), "/tmp/file.txt", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", capturedMethod)
	}
	if capturedPath != "/sandboxes/sbx-123/files" {
		t.Errorf("expected path /sandboxes/sbx-123/files, got %s", capturedPath)
	}
	if !strings.Contains(capturedContentType, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", capturedContentType)
	}
	if capturedBody["path"] != "/tmp/file.txt" {
		t.Errorf("expected body path %q, got %v", "/tmp/file.txt", capturedBody["path"])
	}
	if capturedBody["data"] != "hello" {
		t.Errorf("expected body data %q, got %v", "hello", capturedBody["data"])
	}
	if info == nil {
		t.Fatal("expected non-nil WriteInfo")
	}
	if info.Path != "/tmp/file.txt" {
		t.Errorf("expected WriteInfo.Path %q, got %q", "/tmp/file.txt", info.Path)
	}
	if info.Size != 5 {
		t.Errorf("expected WriteInfo.Size 5, got %d", info.Size)
	}
}

func TestFilesystemWrite_EmptyString(t *testing.T) {
	var capturedBody map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path": "/tmp/empty.txt", "size": 0, 		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	_, err := fs.Write(context.Background(), "/tmp/empty.txt", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedBody["data"] != "" {
		t.Errorf("expected empty data, got %v", capturedBody["data"])
	}
}

func TestFilesystemWrite_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("disk full"))
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	_, err := fs.Write(context.Background(), "/tmp/file.txt", "data")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// ---------- Filesystem.WriteBytes (binary) ----------

func TestFilesystemWriteBytes_SendsPutToRawEndpoint(t *testing.T) {
	var capturedMethod string
	var capturedContentType string
	var capturedPath string
	var capturedBody []byte

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/raw", func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedContentType = r.Header.Get("Content-Type")
		capturedPath = r.URL.Query().Get("path")
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path": "/tmp/file.bin", "size": len(capturedBody), 		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	payload := []byte{0x89, 0x50, 0x4e, 0x47}
	info, err := fs.WriteBytes(context.Background(), "/tmp/file.bin", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMethod != http.MethodPut {
		t.Errorf("expected PUT, got %s", capturedMethod)
	}
	if !strings.Contains(strings.ToLower(capturedContentType), "application/octet-stream") {
		t.Errorf("expected Content-Type application/octet-stream, got %q", capturedContentType)
	}
	if capturedPath != "/tmp/file.bin" {
		t.Errorf("expected query path %q, got %q", "/tmp/file.bin", capturedPath)
	}
	if !bytes.Equal(capturedBody, payload) {
		t.Errorf("body mismatch: expected %v, got %v", payload, capturedBody)
	}
	if info == nil {
		t.Fatal("expected non-nil WriteInfo")
	}
	if info.Size != int64(len(payload)) {
		t.Errorf("expected size %d, got %d", len(payload), info.Size)
	}
}

func TestFilesystemWriteBytes_PNGMagic(t *testing.T) {
	pngMagic := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	var capturedBody []byte

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/raw", func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path": "/tmp/img.png", "size": len(capturedBody), 		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	_, err := fs.WriteBytes(context.Background(), "/tmp/img.png", pngMagic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(capturedBody, pngMagic) {
		t.Errorf("PNG magic bytes corrupted on write: expected %v, got %v", pngMagic, capturedBody)
	}
}

func TestFilesystemWriteBytes_NonUTF8ByteIdentical(t *testing.T) {
	payload := []byte{0xFF, 0xFE, 0x00, 0x01, 0x80, 0x81, 0xC0, 0xC1}
	var capturedBody []byte

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/raw", func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path": "/tmp/x.bin", "size": len(capturedBody), 		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	_, err := fs.WriteBytes(context.Background(), "/tmp/x.bin", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(capturedBody, payload) {
		t.Errorf("non-UTF8 bytes corrupted on write: expected %v, got %v", payload, capturedBody)
	}
}

func TestFilesystemWriteBytes_NoUFFDCorruption(t *testing.T) {
	// Regression test: bytes must NOT be lossy-decoded as UTF-8 (U+FFFD = 0xEF 0xBF 0xBD)
	payload := []byte{0xFF, 0xFE}
	var capturedBody []byte

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/raw", func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path": "/tmp/x.bin", "size": 2, 		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	_, err := fs.WriteBytes(context.Background(), "/tmp/x.bin", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for U+FFFD fingerprint (0xEF 0xBF 0xBD)
	for i := 0; i+2 < len(capturedBody); i++ {
		if capturedBody[i] == 0xEF && capturedBody[i+1] == 0xBF && capturedBody[i+2] == 0xBD {
			t.Fatal("SDK lossy-decoded bytes as UTF-8 (U+FFFD fingerprint detected in wire body)")
		}
	}
	if !bytes.Equal(capturedBody, payload) {
		t.Errorf("wire body mismatch: expected %v, got %v", payload, capturedBody)
	}
}

func TestFilesystemWriteBytes_Random4KBRoundTrip(t *testing.T) {
	payload := make([]byte, 4096)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("failed to generate random bytes: %v", err)
	}
	var capturedBody []byte

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/raw", func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path": "/tmp/b.bin", "size": len(capturedBody), 		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	_, err := fs.WriteBytes(context.Background(), "/tmp/b.bin", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(capturedBody, payload) {
		t.Errorf("4KB random bytes mismatch: wrote %d bytes, captured %d bytes", len(payload), len(capturedBody))
	}
}

func TestFilesystemWriteBytes_EmptyPayload(t *testing.T) {
	var capturedBody []byte

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/raw", func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path": "/tmp/empty.bin", "size": 0, 		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	_, err := fs.WriteBytes(context.Background(), "/tmp/empty.bin", []byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedBody) != 0 {
		t.Errorf("expected empty body, got %d bytes", len(capturedBody))
	}
}

// ---------- Filesystem.WriteFiles (batch) ----------

func TestFilesystemWriteFiles_SendsBatchRequest(t *testing.T) {
	var capturedBody map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/batch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"path": "/a.txt", "size": 3},
			{"path": "/b.txt", "size": 3},
		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	entries := []WriteEntry{
		{Path: "/a.txt", Data: "aaa"},
		{Path: "/b.txt", Data: "bbb"},
	}
	err := fs.WriteFiles(context.Background(), entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files, ok := capturedBody["files"].([]interface{})
	if !ok {
		t.Fatalf("expected files array in body, got %T", capturedBody["files"])
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files in batch, got %d", len(files))
	}
	file0, ok := files[0].(map[string]interface{})
	if ok {
		if file0["path"] != "/a.txt" {
			t.Errorf("expected first file path /a.txt, got %v", file0["path"])
		}
		if file0["data"] != "aaa" {
			t.Errorf("expected first file data 'aaa', got %v", file0["data"])
		}
	}
}

func TestFilesystemWriteFiles_EmptyList(t *testing.T) {
	fs, server := newTestFilesystemServer(t, http.NewServeMux())
	defer server.Close()

	// Empty list should not error (or send empty batch)
	err := fs.WriteFiles(context.Background(), []WriteEntry{})
	// Implementation may or may not make a request; just ensure no panic
	_ = err
}

func TestFilesystemWriteFiles_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/batch", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	err := fs.WriteFiles(context.Background(), []WriteEntry{
		{Path: "/a.txt", Data: "data"},
	})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestFilesystemWriteFiles_WithFileUser(t *testing.T) {
	var capturedBody map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/batch", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"path": "/a.txt", "size": 3},
		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	entries := []WriteEntry{
		{Path: "/a.txt", Data: "aaa"},
	}
	err := fs.WriteFiles(context.Background(), entries, WithFileUser("testuser"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files, ok := capturedBody["files"].([]interface{})
	if !ok || len(files) == 0 {
		t.Fatal("expected files array in body")
	}
	file0, ok := files[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected file entry to be a map")
	}
	username, ok := file0["username"].(string)
	if !ok || username != "testuser" {
		t.Errorf("expected username='testuser' per file, got %v", file0["username"])
	}
}

func TestFilesystemWriteFiles_RejectsByteData(t *testing.T) {
	fs, server := newTestFilesystemServer(t, http.NewServeMux())
	defer server.Close()

	entries := []WriteEntry{
		{Path: "/a.bin", Data: []byte("binary")},
	}
	err := fs.WriteFiles(context.Background(), entries)
	if err == nil {
		t.Fatal("expected error for []byte data in batch write")
	}
	if !strings.Contains(err.Error(), "must be a string") {
		t.Errorf("expected 'must be a string' in error, got: %v", err)
	}
}

// ---------- Filesystem.List ----------

func TestFilesystemList_ReturnsEntryInfoSlice(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		path := r.URL.Query().Get("path")
		if path != "/tmp" {
			t.Errorf("expected path query param %q, got %q", "/tmp", path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"path": "/tmp/a.txt", "type": "file", "size": 100},
			{"path": "/tmp/subdir", "type": "dir", "size": 0},
		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	entries, err := fs.List(context.Background(), "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Path != "/tmp/a.txt" {
		t.Errorf("expected first entry path %q, got %q", "/tmp/a.txt", entries[0].Path)
	}
	if entries[0].Type != FileTypeFile {
		t.Errorf("expected first entry type %q, got %q", FileTypeFile, entries[0].Type)
	}
	if entries[0].Size != 100 {
		t.Errorf("expected first entry size 100, got %d", entries[0].Size)
	}
	if entries[1].Path != "/tmp/subdir" {
		t.Errorf("expected second entry path %q, got %q", "/tmp/subdir", entries[1].Path)
	}
	if entries[1].Type != FileTypeDirectory {
		t.Errorf("expected second entry type %q, got %q", FileTypeDirectory, entries[1].Type)
	}
}

func TestFilesystemList_EmptyDirectory(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	entries, err := fs.List(context.Background(), "/tmp/empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty list, got %d entries", len(entries))
	}
}

func TestFilesystemList_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/list", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	_, err := fs.List(context.Background(), "/tmp")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// ---------- Filesystem.Exists ----------

func TestFilesystemExists_True(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/exists", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		path := r.URL.Query().Get("path")
		if path != "/tmp/file.txt" {
			t.Errorf("expected path %q, got %q", "/tmp/file.txt", path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"exists": true,
		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	exists, err := fs.Exists(context.Background(), "/tmp/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected file to exist")
	}
}

func TestFilesystemExists_False(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/exists", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"exists": false,
		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	exists, err := fs.Exists(context.Background(), "/tmp/missing.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected file to not exist")
	}
}

func TestFilesystemExists_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/exists", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	_, err := fs.Exists(context.Background(), "/tmp/file.txt")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// ---------- Filesystem.GetInfo ----------

func TestFilesystemGetInfo_ReturnsEntryInfo(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/info", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		path := r.URL.Query().Get("path")
		if path != "/tmp/file.txt" {
			t.Errorf("expected path %q, got %q", "/tmp/file.txt", path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path": "/tmp/file.txt",
			"type": "file",
			"size": 42,
		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	info, err := fs.GetInfo(context.Background(), "/tmp/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil EntryInfo")
	}
	if info.Path != "/tmp/file.txt" {
		t.Errorf("expected path %q, got %q", "/tmp/file.txt", info.Path)
	}
	if info.Type != FileTypeFile {
		t.Errorf("expected type %q, got %q", FileTypeFile, info.Type)
	}
	if info.Size != 42 {
		t.Errorf("expected size 42, got %d", info.Size)
	}
}

func TestFilesystemGetInfo_Directory(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path": "/tmp/mydir",
			"type": "dir",
			"size": 0,
		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	info, err := fs.GetInfo(context.Background(), "/tmp/mydir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Type != FileTypeDirectory {
		t.Errorf("expected type %q, got %q", FileTypeDirectory, info.Type)
	}
}

func TestFilesystemGetInfo_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/info", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	_, err := fs.GetInfo(context.Background(), "/tmp/missing.txt")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

// ---------- Filesystem.Remove ----------

func TestFilesystemRemove_SendsDeleteWithQueryParam(t *testing.T) {
	var capturedMethod string
	var capturedQueryPath string
	var capturedURLPath string

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			// Allow other methods for other tests
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		capturedMethod = r.Method
		capturedURLPath = r.URL.Path
		capturedQueryPath = r.URL.Query().Get("path")
		w.WriteHeader(http.StatusNoContent)
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	err := fs.Remove(context.Background(), "/tmp/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMethod != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", capturedMethod)
	}
	if capturedURLPath != "/sandboxes/sbx-123/files" {
		t.Errorf("expected URL path /sandboxes/sbx-123/files, got %s", capturedURLPath)
	}
	if capturedQueryPath != "/tmp/file.txt" {
		t.Errorf("expected query path %q, got %q", "/tmp/file.txt", capturedQueryPath)
	}
}

func TestFilesystemRemove_SpecialCharsProperlyURLEncoded(t *testing.T) {
	// IMPORTANT: path with special chars (&, ?, #, =) must be a single query param value,
	// NOT concatenated into the URL where these chars would be misinterpreted.
	testPaths := []string{
		"/tmp/evil?inject=true&admin=1",
		"/tmp/file with spaces.txt",
		"/tmp/hash#tag.txt",
		"/tmp/equals=sign.txt",
		"/tmp/special&chars.txt",
		"/tmp/unicodeéè.txt",
	}

	for _, testPath := range testPaths {
		t.Run(testPath, func(t *testing.T) {
			var capturedRawQuery string

			mux := http.NewServeMux()
			mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				capturedRawQuery = r.URL.RawQuery
				w.WriteHeader(http.StatusNoContent)
			})

			fs, server := newTestFilesystemServer(t, mux)
			defer server.Close()

			err := fs.Remove(context.Background(), testPath)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Parse the raw query to verify proper encoding
			values, parseErr := url.ParseQuery(capturedRawQuery)
			if parseErr != nil {
				t.Fatalf("failed to parse query: %v", parseErr)
			}
			got := values.Get("path")
			if got != testPath {
				t.Errorf("path not properly URL-encoded: expected %q, got %q (raw query: %s)", testPath, got, capturedRawQuery)
			}

			// Verify special chars in path did NOT become separate query params
			if strings.Contains(testPath, "?") || strings.Contains(testPath, "&") {
				if values.Get("inject") != "" || values.Get("admin") != "" {
					t.Error("special chars in path were interpreted as separate query params (URL injection vulnerability)")
				}
			}
		})
	}
}

func TestFilesystemRemove_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	err := fs.Remove(context.Background(), "/tmp/file.txt")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// ---------- Filesystem.Rename ----------

func TestFilesystemRename_SendsPatchWithBody(t *testing.T) {
	var capturedMethod string
	var capturedBody map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		capturedMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path": "/new.txt", "type": "file", "size": 10,
		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	err := fs.Rename(context.Background(), "/old.txt", "/new.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMethod != http.MethodPatch {
		t.Errorf("expected PATCH, got %s", capturedMethod)
	}
	if capturedBody["old_path"] != "/old.txt" {
		t.Errorf("expected old_path %q, got %v", "/old.txt", capturedBody["old_path"])
	}
	if capturedBody["new_path"] != "/new.txt" {
		t.Errorf("expected new_path %q, got %v", "/new.txt", capturedBody["new_path"])
	}
}

func TestFilesystemRename_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	err := fs.Rename(context.Background(), "/old.txt", "/new.txt")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// ---------- Filesystem.MakeDir ----------

func TestFilesystemMakeDir_SendsPostWithBody(t *testing.T) {
	var capturedMethod string
	var capturedPath string
	var capturedBody map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/mkdir", func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"created": true,
		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	err := fs.MakeDir(context.Background(), "/tmp/newdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", capturedMethod)
	}
	if capturedPath != "/sandboxes/sbx-123/files/mkdir" {
		t.Errorf("expected path /sandboxes/sbx-123/files/mkdir, got %s", capturedPath)
	}
	if capturedBody["path"] != "/tmp/newdir" {
		t.Errorf("expected body path %q, got %v", "/tmp/newdir", capturedBody["path"])
	}
}

func TestFilesystemMakeDir_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files/mkdir", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	err := fs.MakeDir(context.Background(), "/tmp/newdir")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// ---------- Filesystem.Watch ----------

func TestFilesystemWatch_ReturnsNotImplementedError(t *testing.T) {
	fs := NewTestFilesystem("sbx-123", NewTestAPIClient(NewConfig()))

	handle, err := fs.Watch(context.Background(), "/tmp")
	if err == nil {
		t.Fatal("expected error from Watch, got nil")
	}
	if handle != nil {
		t.Error("expected nil handle from unimplemented Watch")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("expected 'not yet implemented' error, got: %v", err)
	}
}

// ---------- WatchHandle tests (unit) ----------

func TestWatchHandle_EventsReturnsReadOnlyChannel(t *testing.T) {
	events := make(chan FilesystemEvent, 1)
	done := make(chan struct{})
	handle := &WatchHandle{
		events: events,
		done:   done,
	}

	ch := handle.Events()
	if ch == nil {
		t.Fatal("expected non-nil events channel")
	}

	// Send an event and verify it's received
	now := time.Now()
	events <- FilesystemEvent{
		Type:      EventCreated,
		Path:      "/tmp/test.txt",
		Timestamp: now,
	}

	select {
	case evt := <-ch:
		if evt.Type != EventCreated {
			t.Errorf("expected event type %q, got %q", EventCreated, evt.Type)
		}
		if evt.Path != "/tmp/test.txt" {
			t.Errorf("expected event path %q, got %q", "/tmp/test.txt", evt.Path)
		}
		if !evt.Timestamp.Equal(now) {
			t.Errorf("expected timestamp %v, got %v", now, evt.Timestamp)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for event")
	}
}

// ---------- FilesystemEvent types ----------

func TestFilesystemEventTypes(t *testing.T) {
	if EventCreated != "created" {
		t.Errorf("expected EventCreated=%q, got %q", "created", EventCreated)
	}
	if EventModified != "modified" {
		t.Errorf("expected EventModified=%q, got %q", "modified", EventModified)
	}
	if EventDeleted != "deleted" {
		t.Errorf("expected EventDeleted=%q, got %q", "deleted", EventDeleted)
	}
}

// ---------- FileType constants ----------

func TestFileTypeConstants(t *testing.T) {
	if FileTypeFile != "file" {
		t.Errorf("expected FileTypeFile=%q, got %q", "file", FileTypeFile)
	}
	if FileTypeDirectory != "dir" {
		t.Errorf("expected FileTypeDirectory=%q, got %q", "dir", FileTypeDirectory)
	}
	if FileTypeSymlink != "symlink" {
		t.Errorf("expected FileTypeSymlink=%q, got %q", "symlink", FileTypeSymlink)
	}
	if FileTypeOther != "other" {
		t.Errorf("expected FileTypeOther=%q, got %q", "other", FileTypeOther)
	}
}

// ---------- FileOption tests ----------

func TestFileOption_WithFileUser(t *testing.T) {
	opts := &fileOpts{}
	WithFileUser("admin")(opts)
	if opts.User != "admin" {
		t.Errorf("expected user %q, got %q", "admin", opts.User)
	}
}

func TestFileOption_WithFileUser_Empty(t *testing.T) {
	opts := &fileOpts{}
	WithFileUser("")(opts)
	if opts.User != "" {
		t.Errorf("expected empty user, got %q", opts.User)
	}
}

func TestFileOption_WithFileUser_LastWins(t *testing.T) {
	opts := &fileOpts{}
	WithFileUser("first")(opts)
	WithFileUser("second")(opts)
	if opts.User != "second" {
		t.Errorf("expected last user to win, got %q", opts.User)
	}
}

// ---------- Context cancellation ----------

func TestFilesystemRead_ContextCancellation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": "late",
		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := fs.Read(ctx, "/tmp/file.txt")
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}
}

func TestFilesystemWrite_ContextCancellation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path": "/tmp/file.txt", "size": 5, 		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := fs.Write(ctx, "/tmp/file.txt", "hello")
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}
}

// ---------- Error type mapping tests ----------

func TestFilesystemRead_AuthenticationError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	_, err := fs.Read(context.Background(), "/tmp/file.txt")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if authErr, ok := err.(*AuthenticationError); ok {
		if authErr.StatusCode != 401 {
			t.Errorf("expected status 401, got %d", authErr.StatusCode)
		}
	}
}

func TestFilesystemWrite_InsufficientStorage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInsufficientStorage)
		w.Write([]byte("disk full"))
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	_, err := fs.Write(context.Background(), "/tmp/file.txt", "data")
	if err == nil {
		t.Fatal("expected error for 507 response")
	}
	if spaceErr, ok := err.(*NotEnoughSpaceError); ok {
		if spaceErr.StatusCode != 507 {
			t.Errorf("expected status 507, got %d", spaceErr.StatusCode)
		}
	}
}

// ---------- Table-driven comprehensive tests ----------

func TestFilesystemOperations_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		operation  string // "read", "exists", "getinfo", "remove", "mkdir"
		path       string
		statusCode int
		response   string
		wantErr    bool
	}{
		{
			name:       "read success",
			operation:  "read",
			path:       "/tmp/test.txt",
			statusCode: 200,
			response:   `{"content": "data"}`,
		},
		{
			name:       "read 404",
			operation:  "read",
			path:       "/tmp/missing.txt",
			statusCode: 404,
			response:   "not found",
			wantErr:    true,
		},
		{
			name:       "read 500",
			operation:  "read",
			path:       "/tmp/error.txt",
			statusCode: 500,
			response:   "server error",
			wantErr:    true,
		},
		{
			name:       "exists true",
			operation:  "exists",
			path:       "/tmp/yes.txt",
			statusCode: 200,
			response:   `{"exists": true}`,
		},
		{
			name:       "exists false",
			operation:  "exists",
			path:       "/tmp/no.txt",
			statusCode: 200,
			response:   `{"exists": false}`,
		},
		{
			name:       "remove success",
			operation:  "remove",
			path:       "/tmp/trash.txt",
			statusCode: 204,
			response:   "",
		},
		{
			name:       "remove 404",
			operation:  "remove",
			path:       "/tmp/missing.txt",
			statusCode: 404,
			response:   "not found",
			wantErr:    true,
		},
		{
			name:       "mkdir success",
			operation:  "mkdir",
			path:       "/tmp/newdir",
			statusCode: 200,
			response:   `{"created": true}`,
		},
		{
			name:       "mkdir 500",
			operation:  "mkdir",
			path:       "/tmp/fail",
			statusCode: 500,
			response:   "error",
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()

			// Register handlers for all endpoints
			mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.statusCode)
				w.Write([]byte(tc.response))
			})
			mux.HandleFunc("/sandboxes/sbx-123/files/exists", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.statusCode)
				w.Write([]byte(tc.response))
			})
			mux.HandleFunc("/sandboxes/sbx-123/files/info", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.statusCode)
				w.Write([]byte(tc.response))
			})
			mux.HandleFunc("/sandboxes/sbx-123/files/mkdir", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.statusCode)
				w.Write([]byte(tc.response))
			})

			fs, server := newTestFilesystemServer(t, mux)
			defer server.Close()

			var err error
			switch tc.operation {
			case "read":
				_, err = fs.Read(context.Background(), tc.path)
			case "exists":
				_, err = fs.Exists(context.Background(), tc.path)
			case "remove":
				err = fs.Remove(context.Background(), tc.path)
			case "mkdir":
				err = fs.MakeDir(context.Background(), tc.path)
			}

			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ---------- WriteEntry model tests ----------

func TestWriteEntry_StringData(t *testing.T) {
	entry := WriteEntry{
		Path: "/a.txt",
		Data: "hello",
	}
	if entry.Path != "/a.txt" {
		t.Errorf("expected path %q, got %q", "/a.txt", entry.Path)
	}
	s, ok := entry.Data.(string)
	if !ok {
		t.Fatalf("expected Data to be string, got %T", entry.Data)
	}
	if s != "hello" {
		t.Errorf("expected data %q, got %q", "hello", s)
	}
}

func TestWriteEntry_ByteData(t *testing.T) {
	data := []byte{0x89, 0x50, 0x4e, 0x47}
	entry := WriteEntry{
		Path: "/b.bin",
		Data: data,
	}
	b, ok := entry.Data.([]byte)
	if !ok {
		t.Fatalf("expected Data to be []byte, got %T", entry.Data)
	}
	if !bytes.Equal(b, data) {
		t.Errorf("expected data %v, got %v", data, b)
	}
}

// ---------- WriteInfo model tests ----------

func TestWriteInfo_Fields(t *testing.T) {
	info := WriteInfo{
		Path: "/tmp/file.txt",
		Size: 42,
	}
	if info.Path != "/tmp/file.txt" {
		t.Errorf("expected path %q, got %q", "/tmp/file.txt", info.Path)
	}
	if info.Size != 42 {
		t.Errorf("expected size 42, got %d", info.Size)
	}
}

// ---------- EntryInfo model tests ----------

func TestEntryInfo_AllFields_Filesystem(t *testing.T) {
	info := EntryInfo{
		Path: "/tmp/file.txt",
		Type: FileTypeFile,
		Size: 100,
	}
	if info.Path != "/tmp/file.txt" {
		t.Errorf("expected path %q, got %q", "/tmp/file.txt", info.Path)
	}
	if info.Type != FileTypeFile {
		t.Errorf("expected type %q, got %q", FileTypeFile, info.Type)
	}
	if info.Size != 100 {
		t.Errorf("expected size 100, got %d", info.Size)
	}
}

// ---------- Concurrent access ----------

func TestFilesystem_ConcurrentReads(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-123/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": "data",
		})
	})

	fs, server := newTestFilesystemServer(t, mux)
	defer server.Close()

	// Launch multiple concurrent reads
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := fs.Read(context.Background(), "/tmp/file.txt")
			errs <- err
		}()
	}

	for i := 0; i < 10; i++ {
		err := <-errs
		if err != nil {
			t.Errorf("concurrent read %d failed: %v", i, err)
		}
	}
}

// ---------- Unicode path tests ----------

func TestFilesystem_UnicodePaths(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"chinese chars", "/tmp/你好.txt"},
		{"emoji", "/tmp/\U0001f600.txt"},
		{"accented chars", "/tmp/café.txt"},
		{"japanese chars", "/tmp/テスト.txt"},
		{"arabic chars", "/tmp/مرحبا.txt"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedPath string

			mux := http.NewServeMux()
			mux.HandleFunc("/sandboxes/sbx-123/files/exists", func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.Query().Get("path")
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{"exists": true})
			})

			fs, server := newTestFilesystemServer(t, mux)
			defer server.Close()

			_, err := fs.Exists(context.Background(), tc.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if capturedPath != tc.path {
				t.Errorf("unicode path not preserved: expected %q, got %q", tc.path, capturedPath)
			}
		})
	}
}
