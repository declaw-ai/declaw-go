package declaw

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// templateTestEnv sets up a test server and configures sandbox options to
// point at it. Returns the server, a cleanup func, and the SandboxOption.
func templateTestEnv(t *testing.T, handler http.Handler) (*httptest.Server, SandboxOption) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts, func(o *sandboxOpts) {
		o.APIKey = "test-key"
		o.APIURL = ts.URL
	}
}

// ---------------------------------------------------------------------------
// BuildTemplate (foreground) tests
// ---------------------------------------------------------------------------

func TestBuildTemplate_Foreground_BasicSpec(t *testing.T) {
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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"build_id": "bld-1", "status": "success", "template_id": "tpl-abc"}`))
	})

	_, opt := templateTestEnv(t, handler)

	spec := TemplateSpec{
		BaseImage: "ubuntu:22.04",
		RunCmds:   []string{"apt-get update"},
	}

	info, err := BuildTemplate(context.Background(), spec, opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/templates/build" {
		t.Errorf("expected path /templates/build, got %s", gotPath)
	}

	// Verify body fields
	if baseImg, _ := gotBody["base_image"].(string); baseImg != "ubuntu:22.04" {
		t.Errorf("expected base_image='ubuntu:22.04', got %q", gotBody["base_image"])
	}
	if runCmds, ok := gotBody["run_cmds"].([]interface{}); ok {
		if len(runCmds) != 1 || runCmds[0] != "apt-get update" {
			t.Errorf("expected run_cmds=['apt-get update'], got %v", runCmds)
		}
	} else {
		t.Errorf("expected run_cmds to be array, got %T", gotBody["run_cmds"])
	}

	// Foreground: background should be false or absent
	if bg, ok := gotBody["background"]; ok {
		if bgBool, ok := bg.(bool); ok && bgBool {
			t.Error("expected background=false for foreground build")
		}
	}

	if info == nil {
		t.Fatal("expected non-nil BuildInfo")
	}
	if info.BuildID != "bld-1" {
		t.Errorf("expected BuildID='bld-1', got %q", info.BuildID)
	}
	if info.Status != "success" {
		t.Errorf("expected Status='success', got %q", info.Status)
	}
	if info.TemplateID != "tpl-abc" {
		t.Errorf("expected TemplateID='tpl-abc', got %q", info.TemplateID)
	}
}

func TestBuildTemplate_Foreground_AllFields(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"build_id": "bld-full", "status": "success", "template_id": "tpl-full"}`))
	})

	_, opt := templateTestEnv(t, handler)

	spec := TemplateSpec{
		BaseImage:   "python:3.12",
		RunCmds:     []string{"pip install numpy", "pip install pandas"},
		Copies:      []CopyItem{{Src: "config.json", Dst: "/app/config.json", Mode: "0644"}},
		Envs:        map[string]string{"APP_ENV": "production", "DEBUG": "false"},
		AptPackages: []string{"curl", "git"},
		StartCmd:    "python main.py",
		DiskMB:      2048,
	}

	info, err := BuildTemplate(context.Background(), spec, opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Verify base_image
	if v, _ := gotBody["base_image"].(string); v != "python:3.12" {
		t.Errorf("expected base_image='python:3.12', got %q", v)
	}

	// Verify run_cmds
	if cmds, ok := gotBody["run_cmds"].([]interface{}); ok {
		if len(cmds) != 2 {
			t.Errorf("expected 2 run_cmds, got %d", len(cmds))
		}
	}

	// Verify copies
	if copies, ok := gotBody["copies"].([]interface{}); ok {
		if len(copies) != 1 {
			t.Errorf("expected 1 copy item, got %d", len(copies))
		}
		if len(copies) > 0 {
			copyMap, ok := copies[0].(map[string]interface{})
			if ok {
				if copyMap["src"] != "config.json" {
					t.Errorf("expected copy src='config.json', got %v", copyMap["src"])
				}
				if copyMap["dst"] != "/app/config.json" {
					t.Errorf("expected copy dst='/app/config.json', got %v", copyMap["dst"])
				}
			}
		}
	}

	// Verify envs
	if envs, ok := gotBody["envs"].(map[string]interface{}); ok {
		if envs["APP_ENV"] != "production" {
			t.Errorf("expected envs.APP_ENV='production', got %v", envs["APP_ENV"])
		}
	}

	// Verify apt_packages
	if pkgs, ok := gotBody["apt_packages"].([]interface{}); ok {
		if len(pkgs) != 2 {
			t.Errorf("expected 2 apt_packages, got %d", len(pkgs))
		}
	}

	// Verify start_cmd
	if v, _ := gotBody["start_cmd"].(string); v != "python main.py" {
		t.Errorf("expected start_cmd='python main.py', got %q", v)
	}

	// Verify disk_mb
	if v, ok := gotBody["disk_mb"].(float64); ok {
		if int(v) != 2048 {
			t.Errorf("expected disk_mb=2048, got %v", v)
		}
	} else {
		t.Errorf("expected disk_mb in body, got %v (%T)", gotBody["disk_mb"], gotBody["disk_mb"])
	}

	if info.BuildID != "bld-full" {
		t.Errorf("expected BuildID='bld-full', got %q", info.BuildID)
	}
}

