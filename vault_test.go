package declaw_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	declaw "github.com/declaw-ai/declaw-go"
)

func vaultTestServer(t *testing.T, handler http.HandlerFunc) *declaw.VaultClient {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	client := declaw.NewTestAPIClient(&declaw.Config{APIKey: "test-key", APIURL: ts.URL})
	return declaw.NewTestVaultClient(client)
}

// defaultTeamHandler answers the team/env resolution endpoints with a
// pre-existing "default" team + "prod" env, and forwards everything else to next.
func defaultTeamHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/teams":
			_, _ = w.Write([]byte(`{"teams":[{"team_id":"team-def","name":"default","created_at":"2026-01-01T00:00:00Z"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/teams/team-def/environments":
			_, _ = w.Write([]byte(`{"environments":[{"env_id":"env-prod","name":"prod"}]}`))
		default:
			next(w, r)
		}
	}
}

func TestVaultClient_CreateSecret_UsesDefaultTeamAndProd(t *testing.T) {
	t.Parallel()
	var gotPath string
	var gotBody map[string]any
	v := vaultTestServer(t, defaultTeamHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/teams/team-def/vault/secrets" {
			gotPath = r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"secret_id":"sec-1","team_id":"team-def","env_id":"env-prod","name":"openai"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	sec, err := v.CreateSecret(context.Background(), declaw.CreateSecretInput{Provider: "openai", Name: "openai", Value: "sk-x"})
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}
	if gotPath != "/teams/team-def/vault/secrets" {
		t.Errorf("wrong path: %s", gotPath)
	}
	if gotBody["environment"] != "prod" {
		t.Errorf("environment should default to prod, got %v", gotBody["environment"])
	}
	if gotBody["value"] != "sk-x" || gotBody["provider"] != "openai" {
		t.Errorf("body wrong: %+v", gotBody)
	}
	if sec.SecretID != "sec-1" || sec.Name != "openai" {
		t.Errorf("parsed secret wrong: %+v", sec)
	}
}

func TestVaultClient_CreateSecret_AutoProvisionsDefaultTeam(t *testing.T) {
	t.Parallel()
	var createdTeam, createdEnv bool
	v := vaultTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/teams":
			_, _ = w.Write([]byte(`{"teams":[]}`)) // none yet
		case r.Method == http.MethodPost && r.URL.Path == "/teams":
			createdTeam = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"team_id":"team-new","name":"default","created_at":"2026-01-02T00:00:00Z"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/teams/team-new/environments":
			_, _ = w.Write([]byte(`{"environments":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/teams/team-new/environments":
			createdEnv = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"env_id":"env-prod","name":"prod"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/teams/team-new/vault/secrets":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"secret_id":"sec-9","name":"stripe"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	_, err := v.CreateSecret(context.Background(), declaw.CreateSecretInput{
		Name: "stripe", Value: "sk", Scopes: []declaw.VaultScope{{DomainRegex: `~^api\.stripe\.com$`, InjectionType: "bearer"}},
	})
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}
	if !createdTeam || !createdEnv {
		t.Errorf("expected auto-create of default team (%v) + prod env (%v)", createdTeam, createdEnv)
	}
}

