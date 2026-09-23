package declaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
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
// Build tests
// ---------------------------------------------------------------------------

func init() {
	// Real builds are polled every few seconds and ride out two minutes of
	// failed status checks; the tests only exercise the loop.
	buildPollInterval = time.Millisecond
	buildPollErrorWindow = 50 * time.Millisecond
}

// buildServer fakes the two build endpoints the way sandbox-manager answers
// them. POST /templates/build records the body and answers 201 "building";
// each GET /templates/builds/bld-1 answers the next entry of statuses, and the
// last entry repeats.
type buildServer struct {
	mu       sync.Mutex
	statuses []string
	body     map[string]interface{}
	posts    int
	polls    int
}

func (s *buildServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/templates/build":
		s.posts++
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &s.body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"build_id": "bld-1", "status": "building", "template_id": "tpl-1"}`))
	case r.Method == http.MethodGet && r.URL.Path == "/templates/builds/bld-1":
		i := s.polls
		if i >= len(s.statuses) {
			i = len(s.statuses) - 1
		}
		s.polls++
		_, _ = w.Write([]byte(s.statuses[i]))
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "not found"}`))
	}
}

func (s *buildServer) counts() (posts, polls int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.posts, s.polls
}

const (
	statusBuilding  = `{"build_id": "bld-1", "status": "building", "template_id": "tpl-1", "logs": ["Step 1/3"]}`
	statusCompleted = `{"build_id": "bld-1", "status": "completed", "template_id": "tpl-1", "logs": ["Step 1/3", "Step 2/3", "Step 3/3"]}`
)

