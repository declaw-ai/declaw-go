package declaw

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeGzipData returns a minimal gzip-compressed payload for testing.
func makeGzipData(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(content)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func volumeTestEnv(t *testing.T, handler http.Handler) (*httptest.Server, SandboxOption) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts, func(o *sandboxOpts) {
		o.APIKey = "test-key"
		o.APIURL = ts.URL
	}
}

// ---------------------------------------------------------------------------
// CreateVolume tests
// ---------------------------------------------------------------------------

func TestCreateVolume_Success(t *testing.T) {
	t.Parallel()

	var (
		mu        sync.Mutex
		gotMethod string
		gotPath   string
		gotQuery  string
		rawBody   []byte
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("name")
		rawBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"volume_id": "vol-123",
			"owner_id": "owner-1",
			"name": "my-volume",
			"blob_key": "bk-1",
			"size_bytes": 1024,
			"content_type": "application/octet-stream",
			"created_at": "2026-01-01T00:00:00Z"
		}`))
	})

	_, opt := volumeTestEnv(t, handler)

	gzData := makeGzipData(t, "file-content")
	vol, err := CreateVolume(context.Background(), "my-volume", gzData, opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/volumes" {
		t.Errorf("expected path /volumes, got %s", gotPath)
	}
	if gotQuery != "my-volume" {
		t.Errorf("expected query name='my-volume', got %q", gotQuery)
	}

	// Body should be the raw gzip bytes
	if !bytes.Equal(rawBody, gzData) {
		t.Errorf("expected raw gzip body, got %d bytes", len(rawBody))
	}

	if vol == nil {
		t.Fatal("expected non-nil VolumeInfo")
	}
	if vol.VolumeID != "vol-123" {
		t.Errorf("expected VolumeID='vol-123', got %q", vol.VolumeID)
	}
	if vol.Name != "my-volume" {
		t.Errorf("expected Name='my-volume', got %q", vol.Name)
	}
	if vol.SizeBytes != 1024 {
		t.Errorf("expected SizeBytes=1024, got %d", vol.SizeBytes)
	}
}

func TestCreateVolume_EmptyData(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"volume_id": "vol-empty",
			"owner_id": "owner-1",
			"name": "empty-vol",
			"size_bytes": 0,
			"created_at": "2026-01-01T00:00:00Z"
		}`))
	})

	_, opt := volumeTestEnv(t, handler)

	vol, err := CreateVolume(context.Background(), "empty-vol", []byte{}, opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if vol.SizeBytes != 0 {
		t.Errorf("expected SizeBytes=0, got %d", vol.SizeBytes)
	}
}

func TestCreateVolume_NilData(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"volume_id": "vol-nil",
			"owner_id": "owner-1",
			"name": "nil-vol",
			"size_bytes": 0,
			"created_at": "2026-01-01T00:00:00Z"
		}`))
	})

	_, opt := volumeTestEnv(t, handler)

	vol, err := CreateVolume(context.Background(), "nil-vol", nil, opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if vol.VolumeID != "vol-nil" {
		t.Errorf("expected VolumeID='vol-nil', got %q", vol.VolumeID)
	}
}

func TestCreateVolume_LargeData(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"volume_id": "vol-large",
			"owner_id": "owner-1",
			"name": "big-vol",
			"size_bytes": 10485760,
			"created_at": "2026-01-01T00:00:00Z"
		}`))
	})

	_, opt := volumeTestEnv(t, handler)

	// Create gzip data from a large payload
	gzData := makeGzipData(t, string(make([]byte, 10*1024*1024)))
	vol, err := CreateVolume(context.Background(), "big-vol", gzData, opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if vol.VolumeID != "vol-large" {
		t.Errorf("expected VolumeID='vol-large', got %q", vol.VolumeID)
	}
}

func TestCreateVolume_ServerError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "storage unavailable"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	gzData := makeGzipData(t, "data")
	_, err := CreateVolume(context.Background(), "fail-vol", gzData, opt)
	if err == nil {
		t.Fatal("expected an error on 500 response")
	}
}