func TestVaultClient_RotateSecret_ByName(t *testing.T) {
	t.Parallel()
	var rotatedID string
	var rotateBody map[string]any
	v := vaultTestServer(t, defaultTeamHandler(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/teams/team-def/vault/secrets":
			_, _ = w.Write([]byte(`{"secrets":[{"secret_id":"sec-42","name":"stripe"}]}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/rotate"):
			rotatedID = strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/teams/team-def/vault/secrets/"), "/rotate")
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &rotateBody)
			_, _ = w.Write([]byte(`{"rotated":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	if err := v.RotateSecret(context.Background(), "stripe", "new-val"); err != nil {
		t.Fatalf("RotateSecret: %v", err)
	}
	if rotatedID != "sec-42" {
		t.Errorf("rotated wrong id (name->id resolution failed): %q", rotatedID)
	}
	if rotateBody["value"] != "new-val" {
		t.Errorf("rotate value not sent: %+v", rotateBody)
	}
}

func TestVaultClient_UpdateScopes_ByName(t *testing.T) {
	t.Parallel()
	var updatedID string
	var updateBody map[string]any
	v := vaultTestServer(t, defaultTeamHandler(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/teams/team-def/vault/secrets":
			_, _ = w.Write([]byte(`{"secrets":[{"secret_id":"sec-42","name":"stripe"}]}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/scopes"):
			updatedID = strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/teams/team-def/vault/secrets/"), "/scopes")
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &updateBody)
			_, _ = w.Write([]byte(`{"updated":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	scopes := []declaw.VaultScope{{DomainRegex: `~^api\.stripe\.com$`, InjectionType: "bearer"}}
	if err := v.UpdateScopes(context.Background(), "stripe", scopes); err != nil {
		t.Fatalf("UpdateScopes: %v", err)
	}
	if updatedID != "sec-42" {
		t.Errorf("updated wrong id (name->id resolution failed): %q", updatedID)
	}
	sent, ok := updateBody["scopes"].([]any)
	if !ok || len(sent) != 1 {
		t.Fatalf("scopes not sent correctly: %+v", updateBody)
	}
	if first, _ := sent[0].(map[string]any); first["domain_regex"] != `~^api\.stripe\.com$` || first["injection_type"] != "bearer" {
		t.Errorf("scope body wrong: %+v", sent[0])
	}

	// Empty scopes is rejected client-side, before any HTTP call.
	if err := v.UpdateScopes(context.Background(), "stripe", nil); err == nil {
		t.Error("expected error for empty scopes")
	}
}

func TestVaultClient_DeleteSecret_ByName(t *testing.T) {
	t.Parallel()
	var delPath string
	v := vaultTestServer(t, defaultTeamHandler(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/teams/team-def/vault/secrets":
			_, _ = w.Write([]byte(`{"secrets":[{"secret_id":"sec-7","name":"openai"}]}`))
		case r.Method == http.MethodDelete:
			delPath = r.URL.Path
			_, _ = w.Write([]byte(`{"deleted":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	if err := v.DeleteSecret(context.Background(), "openai"); err != nil {
		t.Fatalf("DeleteSecret: %v", err)
	}
	if delPath != "/teams/team-def/vault/secrets/sec-7" {
		t.Errorf("delete wrong path (name->id resolution failed): %s", delPath)
	}
}

func TestVaultClient_DeleteSecret_NotFound(t *testing.T) {
	t.Parallel()
	v := vaultTestServer(t, defaultTeamHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/teams/team-def/vault/secrets" {
			_, _ = w.Write([]byte(`{"secrets":[]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	if err := v.DeleteSecret(context.Background(), "ghost"); err == nil {
		t.Fatal("expected not-found error for missing secret")
	}
}

func TestVaultClient_ListSecrets_EmptyWhenNoDefaultTeam(t *testing.T) {
	t.Parallel()
	v := vaultTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/teams" {
			_, _ = w.Write([]byte(`{"teams":[]}`)) // no default team yet
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	secs, err := v.ListSecrets(context.Background())
	if err != nil {
		t.Fatalf("ListSecrets: %v", err)
	}
	if len(secs) != 0 {
		t.Errorf("expected empty, got %d", len(secs))
	}
}

func TestVaultClient_ListPresets(t *testing.T) {
	t.Parallel()
	v := vaultTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/vault/presets" {
			t.Errorf("wrong path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"presets":[{"key":"openai","name":"OpenAI","category":"llm","key_hint":"sk-…","scopes":[]}]}`))
	})
	presets, err := v.ListPresets(context.Background())
	if err != nil {
		t.Fatalf("ListPresets: %v", err)
	}
	if len(presets) != 1 || presets[0].Key != "openai" {
		t.Errorf("parsed presets wrong: %+v", presets)
	}
}