func TestBuildTemplate_Foreground_DiskMBOmittedWhenZero(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"build_id": "bld-no-disk", "status": "success", "template_id": "tpl-nd"}`))
	})

	_, opt := templateTestEnv(t, handler)

	spec := TemplateSpec{
		BaseImage: "ubuntu:22.04",
		DiskMB:    0, // Zero means omit
	}

	_, err := BuildTemplate(context.Background(), spec, opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// disk_mb should not be present or should be 0
	if v, ok := gotBody["disk_mb"]; ok {
		if diskVal, ok := v.(float64); ok && diskVal != 0 {
			t.Errorf("expected disk_mb to be omitted when zero, but got %v", diskVal)
		}
	}
	// Not having disk_mb at all is the ideal outcome
}

func TestBuildTemplate_Foreground_ServerError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "build failed"}`))
	})

	_, opt := templateTestEnv(t, handler)

	_, err := BuildTemplate(context.Background(), TemplateSpec{BaseImage: "ubuntu:22.04"}, opt)
	if err == nil {
		t.Fatal("expected an error on 500 response")
	}
}

func TestBuildTemplate_Foreground_EmptySpec(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"build_id": "bld-empty", "status": "success", "template_id": "tpl-e"}`))
	})

	_, opt := templateTestEnv(t, handler)

	info, err := BuildTemplate(context.Background(), TemplateSpec{}, opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if info.BuildID != "bld-empty" {
		t.Errorf("expected BuildID='bld-empty', got %q", info.BuildID)
	}
}

// ---------------------------------------------------------------------------
// BuildTemplateBackground tests
// ---------------------------------------------------------------------------

func TestBuildTemplateBackground_SetsBackgroundFlag(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"build_id": "bld-bg", "status": "queued"}`))
	})

	_, opt := templateTestEnv(t, handler)

	spec := TemplateSpec{BaseImage: "ubuntu:22.04"}
	info, err := BuildTemplateBackground(context.Background(), spec, opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	bg, ok := gotBody["background"]
	if !ok {
		t.Fatal("expected 'background' field in request body")
	}
	if bgBool, ok := bg.(bool); !ok || !bgBool {
		t.Errorf("expected background=true, got %v", bg)
	}

	if info.Status != "queued" {
		t.Errorf("expected Status='queued', got %q", info.Status)
	}
}

func TestBuildTemplateBackground_WithDiskMB(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"build_id": "bld-bg-disk", "status": "queued"}`))
	})

	_, opt := templateTestEnv(t, handler)

	spec := TemplateSpec{BaseImage: "ubuntu:22.04", DiskMB: 4096}
	_, err := BuildTemplateBackground(context.Background(), spec, opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if v, ok := gotBody["disk_mb"].(float64); ok {
		if int(v) != 4096 {
			t.Errorf("expected disk_mb=4096, got %v", v)
		}
	} else {
		t.Error("expected disk_mb in background build body")
	}
	if bg, ok := gotBody["background"].(bool); !ok || !bg {
		t.Error("expected background=true in background build")
	}
}

// ---------------------------------------------------------------------------
// GetBuildStatus tests
// ---------------------------------------------------------------------------

func TestGetBuildStatus_Success(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"build_id": "bld-1", "status": "building"}`))
	})

	_, opt := templateTestEnv(t, handler)

	info, err := GetBuildStatus(context.Background(), "bld-1", opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/templates/builds/bld-1" {
		t.Errorf("expected path /templates/builds/bld-1, got %s", gotPath)
	}
	if info.BuildID != "bld-1" {
		t.Errorf("expected BuildID='bld-1', got %q", info.BuildID)
	}
	if info.Status != "building" {
		t.Errorf("expected Status='building', got %q", info.Status)
	}
}

func TestGetBuildStatus_NotFound(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "build not found"}`))
	})

	_, opt := templateTestEnv(t, handler)

	_, err := GetBuildStatus(context.Background(), "bld-missing", opt)
	if err == nil {
		t.Fatal("expected an error on 404 response")
	}
}

func TestGetBuildStatus_CompletedWithTemplate(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"build_id": "bld-done", "status": "success", "template_id": "tpl-xyz"}`))
	})

	_, opt := templateTestEnv(t, handler)

	info, err := GetBuildStatus(context.Background(), "bld-done", opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if info.TemplateID != "tpl-xyz" {
		t.Errorf("expected TemplateID='tpl-xyz', got %q", info.TemplateID)
	}
}

// ---------------------------------------------------------------------------
// ListTemplates tests
// ---------------------------------------------------------------------------