// The server binds {"template": {...}, "alias": ...} and silently ignores any
// other shape or field name, so the body must match its contract exactly —
// "contains the fields" is how the flat body, apt_packages and background all
// passed before.
func TestBuildTemplate_RequestMatchesServerContract(t *testing.T) {
	t.Parallel()

	srv := &buildServer{statuses: []string{statusCompleted}}
	_, opt := templateTestEnv(t, srv)

	spec := TemplateSpec{
		Alias:       "my-tpl",
		BaseImage:   "python:3.12",
		RunCmds:     []string{"pip install numpy", "pip install pandas"},
		Envs:        map[string]string{"APP_ENV": "production"},
		AptPackages: []string{"curl", "git"},
		StartCmd:    "python main.py",
		DiskMB:      2048,
	}
	if _, err := BuildTemplate(context.Background(), spec, opt); err != nil {
		t.Fatalf("BuildTemplate: %v", err)
	}

	want := map[string]interface{}{
		"alias":   "my-tpl",
		"disk_mb": float64(2048),
		"template": map[string]interface{}{
			"base_image": "python:3.12",
			"run_cmds":   []interface{}{"pip install numpy", "pip install pandas"},
			"envs":       map[string]interface{}{"APP_ENV": "production"},
			"packages":   []interface{}{"curl", "git"},
			"start_cmd":  "python main.py",
		},
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if !reflect.DeepEqual(srv.body, want) {
		t.Errorf("request body mismatch\n got: %#v\nwant: %#v", srv.body, want)
	}
}

func TestBuildTemplate_Dockerfile(t *testing.T) {
	t.Parallel()

	srv := &buildServer{statuses: []string{statusCompleted}}
	_, opt := templateTestEnv(t, srv)

	spec := TemplateSpec{Alias: "df-tpl", Dockerfile: "FROM ubuntu:22.04\nRUN apt-get update\n"}
	if _, err := BuildTemplate(context.Background(), spec, opt); err != nil {
		t.Fatalf("BuildTemplate: %v", err)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	want := map[string]interface{}{"dockerfile": "FROM ubuntu:22.04\nRUN apt-get update\n"}
	if !reflect.DeepEqual(srv.body["template"], want) {
		t.Errorf("template = %#v, want %#v", srv.body["template"], want)
	}
}

func TestBuildTemplate_DiskMBOmittedWhenZero(t *testing.T) {
	t.Parallel()

	srv := &buildServer{statuses: []string{statusCompleted}}
	_, opt := templateTestEnv(t, srv)

	if _, err := BuildTemplate(context.Background(), TemplateSpec{Alias: "a", BaseImage: "ubuntu:22.04"}, opt); err != nil {
		t.Fatalf("BuildTemplate: %v", err)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if v, ok := srv.body["disk_mb"]; ok {
		t.Errorf("disk_mb = %v, want it omitted when zero", v)
	}
}

func TestBuildTemplate_WaitsForCompletion(t *testing.T) {
	t.Parallel()

	srv := &buildServer{statuses: []string{statusBuilding, statusBuilding, statusCompleted}}
	_, opt := templateTestEnv(t, srv)

	info, err := BuildTemplate(context.Background(), TemplateSpec{Alias: "a"}, opt)
	if err != nil {
		t.Fatalf("BuildTemplate: %v", err)
	}
	if info.Status != BuildStatusCompleted {
		t.Errorf("Status = %q, want %q", info.Status, BuildStatusCompleted)
	}
	if info.TemplateID != "tpl-1" {
		t.Errorf("TemplateID = %q, want tpl-1", info.TemplateID)
	}
	if want := []string{"Step 1/3", "Step 2/3", "Step 3/3"}; !reflect.DeepEqual(info.Logs, want) {
		t.Errorf("Logs = %v, want %v", info.Logs, want)
	}
	if posts, polls := srv.counts(); posts != 1 || polls != 3 {
		t.Errorf("posts=%d polls=%d, want 1 post and 3 polls (stop at the first terminal status)", posts, polls)
	}
}

func TestBuildTemplate_FailedBuildReturnsBuildError(t *testing.T) {
	t.Parallel()

	logs := make([]string, 25)
	for i := range logs {
		logs[i] = fmt.Sprintf("line %02d", i+1)
	}
	failed, _ := json.Marshal(map[string]interface{}{
		"build_id": "bld-1", "status": "failed", "template_id": "tpl-1", "logs": logs,
	})
	srv := &buildServer{statuses: []string{string(failed)}}
	_, opt := templateTestEnv(t, srv)

	info, err := BuildTemplate(context.Background(), TemplateSpec{Alias: "a"}, opt)
	var buildErr *BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("err = %v (%T), want *BuildError", err, err)
	}
	if buildErr.BuildID != "bld-1" || len(buildErr.Logs) != 25 {
		t.Errorf("BuildError BuildID=%q with %d log lines, want bld-1 with 25", buildErr.BuildID, len(buildErr.Logs))
	}
	msg := err.Error()
	if !strings.Contains(msg, "line 25") || !strings.Contains(msg, "line 06") {
		t.Errorf("message should quote the last 20 log lines, got:\n%s", msg)
	}
	if strings.Contains(msg, "line 05") {
		t.Errorf("message should quote only the last 20 log lines, got:\n%s", msg)
	}
	if info == nil || info.Status != BuildStatusFailed {
		t.Errorf("info = %+v, want the failed build's BuildInfo", info)
	}
}

func TestBuildTemplate_ContextExpiresWhileWaiting(t *testing.T) {
	t.Parallel()

	srv := &buildServer{statuses: []string{statusBuilding}}
	_, opt := templateTestEnv(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	info, err := BuildTemplate(ctx, TemplateSpec{Alias: "a"}, opt)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if info == nil || info.BuildID != "bld-1" {
		t.Errorf("info = %+v, want the running build's BuildInfo so the caller can keep following it", info)
	}
}

func TestBuildTemplate_CopiesRejectedBeforeAnyRequest(t *testing.T) {
	t.Parallel()

	srv := &buildServer{statuses: []string{statusCompleted}}
	_, opt := templateTestEnv(t, srv)

	spec := TemplateSpec{Alias: "a", Copies: []CopyItem{{Src: "config.json", Dst: "/app/config.json"}}}
	for name, build := range map[string]func(context.Context, TemplateSpec, ...SandboxOption) (*BuildInfo, error){
		"BuildTemplate":           BuildTemplate,
		"BuildTemplateBackground": BuildTemplateBackground,
	} {
		_, err := build(context.Background(), spec, opt)
		var argErr *InvalidArgumentError
		if !errors.As(err, &argErr) {
			t.Errorf("%s: err = %v (%T), want *InvalidArgumentError", name, err, err)
		}
	}
	if posts, _ := srv.counts(); posts != 0 {
		t.Errorf("posts = %d, want 0: a spec with Copies must fail before reaching the API", posts)
	}
}

func TestBuildTemplate_ServerError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "build failed"}`))
	})

	_, opt := templateTestEnv(t, handler)

	_, err := BuildTemplate(context.Background(), TemplateSpec{Alias: "a", BaseImage: "ubuntu:22.04"}, opt)
	if err == nil {
		t.Fatal("expected an error on 500 response")
	}
}

func TestBuildTemplateBackground_ReturnsWithoutPolling(t *testing.T) {
	t.Parallel()

	srv := &buildServer{statuses: []string{statusCompleted}}
	_, opt := templateTestEnv(t, srv)

	info, err := BuildTemplateBackground(context.Background(), TemplateSpec{Alias: "bg-tpl", DiskMB: 4096}, opt)
	if err != nil {
		t.Fatalf("BuildTemplateBackground: %v", err)
	}
	if info.Status != BuildStatusBuilding || info.BuildID != "bld-1" {
		t.Errorf("info = %+v, want build bld-1 still building", info)
	}
	if _, polls := srv.counts(); polls != 0 {
		t.Errorf("polls = %d, want 0", polls)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	want := map[string]interface{}{"alias": "bg-tpl", "disk_mb": float64(4096), "template": map[string]interface{}{}}
	if !reflect.DeepEqual(srv.body, want) {
		t.Errorf("request body mismatch\n got: %#v\nwant: %#v", srv.body, want)
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
		_, _ = w.Write([]byte(`{"build_id": "bld-1", "status": "building", "logs": ["Step 1/2", "Step 2/2"]}`))
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
	if info.Status != BuildStatusBuilding {
		t.Errorf("expected Status='building', got %q", info.Status)
	}
	if want := []string{"Step 1/2", "Step 2/2"}; !reflect.DeepEqual(info.Logs, want) {
		t.Errorf("Logs = %v, want %v", info.Logs, want)
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
		// A build with no output yet serializes its logs as null.
		_, _ = w.Write([]byte(`{"build_id": "bld-done", "status": "completed", "template_id": "tpl-xyz", "logs": null}`))
	})

	_, opt := templateTestEnv(t, handler)

	info, err := GetBuildStatus(context.Background(), "bld-done", opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if info.TemplateID != "tpl-xyz" || info.Status != BuildStatusCompleted {
		t.Errorf("info = %+v, want completed build of tpl-xyz", info)
	}
	if len(info.Logs) != 0 {
		t.Errorf("Logs = %v, want none", info.Logs)
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
		_, _ = w.Write([]byte(`{"templates": [
			{"template_id": "tpl-1", "alias": "python", "created_at": "2026-01-01T00:00:00Z"},
			{"template_id": "tpl-2", "alias": "node", "created_at": "2026-01-02T00:00:00Z"}
		]}`))
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
		_, _ = w.Write([]byte(`{"templates": []}`))
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

// ---------------------------------------------------------------------------
// WaitForBuild: log streaming across the server's cap, and failed polls
// ---------------------------------------------------------------------------

// storedLogs is what sandbox-manager returns for a build that has produced
// total lines: past its 2000-line cap it keeps the truncation notice plus the
// newest 1999, so the array stops growing and positions shift.
func storedLogs(total int) []string {
	const limit = 2000
	lines := make([]string, total)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	if total <= limit {
		return lines
	}
	return append([]string{buildLogTruncationNotice}, lines[total-(limit-1):]...)
}

func TestLogCursor(t *testing.T) {
	t.Parallel()

	lines := func(from, to int) []string {
		var out []string
		for i := from; i <= to; i++ {
			out = append(out, fmt.Sprintf("line %d", i))
		}
		return out
	}

	var c logCursor
	if got := c.next(storedLogs(1980)); !reflect.DeepEqual(got, lines(1, 1980)) {
		t.Fatalf("growing log: got %d lines, want 1..1980", len(got))
	}
	if got := c.next(storedLogs(1980)); len(got) != 0 {
		t.Fatalf("unchanged log: got %v, want nothing", got)
	}
	if got := c.next(storedLogs(2050)); !reflect.DeepEqual(got, lines(1981, 2050)) {
		t.Fatalf("first trimmed log: got %d lines starting %q, want 1981..2050", len(got), first(got))
	}
	if got := c.next(storedLogs(2600)); !reflect.DeepEqual(got, lines(2051, 2600)) {
		t.Fatalf("trimmed log: got %d lines starting %q, want 2051..2600", len(got), first(got))
	}
	// More output than the server keeps arrived between two polls: nothing
	// handed out is left, so the notice marks the gap before what remains.
	if got := c.next(storedLogs(9000)); !reflect.DeepEqual(got, storedLogs(9000)) {
		t.Fatalf("gap: got %d lines starting %q, want the notice then 7002..9000", len(got), first(got))
	}

	// Following a build that is already past the cap starts from the notice.
	var late logCursor
	if got := late.next(storedLogs(2500)); !reflect.DeepEqual(got, storedLogs(2500)) {
		t.Fatalf("late start: got %d lines starting %q, want the notice then 502..2500", len(got), first(got))
	}
}

func first(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

// pollResponses serves GET /templates/builds/bld-1 from a script; the last
// entry repeats.
type pollResponses struct {
	mu    sync.Mutex
	codes []int
	docs  []interface{}
	polls int
}

func (p *pollResponses) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	i := p.polls
	if i >= len(p.codes) {
		i = len(p.codes) - 1
	}
	p.polls++
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(p.codes[i])
	_ = json.NewEncoder(w).Encode(p.docs[i])
}

func (p *pollResponses) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.polls
}

func buildDoc(status string, logs []string) map[string]interface{} {
	return map[string]interface{}{"build_id": "bld-1", "status": status, "template_id": "tpl-1", "logs": logs}
}

func TestWaitForBuild_StreamsEachLineOnceAcrossTheLogCap(t *testing.T) {
	t.Parallel()

	srv := &pollResponses{}
	for i, total := range []int{1980, 2050, 2600, 3000} {
		status := BuildStatusBuilding
		if i == 3 {
			status = BuildStatusCompleted
		}
		srv.codes = append(srv.codes, http.StatusOK)
		srv.docs = append(srv.docs, buildDoc(status, storedLogs(total)))
	}
	_, opt := templateTestEnv(t, srv)

	var got []string
	info, err := WaitForBuild(context.Background(), "bld-1", func(line string) { got = append(got, line) }, opt)
	if err != nil {
		t.Fatalf("WaitForBuild: %v", err)
	}
	if info.Status != BuildStatusCompleted || info.TemplateID != "tpl-1" {
		t.Errorf("info = %+v, want the completed build of tpl-1", info)
	}
	want := make([]string, 3000)
	for i := range want {
		want[i] = fmt.Sprintf("line %d", i+1)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("delivered %d lines (first %q, last %q), want lines 1..3000 once each, in order",
			len(got), first(got), got[len(got)-1])
	}
}

func TestWaitForBuild_RidesOutTemporaryFailures(t *testing.T) {
	t.Parallel()

	busy := map[string]interface{}{"message": "slow down"}
	srv := &pollResponses{
		codes: []int{http.StatusTooManyRequests, http.StatusTooManyRequests, http.StatusOK},
		docs:  []interface{}{busy, busy, buildDoc(BuildStatusCompleted, nil)},
	}
	_, opt := templateTestEnv(t, srv)

	info, err := WaitForBuild(context.Background(), "bld-1", nil, opt)
	if err != nil {
		t.Fatalf("WaitForBuild: %v, want the two 429s ridden out", err)
	}
	if info.Status != BuildStatusCompleted || srv.count() != 3 {
		t.Errorf("status %q after %d polls, want completed after 3", info.Status, srv.count())
	}
}

func TestWaitForBuild_GivesUpAfterTheErrorWindow(t *testing.T) {
	t.Parallel()

	srv := &pollResponses{
		codes: []int{http.StatusTooManyRequests},
		docs:  []interface{}{map[string]interface{}{"message": "slow down"}},
	}
	_, opt := templateTestEnv(t, srv)

	// Bounded, so a wait that never gives up fails here instead of hanging.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := WaitForBuild(ctx, "bld-1", nil, opt)
	var rateErr *RateLimitError
	if !errors.As(err, &rateErr) {
		t.Fatalf("err = %v (%T), want the last *RateLimitError once the error window passed", err, err)
	}
}

func TestWaitForBuild_RejectionEndsTheWait(t *testing.T) {
	t.Parallel()

	srv := &pollResponses{
		codes: []int{http.StatusNotFound},
		docs:  []interface{}{map[string]interface{}{"message": "build bld-1 not found"}},
	}
	_, opt := templateTestEnv(t, srv)

	_, err := WaitForBuild(context.Background(), "bld-1", nil, opt)
	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("err = %v (%T), want *NotFoundError", err, err)
	}
	if srv.count() != 1 {
		t.Errorf("polls = %d, want 1: a 4xx must not be retried", srv.count())
	}
}