func TestCreateVolume_InsufficientStorage(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInsufficientStorage)
		_, _ = w.Write([]byte(`{"error": "not enough space"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	gzData := makeGzipData(t, "data")
	_, err := CreateVolume(context.Background(), "full-vol", gzData, opt)
	if err == nil {
		t.Fatal("expected an error on 507 response")
	}
}

func TestCreateVolume_RejectsNonGzipData(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called for non-gzip data")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"volume_id":"vol-x"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	_, err := CreateVolume(context.Background(), "bad-vol", []byte("not-gzip"), opt)
	if err == nil {
		t.Fatal("expected an error for non-gzip data")
	}
	if !strings.Contains(err.Error(), "gzip") {
		t.Errorf("expected gzip error message, got: %v", err)
	}
}

func TestCreateVolume_ContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"volume_id":"vol-x"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	gzData := makeGzipData(t, "data")
	_, err := CreateVolume(ctx, "ctx-vol", gzData, opt)
	if err == nil {
		t.Fatal("expected an error when context is already canceled")
	}
}

// ---------------------------------------------------------------------------
// ListVolumes tests
// ---------------------------------------------------------------------------

func TestListVolumes_Success(t *testing.T) {
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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"volumes": [
			{"volume_id": "vol-1", "name": "data-1", "size_bytes": 100, "created_at": "2026-01-01T00:00:00Z"},
			{"volume_id": "vol-2", "name": "data-2", "size_bytes": 200, "created_at": "2026-01-02T00:00:00Z"}
		]}`))
	})

	_, opt := volumeTestEnv(t, handler)

	volumes, err := ListVolumes(context.Background(), opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/volumes" {
		t.Errorf("expected path /volumes, got %s", gotPath)
	}

	if len(volumes) != 2 {
		t.Fatalf("expected 2 volumes, got %d", len(volumes))
	}
	if volumes[0].VolumeID != "vol-1" {
		t.Errorf("expected first volume ID 'vol-1', got %q", volumes[0].VolumeID)
	}
	if volumes[1].VolumeID != "vol-2" {
		t.Errorf("expected second volume ID 'vol-2', got %q", volumes[1].VolumeID)
	}
}

func TestListVolumes_EmptyList(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"volumes": []}`))
	})

	_, opt := volumeTestEnv(t, handler)

	volumes, err := ListVolumes(context.Background(), opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(volumes) != 0 {
		t.Errorf("expected 0 volumes, got %d", len(volumes))
	}
}

func TestListVolumes_ServerError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, opt := volumeTestEnv(t, handler)

	_, err := ListVolumes(context.Background(), opt)
	if err == nil {
		t.Fatal("expected an error on 500 response")
	}
}

func TestListVolumes_AuthenticationError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "invalid api key"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	_, err := ListVolumes(context.Background(), opt)
	if err == nil {
		t.Fatal("expected an error on 401 response")
	}
}

// ---------------------------------------------------------------------------
// GetVolume tests
// ---------------------------------------------------------------------------

func TestGetVolume_Success(t *testing.T) {
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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"volume_id": "vol-123",
			"owner_id": "owner-1",
			"name": "my-volume",
			"blob_key": "bk-1",
			"size_bytes": 4096,
			"content_type": "application/gzip",
			"created_at": "2026-01-15T12:00:00Z",
			"metadata": {"env": "prod"}
		}`))
	})

	_, opt := volumeTestEnv(t, handler)

	vol, err := GetVolume(context.Background(), "vol-123", opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/volumes/vol-123" {
		t.Errorf("expected path /volumes/vol-123, got %s", gotPath)
	}
	if vol.VolumeID != "vol-123" {
		t.Errorf("expected VolumeID='vol-123', got %q", vol.VolumeID)
	}
	if vol.Name != "my-volume" {
		t.Errorf("expected Name='my-volume', got %q", vol.Name)
	}
	if vol.SizeBytes != 4096 {
		t.Errorf("expected SizeBytes=4096, got %d", vol.SizeBytes)
	}
	if vol.ContentType != "application/gzip" {
		t.Errorf("expected ContentType='application/gzip', got %q", vol.ContentType)
	}
	if vol.Metadata == nil || vol.Metadata["env"] != "prod" {
		t.Errorf("expected metadata env=prod, got %v", vol.Metadata)
	}
}

func TestGetVolume_NotFound(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "volume not found"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	_, err := GetVolume(context.Background(), "vol-missing", opt)
	if err == nil {
		t.Fatal("expected an error on 404 response")
	}
}