func TestListTemplates_Success(t *testing.T) {
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
		_, _ = w.Write([]byte(`[
			{"template_id": "tpl-1", "alias": "python", "created_at": "2026-01-01T00:00:00Z"},
			{"template_id": "tpl-2", "alias": "node", "created_at": "2026-01-02T00:00:00Z"}
		]`))
	})

	_, opt := templateTestEnv(t, handler)

	templates, err := ListTemplates(context.Background(), opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/templates" {
		t.Errorf("expected path /templates, got %s", gotPath)
	}

	if len(templates) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(templates))
	}
	if templates[0].TemplateID != "tpl-1" {
		t.Errorf("expected first template ID 'tpl-1', got %q", templates[0].TemplateID)
	}
	if templates[0].Alias != "python" {
		t.Errorf("expected first template alias 'python', got %q", templates[0].Alias)
	}
	if templates[1].TemplateID != "tpl-2" {
		t.Errorf("expected second template ID 'tpl-2', got %q", templates[1].TemplateID)
	}
}

func TestListTemplates_EmptyList(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})

	_, opt := templateTestEnv(t, handler)

	templates, err := ListTemplates(context.Background(), opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(templates) != 0 {
		t.Errorf("expected 0 templates, got %d", len(templates))
	}
}

func TestListTemplates_ServerError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal"}`))
	})

	_, opt := templateTestEnv(t, handler)

	_, err := ListTemplates(context.Background(), opt)
	if err == nil {
		t.Fatal("expected an error on 500 response")
	}
}

// ---------------------------------------------------------------------------
// GetTemplate tests
// ---------------------------------------------------------------------------

func TestGetTemplate_Success(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"template_id": "tpl-abc", "alias": "python", "created_at": "2026-01-01T00:00:00Z"}`))
	})

	_, opt := templateTestEnv(t, handler)

	info, err := GetTemplate(context.Background(), "tpl-abc", opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/templates/tpl-abc" {
		t.Errorf("expected path /templates/tpl-abc, got %s", gotPath)
	}
	if info.TemplateID != "tpl-abc" {
		t.Errorf("expected TemplateID='tpl-abc', got %q", info.TemplateID)
	}
	if info.Alias != "python" {
		t.Errorf("expected Alias='python', got %q", info.Alias)
	}
}

func TestGetTemplate_NotFound(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "template not found"}`))
	})

	_, opt := templateTestEnv(t, handler)

	_, err := GetTemplate(context.Background(), "tpl-missing", opt)
	if err == nil {
		t.Fatal("expected an error on 404 response")
	}
}

// ---------------------------------------------------------------------------
// DeleteTemplate tests
// ---------------------------------------------------------------------------

func TestDeleteTemplate_Success(t *testing.T) {
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

	_, opt := templateTestEnv(t, handler)

	err := DeleteTemplate(context.Background(), "tpl-abc", opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
	if gotPath != "/templates/tpl-abc" {
		t.Errorf("expected path /templates/tpl-abc, got %s", gotPath)
	}
}

func TestDeleteTemplate_NotFound(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "not found"}`))
	})

	_, opt := templateTestEnv(t, handler)

	err := DeleteTemplate(context.Background(), "tpl-gone", opt)
	if err == nil {
		t.Fatal("expected an error on 404 response")
	}
}

func TestDeleteTemplate_AuthError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
	})

	_, opt := templateTestEnv(t, handler)

	err := DeleteTemplate(context.Background(), "tpl-unauth", opt)
	if err == nil {
		t.Fatal("expected an error on 401 response")
	}
}

// ---------------------------------------------------------------------------
// Template with Dockerfile field
// ---------------------------------------------------------------------------

func TestBuildTemplate_Foreground_Dockerfile(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"build_id": "bld-df", "status": "success", "template_id": "tpl-df"}`))
	})

	_, opt := templateTestEnv(t, handler)

	spec := TemplateSpec{
		Dockerfile: "FROM ubuntu:22.04\nRUN apt-get update\n",
	}

	_, err := BuildTemplate(context.Background(), spec, opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if v, _ := gotBody["dockerfile"].(string); v != "FROM ubuntu:22.04\nRUN apt-get update\n" {
		t.Errorf("expected dockerfile field, got %q", v)
	}
}

// ---------------------------------------------------------------------------
// ContextCancellation for template operations
// ---------------------------------------------------------------------------

func TestBuildTemplate_ContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"build_id": "bld-x", "status": "success"}`))
	})

	_, opt := templateTestEnv(t, handler)

	_, err := BuildTemplate(ctx, TemplateSpec{BaseImage: "ubuntu:22.04"}, opt)
	if err == nil {
		t.Fatal("expected an error when context is already canceled")
	}
}

func TestListTemplates_ContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})

	_, opt := templateTestEnv(t, handler)

	_, err := ListTemplates(ctx, opt)
	if err == nil {
		t.Fatal("expected an error when context is already canceled")
	}
}
