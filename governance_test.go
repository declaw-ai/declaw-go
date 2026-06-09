package declaw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func governanceTestEnv(t *testing.T, handler http.Handler) (*httptest.Server, SandboxOption) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts, func(o *sandboxOpts) {
		o.APIKey = "test-key"
		o.APIURL = ts.URL
	}
}

// ---------------------------------------------------------------------------
// ListGovernancePacks tests
// ---------------------------------------------------------------------------

func TestListGovernancePacks_Success(t *testing.T) {
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
			"packs": [
				{
					"name": "owasp-llm-top10",
					"version": "v1",
					"framework": "OWASP Top 10 for LLM Applications (2025)",
					"description": "Enforces controls aligned to the OWASP LLM Top 10 (2025 edition).",
					"gates": ["cmd", "network", "content"],
					"enforces": [
						{
							"control": "OWASP-LLM06-ExcessiveAgency",
							"gate": "cmd",
							"rule": "block_shell_escape",
							"playbook": "https://owasp.org/llm06"
						}
					],
					"advisory": [
						{
							"control": "OWASP-LLM03-TrainingDataPoisoning",
							"reason": "Requires out-of-band data pipeline controls"
						}
					],
					"policy_ref": "owasp-llm-top10@v1",
					"seeded": true
				},
				{
					"name": "pci-dss-v4",
					"version": "v1",
					"framework": "PCI DSS v4.0",
					"description": "Controls for payment card data environments.",
					"gates": ["content"],
					"enforces": [],
					"advisory": [],
					"policy_ref": "pci-dss-v4@v1",
					"seeded": false
				}
			]
		}`))
	})

	_, opt := governanceTestEnv(t, handler)

	packs, err := ListGovernancePacks(context.Background(), opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/governance/packs" {
		t.Errorf("expected path /governance/packs, got %s", gotPath)
	}

	if len(packs) != 2 {
		t.Fatalf("expected 2 packs, got %d", len(packs))
	}

	first := packs[0]
	if first.Name != "owasp-llm-top10" {
		t.Errorf("expected Name='owasp-llm-top10', got %q", first.Name)
	}
	if first.Version != "v1" {
		t.Errorf("expected Version='v1', got %q", first.Version)
	}
	if first.Framework != "OWASP Top 10 for LLM Applications (2025)" {
		t.Errorf("unexpected Framework: %q", first.Framework)
	}
	if len(first.Gates) != 3 {
		t.Errorf("expected 3 gates, got %d", len(first.Gates))
	}
	if len(first.Enforces) != 1 {
		t.Errorf("expected 1 enforced control, got %d", len(first.Enforces))
	}
	if first.Enforces[0].Control != "OWASP-LLM06-ExcessiveAgency" {
		t.Errorf("unexpected Enforces[0].Control: %q", first.Enforces[0].Control)
	}
	if first.Enforces[0].Gate != "cmd" {
		t.Errorf("unexpected Enforces[0].Gate: %q", first.Enforces[0].Gate)
	}
	if first.Enforces[0].Rule != "block_shell_escape" {
		t.Errorf("unexpected Enforces[0].Rule: %q", first.Enforces[0].Rule)
	}
	if len(first.Advisory) != 1 {
		t.Errorf("expected 1 advisory, got %d", len(first.Advisory))
	}
	if first.Advisory[0].Control != "OWASP-LLM03-TrainingDataPoisoning" {
		t.Errorf("unexpected Advisory[0].Control: %q", first.Advisory[0].Control)
	}
	if first.PolicyRef != "owasp-llm-top10@v1" {
		t.Errorf("expected PolicyRef='owasp-llm-top10@v1', got %q", first.PolicyRef)
	}
	if !first.Seeded {
		t.Errorf("expected Seeded=true")
	}

	second := packs[1]
	if second.Name != "pci-dss-v4" {
		t.Errorf("expected second pack Name='pci-dss-v4', got %q", second.Name)
	}
	if second.Seeded {
		t.Errorf("expected second pack Seeded=false")
	}
}

func TestListGovernancePacks_EmptyList(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"packs": []}`))
	})

	_, opt := governanceTestEnv(t, handler)

	packs, err := ListGovernancePacks(context.Background(), opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(packs) != 0 {
		t.Errorf("expected 0 packs, got %d", len(packs))
	}
}