func TestGetVolume_EmptyID(t *testing.T) {
	t.Parallel()

	var gotPath string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid volume id"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	_, err := GetVolume(context.Background(), "", opt)
	// Sending empty ID: should either fail locally or get a bad request
	_ = err
	_ = gotPath
}

// ---------------------------------------------------------------------------
// DownloadVolume tests
// ---------------------------------------------------------------------------

func TestDownloadVolume_Success(t *testing.T) {
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

		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("raw-volume-data-here"))
	})

	_, opt := volumeTestEnv(t, handler)

	data, err := DownloadVolume(context.Background(), "vol-123", opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/volumes/vol-123/download" {
		t.Errorf("expected path /volumes/vol-123/download, got %s", gotPath)
	}
	if string(data) != "raw-volume-data-here" {
		t.Errorf("expected raw data 'raw-volume-data-here', got %q", string(data))
	}
}

func TestDownloadVolume_EmptyContent(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		// Empty body
	})

	_, opt := volumeTestEnv(t, handler)

	data, err := DownloadVolume(context.Background(), "vol-empty", opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty data, got %d bytes", len(data))
	}
}

func TestDownloadVolume_BinaryContent(t *testing.T) {
	t.Parallel()

	binaryData := []byte{0x00, 0x01, 0xFF, 0xFE, 0x89, 0x50, 0x4E, 0x47}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(binaryData)
	})

	_, opt := volumeTestEnv(t, handler)

	data, err := DownloadVolume(context.Background(), "vol-bin", opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(data) != len(binaryData) {
		t.Fatalf("expected %d bytes, got %d", len(binaryData), len(data))
	}
	for i, b := range binaryData {
		if data[i] != b {
			t.Errorf("byte %d: expected 0x%02x, got 0x%02x", i, b, data[i])
		}
	}
}

func TestDownloadVolume_NotFound(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "volume not found"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	_, err := DownloadVolume(context.Background(), "vol-gone", opt)
	if err == nil {
		t.Fatal("expected an error on 404 response")
	}
}

func TestDownloadVolume_ContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	})

	_, opt := volumeTestEnv(t, handler)

	_, err := DownloadVolume(ctx, "vol-ctx", opt)
	if err == nil {
		t.Fatal("expected an error when context is already canceled")
	}
}

// ---------------------------------------------------------------------------
// DeleteVolume tests
// ---------------------------------------------------------------------------

func TestDeleteVolume_Success(t *testing.T) {
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

	_, opt := volumeTestEnv(t, handler)

	err := DeleteVolume(context.Background(), "vol-123", opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
	if gotPath != "/volumes/vol-123" {
		t.Errorf("expected path /volumes/vol-123, got %s", gotPath)
	}
}

func TestDeleteVolume_NotFound(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "not found"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	err := DeleteVolume(context.Background(), "vol-gone", opt)
	if err == nil {
		t.Fatal("expected an error on 404 response")
	}
}

func TestDeleteVolume_AuthenticationError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error": "forbidden"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	err := DeleteVolume(context.Background(), "vol-unauth", opt)
	if err == nil {
		t.Fatal("expected an error on 403 response")
	}
}

func TestDeleteVolume_ContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	_, opt := volumeTestEnv(t, handler)

	err := DeleteVolume(ctx, "vol-ctx", opt)
	if err == nil {
		t.Fatal("expected an error when context is already canceled")
	}
}

// ---------------------------------------------------------------------------
// VolumeInfo JSON round-trip test (field tests are in models_test.go)
// ---------------------------------------------------------------------------

func TestVolumeInfo_JSONSerialization(t *testing.T) {
	t.Parallel()

	vi := VolumeInfo{
		VolumeID:  "vol-json",
		Name:      "json-vol",
		SizeBytes: 42,
	}

	data, err := json.Marshal(vi)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var vi2 VolumeInfo
	if err := json.Unmarshal(data, &vi2); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if vi2.VolumeID != vi.VolumeID {
		t.Errorf("VolumeID mismatch after roundtrip: %q vs %q", vi2.VolumeID, vi.VolumeID)
	}
	if vi2.Name != vi.Name {
		t.Errorf("Name mismatch after roundtrip: %q vs %q", vi2.Name, vi.Name)
	}
	if vi2.SizeBytes != vi.SizeBytes {
		t.Errorf("SizeBytes mismatch after roundtrip: %d vs %d", vi2.SizeBytes, vi.SizeBytes)
	}
}
