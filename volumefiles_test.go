package declaw

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// VolumeFiles.Write
// ---------------------------------------------------------------------------

func TestVolumeFiles_Write_Unconditional(t *testing.T) {
	t.Parallel()

	var (
		mu        sync.Mutex
		gotMethod string
		gotPath   string
		gotPathQ  string
		hasIf     bool
		gotCT     string
		rawBody   []byte
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotPathQ = r.URL.Query().Get("path")
		_, hasIf = r.URL.Query()["if_version"]
		gotCT = r.Header.Get("Content-Type")
		rawBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"path":"/a.txt"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	err := VolumeFilesFor("vol-1", opt).Write(context.Background(), "/a.txt", []byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotMethod != http.MethodPut {
		t.Errorf("expected PUT, got %s", gotMethod)
	}
	if gotPath != "/volumes/vol-1/files/raw" {
		t.Errorf("expected /volumes/vol-1/files/raw, got %s", gotPath)
	}
	if gotPathQ != "/a.txt" {
		t.Errorf("expected path query=/a.txt, got %q", gotPathQ)
	}
	if hasIf {
		t.Error("expected no if_version query for unconditional write")
	}
	if gotCT != "application/octet-stream" {
		t.Errorf("expected octet-stream content type, got %q", gotCT)
	}
	if string(rawBody) != "hello" {
		t.Errorf("expected body 'hello', got %q", string(rawBody))
	}
}

func TestVolumeFiles_Write_WithIfVersion(t *testing.T) {
	t.Parallel()

	var (
		mu     sync.Mutex
		gotVer string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotVer = r.URL.Query().Get("if_version")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"path":"/a.txt"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	err := VolumeFilesFor("vol-1", opt).Write(context.Background(), "/a.txt", []byte("x"), WithIfVersion("v7"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotVer != "v7" {
		t.Errorf("expected if_version=v7, got %q", gotVer)
	}
}

func TestVolumeFiles_Write_VersionMismatch(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"version mismatch"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	err := VolumeFilesFor("vol-1", opt).Write(context.Background(), "/a.txt", []byte("x"), WithIfVersion("stale"))
	if err == nil {
		t.Fatal("expected an error on 409 response")
	}
	if _, ok := err.(*VersionMismatchError); !ok {
		t.Errorf("expected *VersionMismatchError, got %T", err)
	}
}

func TestVolumeFiles_Write_ConflictWithoutIfVersion(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"volume is tarball-backed"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	err := VolumeFilesFor("vol-1", opt).Write(context.Background(), "/a.txt", []byte("x"))
	if err == nil {
		t.Fatal("expected an error on 409 response")
	}
	// Without if_version this stays a plain ConflictError, not VersionMismatch.
	if _, ok := err.(*VersionMismatchError); ok {
		t.Error("did not expect VersionMismatchError without if_version")
	}
	if _, ok := err.(*ConflictError); !ok {
		t.Errorf("expected *ConflictError, got %T", err)
	}
}

func TestVolumeFiles_Write_QuotaExceeded(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	})

	_, opt := volumeTestEnv(t, handler)

	err := VolumeFilesFor("vol-1", opt).Write(context.Background(), "/a.txt", []byte("x"))
	if _, ok := err.(*NotEnoughSpaceError); !ok {
		t.Errorf("expected *NotEnoughSpaceError, got %T", err)
	}
}

// ---------------------------------------------------------------------------
// VolumeFiles.Read
// ---------------------------------------------------------------------------

func TestVolumeFiles_Read(t *testing.T) {
	t.Parallel()

	var (
		mu       sync.Mutex
		gotPath  string
		gotPathQ string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotPath = r.URL.Path
		gotPathQ = r.URL.Query().Get("path")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("file-bytes"))
	})

	_, opt := volumeTestEnv(t, handler)

	data, err := VolumeFilesFor("vol-1", opt).Read(context.Background(), "/a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/volumes/vol-1/files/raw" {
		t.Errorf("expected /volumes/vol-1/files/raw, got %s", gotPath)
	}
	if gotPathQ != "/a.txt" {
		t.Errorf("expected path=/a.txt, got %q", gotPathQ)
	}
	if string(data) != "file-bytes" {
		t.Errorf("expected 'file-bytes', got %q", string(data))
	}
}

// ---------------------------------------------------------------------------
// VolumeFiles.List
// ---------------------------------------------------------------------------

func TestVolumeFiles_List(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		gotPath string
		gotDir  string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotPath = r.URL.Path
		gotDir = r.URL.Query().Get("path")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"entries":[
			{"name":"a.txt","path":"/a.txt","is_dir":false,"size":10,"mod_time":"2026-01-01T00:00:00Z","mode":420},
			{"name":"sub","path":"/sub","is_dir":true,"size":0,"mod_time":"2026-01-01T00:00:00Z","mode":493}
		]}`))
	})

	_, opt := volumeTestEnv(t, handler)

	entries, err := VolumeFilesFor("vol-1", opt).List(context.Background(), "/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/volumes/vol-1/files/list" {
		t.Errorf("expected /volumes/vol-1/files/list, got %s", gotPath)
	}
	if gotDir != "/" {
		t.Errorf("expected path=/, got %q", gotDir)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "a.txt" || entries[0].IsDir || entries[0].Size != 10 {
		t.Errorf("unexpected first entry: %+v", entries[0])
	}
	if !entries[1].IsDir {
		t.Errorf("expected second entry to be a dir: %+v", entries[1])
	}
}

// ---------------------------------------------------------------------------
// VolumeFiles.Info
// ---------------------------------------------------------------------------

func TestVolumeFiles_Info(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		gotPath string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"a.txt","path":"/a.txt","is_dir":false,"size":5,"mod_time":"2026-01-01T00:00:00Z","mode":420,"version":"v42"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	info, err := VolumeFilesFor("vol-1", opt).Info(context.Background(), "/a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/volumes/vol-1/files/info" {
		t.Errorf("expected /volumes/vol-1/files/info, got %s", gotPath)
	}
	if info.Version != "v42" {
		t.Errorf("expected version=v42, got %q", info.Version)
	}
	if info.Name != "a.txt" || info.Size != 5 {
		t.Errorf("unexpected embedded entry: %+v", info.VolumeFileEntry)
	}
}

// ---------------------------------------------------------------------------
// VolumeFiles.Exists
// ---------------------------------------------------------------------------

func TestVolumeFiles_Exists(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		gotPath string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"exists":true}`))
	})

	_, opt := volumeTestEnv(t, handler)

	exists, err := VolumeFilesFor("vol-1", opt).Exists(context.Background(), "/a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/volumes/vol-1/files/exists" {
		t.Errorf("expected /volumes/vol-1/files/exists, got %s", gotPath)
	}
	if !exists {
		t.Error("expected exists=true")
	}
}