func TestListGovernancePacks_ServerError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	})

	_, opt := governanceTestEnv(t, handler)

	_, err := ListGovernancePacks(context.Background(), opt)
	if err == nil {
		t.Fatal("expected an error on 500 response")
	}
}

func TestListGovernancePacks_ContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"packs": []}`))
	})

	_, opt := governanceTestEnv(t, handler)

	_, err := ListGovernancePacks(ctx, opt)
	if err == nil {
		t.Fatal("expected an error when context is already canceled")
	}
}

// ---------------------------------------------------------------------------
// GetGovernancePack tests
// ---------------------------------------------------------------------------

func TestGetGovernancePack_Success(t *testing.T) {
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
			"name": "owasp-llm-top10",
			"version": "v1",
			"framework": "OWASP Top 10 for LLM Applications (2025)",
			"description": "Enforces controls aligned to the OWASP LLM Top 10 (2025 edition).",
			"gates": ["cmd", "network", "content"],
			"enforces": [
				{
					"control": "OWASP-LLM06-ExcessiveAgency",
					"gate": "cmd",
					"rule": "block_shell_escape",
					"playbook": "https://owasp.org/llm06"
				}
			],
			"advisory": [
				{
					"control": "OWASP-LLM03-TrainingDataPoisoning",
					"reason": "Requires out-of-band data pipeline controls"
				}
			],
			"policy_ref": "owasp-llm-top10@v1",
			"seeded": true
		}`))
	})

	_, opt := governanceTestEnv(t, handler)

	pack, err := GetGovernancePack(context.Background(), "owasp-llm-top10", opt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/governance/packs/owasp-llm-top10" {
		t.Errorf("expected path /governance/packs/owasp-llm-top10, got %s", gotPath)
	}

	if pack == nil {
		t.Fatal("expected non-nil pack")
	}
	if pack.Name != "owasp-llm-top10" {
		t.Errorf("expected Name='owasp-llm-top10', got %q", pack.Name)
	}
	if pack.PolicyRef != "owasp-llm-top10@v1" {
		t.Errorf("expected PolicyRef='owasp-llm-top10@v1', got %q", pack.PolicyRef)
	}
	if !pack.Seeded {
		t.Errorf("expected Seeded=true")
	}
	if len(pack.Enforces) != 1 {
		t.Errorf("expected 1 enforced control, got %d", len(pack.Enforces))
	}
	if pack.Enforces[0].Playbook != "https://owasp.org/llm06" {
		t.Errorf("unexpected Playbook: %q", pack.Enforces[0].Playbook)
	}
	if len(pack.Advisory) != 1 {
		t.Errorf("expected 1 advisory, got %d", len(pack.Advisory))
	}
	if pack.Advisory[0].Reason != "Requires out-of-band data pipeline controls" {
		t.Errorf("unexpected Advisory[0].Reason: %q", pack.Advisory[0].Reason)
	}
}

func TestGetGovernancePack_NotFound(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "governance pack not found"}`))
	})

	_, opt := governanceTestEnv(t, handler)

	_, err := GetGovernancePack(context.Background(), "nonexistent-pack", opt)
	if err == nil {
		t.Fatal("expected an error on 404 response")
	}
}

func TestGetGovernancePack_ContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"x"}`))
	})

	_, opt := governanceTestEnv(t, handler)

	_, err := GetGovernancePack(ctx, "some-pack", opt)
	if err == nil {
		t.Fatal("expected an error when context is already canceled")
	}
}