// ---------------------------------------------------------------------------
// VolumeFiles.Remove
// ---------------------------------------------------------------------------

func TestVolumeFiles_Remove(t *testing.T) {
	t.Parallel()

	var (
		mu          sync.Mutex
		gotMethod   string
		gotPath     string
		gotPathQ    string
		gotRecursiv string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotPathQ = r.URL.Query().Get("path")
		gotRecursiv = r.URL.Query().Get("recursive")
		w.WriteHeader(http.StatusNoContent)
	})

	_, opt := volumeTestEnv(t, handler)

	if err := VolumeFilesFor("vol-1", opt).Remove(context.Background(), "/dir", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
	if gotPath != "/volumes/vol-1/files" {
		t.Errorf("expected /volumes/vol-1/files, got %s", gotPath)
	}
	if gotPathQ != "/dir" {
		t.Errorf("expected path=/dir, got %q", gotPathQ)
	}
	if gotRecursiv != "true" {
		t.Errorf("expected recursive=true, got %q", gotRecursiv)
	}
}

// ---------------------------------------------------------------------------
// VolumeFiles.Rename
// ---------------------------------------------------------------------------

func TestVolumeFiles_Rename(t *testing.T) {
	t.Parallel()

	var (
		mu        sync.Mutex
		gotMethod string
		gotPath   string
		gotBody   map[string]string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotMethod = r.Method
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"old_path":"/a","new_path":"/b"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	if err := VolumeFilesFor("vol-1", opt).Rename(context.Background(), "/a", "/b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotMethod != http.MethodPatch {
		t.Errorf("expected PATCH, got %s", gotMethod)
	}
	if gotPath != "/volumes/vol-1/files" {
		t.Errorf("expected /volumes/vol-1/files, got %s", gotPath)
	}
	if gotBody["old_path"] != "/a" || gotBody["new_path"] != "/b" {
		t.Errorf("unexpected body: %v", gotBody)
	}
}

// ---------------------------------------------------------------------------
// VolumeFiles.Mkdir
// ---------------------------------------------------------------------------

func TestVolumeFiles_Mkdir(t *testing.T) {
	t.Parallel()

	var (
		mu        sync.Mutex
		gotMethod string
		gotPath   string
		gotBody   map[string]string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotMethod = r.Method
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"path":"/newdir"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	if err := VolumeFilesFor("vol-1", opt).Mkdir(context.Background(), "/newdir"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/volumes/vol-1/files/mkdir" {
		t.Errorf("expected /volumes/vol-1/files/mkdir, got %s", gotPath)
	}
	if gotBody["path"] != "/newdir" {
		t.Errorf("unexpected body: %v", gotBody)
	}
}

// ---------------------------------------------------------------------------
// VolumeLocks
// ---------------------------------------------------------------------------

func TestVolumeLocks_Acquire(t *testing.T) {
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
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token":"tok-1","ttl_seconds":30,"expires_at":"2026-01-01T00:00:30Z"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	lock, err := VolumeLocksFor("vol-1", opt).Acquire(context.Background(), "/a.txt", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/volumes/vol-1/locks" {
		t.Errorf("expected /volumes/vol-1/locks, got %s", gotPath)
	}
	if gotBody["path"] != "/a.txt" {
		t.Errorf("expected path=/a.txt, got %v", gotBody["path"])
	}
	if gotBody["ttl_seconds"].(float64) != 30 {
		t.Errorf("expected ttl_seconds=30, got %v", gotBody["ttl_seconds"])
	}
	if lock.Token != "tok-1" || lock.TTLSeconds != 30 {
		t.Errorf("unexpected lock: %+v", lock)
	}
}

func TestVolumeLocks_Acquire_NoTTL(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		hasTTL  bool
		gotBody map[string]interface{}
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, hasTTL = gotBody["ttl_seconds"]
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token":"t","ttl_seconds":60,"expires_at":"2026-01-01T00:01:00Z"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	if _, err := VolumeLocksFor("vol-1", opt).Acquire(context.Background(), "/a", 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if hasTTL {
		t.Error("expected no ttl_seconds in body when ttl <= 0")
	}
}

func TestVolumeLocks_Acquire_Conflict(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"already locked"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	_, err := VolumeLocksFor("vol-1", opt).Acquire(context.Background(), "/a", 10)
	if err == nil {
		t.Fatal("expected an error on 409 response")
	}
	if _, ok := err.(*ConflictError); !ok {
		t.Errorf("expected *ConflictError, got %T", err)
	}
}

func TestVolumeLocks_Release(t *testing.T) {
	t.Parallel()

	var (
		mu        sync.Mutex
		gotMethod string
		gotPath   string
		gotCT     string
		gotBody   map[string]string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"released":true}`))
	})

	_, opt := volumeTestEnv(t, handler)

	released, err := VolumeLocksFor("vol-1", opt).Release(context.Background(), "/a", "tok-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
	if gotPath != "/volumes/vol-1/locks" {
		t.Errorf("expected /volumes/vol-1/locks, got %s", gotPath)
	}
	if gotCT != "application/json" {
		t.Errorf("expected DELETE body content type application/json, got %q", gotCT)
	}
	if gotBody["path"] != "/a" || gotBody["token"] != "tok-1" {
		t.Errorf("unexpected body: %v", gotBody)
	}
	if !released {
		t.Error("expected released=true")
	}
}

func TestVolumeLocks_Renew(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		gotPath string
		gotBody map[string]interface{}
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token":"tok-1","ttl_seconds":30,"expires_at":"2026-01-01T00:00:30Z"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	if err := VolumeLocksFor("vol-1", opt).Renew(context.Background(), "/a", "tok-1", 30); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/volumes/vol-1/locks/renew" {
		t.Errorf("expected /volumes/vol-1/locks/renew, got %s", gotPath)
	}
	if gotBody["token"] != "tok-1" {
		t.Errorf("expected token=tok-1, got %v", gotBody["token"])
	}
}

func TestVolumeLocks_Renew_NotHolder(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"not the holder"}`))
	})

	_, opt := volumeTestEnv(t, handler)

	err := VolumeLocksFor("vol-1", opt).Renew(context.Background(), "/a", "wrong", 30)
	if _, ok := err.(*ConflictError); !ok {
		t.Errorf("expected *ConflictError, got %T", err)
	}
}

func TestVolumeLocks_Status(t *testing.T) {
	t.Parallel()

	var (
		mu       sync.Mutex
		gotPath  string
		gotPathQ string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotPath = r.URL.Path
		gotPathQ = r.URL.Query().Get("path")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"held":true,"expires_in_ms":12000}`))
	})

	_, opt := volumeTestEnv(t, handler)

	status, err := VolumeLocksFor("vol-1", opt).Status(context.Background(), "/a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/volumes/vol-1/locks" {
		t.Errorf("expected /volumes/vol-1/locks, got %s", gotPath)
	}
	if gotPathQ != "/a" {
		t.Errorf("expected path=/a, got %q", gotPathQ)
	}
	if !status.Held || status.ExpiresInMs != 12000 {
		t.Errorf("unexpected status: %+v", status)
	}
}
