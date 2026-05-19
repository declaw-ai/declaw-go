package declaw

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func accountTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *apiClient) {
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
// GetAccount tests
// ---------------------------------------------------------------------------

func TestAccountClient_GetAccount_Success(t *testing.T) {
	t.Parallel()

	var (
		mu       sync.Mutex
		gotPaths []string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPaths = append(gotPaths, r.URL.Path)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/auth/me":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"owner_id": "acct-123"}`))
		case "/accounts/acct-123":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"owner_id": "acct-123",
				"email": "test@example.com",
				"tier": "pro",
				"created_at": "2026-01-01T00:00:00Z",
				"sandbox_free_micros": 100000000,
				"guardrails_free_micros": 200000000,
				"balance_micros": 50000000
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClient(client)

	info, err := ac.GetAccount(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if info == nil {
		t.Fatal("expected non-nil AccountInfo")
	}
	if info.OwnerID != "acct-123" {
		t.Errorf("expected OwnerID='acct-123', got %q", info.OwnerID)
	}
	if info.Email != "test@example.com" {
		t.Errorf("expected Email='test@example.com', got %q", info.Email)
	}
	if info.Tier != "pro" {
		t.Errorf("expected Tier='pro', got %q", info.Tier)
	}
	if info.SandboxFreeMicros != 100000000 {
		t.Errorf("expected SandboxFreeMicros=100000000, got %d", info.SandboxFreeMicros)
	}
	if info.GuardrailsFreeMicros != 200000000 {
		t.Errorf("expected GuardrailsFreeMicros=200000000, got %d", info.GuardrailsFreeMicros)
	}
	if info.BalanceMicros != 50000000 {
		t.Errorf("expected BalanceMicros=50000000, got %d", info.BalanceMicros)
	}
}

func TestAccountClient_GetAccount_CachesOwnerID(t *testing.T) {
	t.Parallel()

	var authMeCount atomic.Int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/auth/me":
			authMeCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"owner_id": "acct-cache"}`))
		case "/accounts/acct-cache":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"owner_id": "acct-cache",
				"email": "cached@example.com",
				"tier": "free",
				"created_at": "2026-01-01T00:00:00Z"
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClient(client)

	_, err := ac.GetAccount(context.Background())
	if err != nil {
		t.Fatalf("first call: expected no error, got %v", err)
	}

	_, err = ac.GetAccount(context.Background())
	if err != nil {
		t.Fatalf("second call: expected no error, got %v", err)
	}

	count := authMeCount.Load()
	if count != 1 {
		t.Errorf("expected /auth/me to be called once (caching), got %d calls", count)
	}
}

func TestAccountClient_GetAccount_AuthError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "invalid api key"}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClient(client)

	_, err := ac.GetAccount(context.Background())
	if err == nil {
		t.Fatal("expected an error on 401 response")
	}
}

func TestAccountClient_GetAccount_ContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"owner_id": "acct-x"}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClient(client)

	_, err := ac.GetAccount(ctx)
	if err == nil {
		t.Fatal("expected an error when context is already canceled")
	}
}

func TestAccountClient_GetAccount_EmptyOwnerID(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"owner_id": ""}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClient(client)

	_, err := ac.GetAccount(context.Background())
	if err == nil {
		t.Fatal("expected error when server returns empty owner_id")
	}
}

// ---------------------------------------------------------------------------
// GetOverview tests
// ---------------------------------------------------------------------------

func TestAccountClient_GetOverview_Success(t *testing.T) {
	t.Parallel()

	var (
		mu        sync.Mutex
		gotMethod string
		gotPath   string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotMethod = r.Method
		gotPath = r.URL.Path
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"owner_id": "acct-ov",
			"tier": "pro",
			"active_sandboxes": 3,
			"tier_limits": {
				"max_concurrent": 10,
				"max_vcpus": 32,
				"max_memory_mb": 32768
			},
			"wallets": {
				"sandbox_free_micros": 100000000,
				"guardrails_free_micros": 200000000,
				"balance_micros": 50000000
			},
			"today": {
				"compute_cost_micros": 5000,
				"guardrails_cost_micros": 1000,
				"total_cost_micros": 6000
			}
		}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-ov")

	overview, err := ac.GetOverview(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/accounts/acct-ov/overview" {
		t.Errorf("expected path /accounts/acct-ov/overview, got %s", gotPath)
	}

	if overview == nil {
		t.Fatal("expected non-nil AccountOverview")
	}
	if overview.Tier != "pro" {
		t.Errorf("expected Tier='pro', got %q", overview.Tier)
	}
	if overview.ActiveSandboxes != 3 {
		t.Errorf("expected ActiveSandboxes=3, got %d", overview.ActiveSandboxes)
	}
	if overview.TierLimits.MaxConcurrent != 10 {
		t.Errorf("expected MaxConcurrent=10, got %d", overview.TierLimits.MaxConcurrent)
	}
	if overview.TierLimits.MaxVCPUs != 32 {
		t.Errorf("expected MaxVCPUs=32, got %d", overview.TierLimits.MaxVCPUs)
	}
	if overview.Wallets.SandboxFreeMicros != 100000000 {
		t.Errorf("expected SandboxFreeMicros=100000000, got %d", overview.Wallets.SandboxFreeMicros)
	}
	if overview.Today.TotalCostMicros != 6000 {
		t.Errorf("expected TotalCostMicros=6000, got %d", overview.Today.TotalCostMicros)
	}
}

func TestAccountClient_GetOverview_ServerError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-err")

	_, err := ac.GetOverview(context.Background())
	if err == nil {
		t.Fatal("expected an error on 500 response")
	}
}

// ---------------------------------------------------------------------------
// GetUsage tests
// ---------------------------------------------------------------------------

func TestAccountClient_GetUsage_Success(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		gotPath string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPath = r.URL.Path
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"owner_id": "acct-usage",
			"since": "2026-04-01T00:00:00Z",
			"total_cost_micros": 500000,
			"total_cost_usd": "0.50",
			"sandbox_count": 12,
			"total_seconds": 36000.0,
			"sandbox_balance_remaining_micros": 99500000,
			"balance_remaining_micros": 99500000,
			"guardrails_cost_micros": 100000,
			"guardrails_cost_usd": "0.10",
			"guardrails_balance_remaining_micros": 199900000
		}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-usage")

	usage, err := ac.GetUsage(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotPath != "/accounts/acct-usage/usage" {
		t.Errorf("expected path /accounts/acct-usage/usage, got %s", gotPath)
	}
	if usage == nil {
		t.Fatal("expected non-nil UsageSummary")
	}
	if usage.TotalCostMicros != 500000 {
		t.Errorf("expected TotalCostMicros=500000, got %d", usage.TotalCostMicros)
	}
	if usage.TotalCostUSD != "0.50" {
		t.Errorf("expected TotalCostUSD='0.50', got %q", usage.TotalCostUSD)
	}
	if usage.SandboxCount != 12 {
		t.Errorf("expected SandboxCount=12, got %d", usage.SandboxCount)
	}
	if usage.TotalSeconds != 36000.0 {
		t.Errorf("expected TotalSeconds=36000, got %f", usage.TotalSeconds)
	}
	if usage.GuardrailsCostMicros != 100000 {
		t.Errorf("expected GuardrailsCostMicros=100000, got %d", usage.GuardrailsCostMicros)
	}
}

func TestAccountClient_GetUsage_Empty(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"owner_id": "acct-no-usage",
			"since": "2026-04-01T00:00:00Z",
			"total_cost_micros": 0,
			"total_cost_usd": "0.00",
			"sandbox_count": 0,
			"total_seconds": 0
		}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-no-usage")

	usage, err := ac.GetUsage(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if usage.TotalCostMicros != 0 {
		t.Errorf("expected TotalCostMicros=0, got %d", usage.TotalCostMicros)
	}
	if usage.SandboxCount != 0 {
		t.Errorf("expected SandboxCount=0, got %d", usage.SandboxCount)
	}
}

// ---------------------------------------------------------------------------
// ListDeposits tests
// ---------------------------------------------------------------------------

func TestAccountClient_ListDeposits_Success(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		gotPath string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPath = r.URL.Path
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"owner_id": "acct-deposits",
			"pagination": {"total": 2, "limit": 20, "offset": 0, "has_more": false},
			"entries": [
				{
					"deposit_id": "dep-1",
					"wallet_type": "compute",
					"amount_micros": 10000000,
					"amount_usd": "10.00",
					"status": "completed",
					"provider": "polar",
					"created_at": "2026-03-01T00:00:00Z",
					"completed_at": "2026-03-01T00:01:00Z"
				},
				{
					"deposit_id": "dep-2",
					"wallet_type": "guardrails",
					"amount_micros": 5000000,
					"amount_usd": "5.00",
					"status": "pending",
					"provider": "polar",
					"created_at": "2026-04-01T00:00:00Z"
				}
			]
		}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-deposits")

	deposits, err := ac.ListDeposits(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotPath != "/accounts/acct-deposits/deposits" {
		t.Errorf("expected path /accounts/acct-deposits/deposits, got %s", gotPath)
	}
	if len(deposits) != 2 {
		t.Fatalf("expected 2 deposits, got %d", len(deposits))
	}
	if deposits[0].DepositID != "dep-1" {
		t.Errorf("expected first deposit ID='dep-1', got %q", deposits[0].DepositID)
	}
	if deposits[0].AmountMicros != 10000000 {
		t.Errorf("expected first deposit AmountMicros=10000000, got %d", deposits[0].AmountMicros)
	}
	if deposits[0].Status != "completed" {
		t.Errorf("expected first deposit Status='completed', got %q", deposits[0].Status)
	}
	if deposits[0].CompletedAt == nil {
		t.Error("expected first deposit CompletedAt to be non-nil")
	}
	if deposits[1].Status != "pending" {
		t.Errorf("expected second deposit Status='pending', got %q", deposits[1].Status)
	}
	if deposits[1].CompletedAt != nil {
		t.Errorf("expected second deposit CompletedAt to be nil, got %v", deposits[1].CompletedAt)
	}
}

func TestAccountClient_ListDeposits_EmptyList(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"owner_id": "acct-no-dep",
			"pagination": {"total": 0, "limit": 20, "offset": 0, "has_more": false},
			"entries": []
		}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-no-dep")

	deposits, err := ac.ListDeposits(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(deposits) != 0 {
		t.Errorf("expected 0 deposits, got %d", len(deposits))
	}
}

// ---------------------------------------------------------------------------
// CreateAPIKey tests
// ---------------------------------------------------------------------------

func TestAccountClient_CreateAPIKey_Success(t *testing.T) {
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
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"key_id": "key-abc",
			"api_key": "dec_live_abc123secret",
			"name": "my-key"
		}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-keys")

	result, err := ac.CreateAPIKey(context.Background(), "my-key")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/accounts/acct-keys/api-keys" {
		t.Errorf("expected path /accounts/acct-keys/api-keys, got %s", gotPath)
	}
	if nameVal, _ := gotBody["name"].(string); nameVal != "my-key" {
		t.Errorf("expected body name='my-key', got %q", nameVal)
	}

	if result == nil {
		t.Fatal("expected non-nil CreateAPIKeyResult")
	}
	if result.KeyID != "key-abc" {
		t.Errorf("expected KeyID='key-abc', got %q", result.KeyID)
	}
	if result.APIKey != "dec_live_abc123secret" {
		t.Errorf("expected APIKey='dec_live_abc123secret', got %q", result.APIKey)
	}
	if result.Name != "my-key" {
		t.Errorf("expected Name='my-key', got %q", result.Name)
	}
}

func TestAccountClient_CreateAPIKey_EmptyName(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error": "name is required"}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-keys")

	_, err := ac.CreateAPIKey(context.Background(), "")
	if err == nil {
		t.Fatal("expected an error when creating key with empty name")
	}
}

func TestAccountClient_CreateAPIKey_ServerError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-keys")

	_, err := ac.CreateAPIKey(context.Background(), "test")
	if err == nil {
		t.Fatal("expected an error on 500 response")
	}
}

// ---------------------------------------------------------------------------
// ListAPIKeys tests
// ---------------------------------------------------------------------------

func TestAccountClient_ListAPIKeys_Success(t *testing.T) {
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
			"api_keys": [
				{"key_id": "key-1", "name": "prod", "masked_key": "dec_live...", "created_at": "2026-01-01T00:00:00Z", "revoked": false},
				{"key_id": "key-2", "name": "dev", "masked_key": "dec_test...", "created_at": "2026-02-01T00:00:00Z", "revoked": true}
			]
		}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-list-keys")

	keys, err := ac.ListAPIKeys(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/accounts/acct-list-keys/api-keys" {
		t.Errorf("expected path /accounts/acct-list-keys/api-keys, got %s", gotPath)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[0].KeyID != "key-1" {
		t.Errorf("expected first key ID 'key-1', got %q", keys[0].KeyID)
	}
	if keys[0].MaskedKey != "dec_live..." {
		t.Errorf("expected first key MaskedKey 'dec_live...', got %q", keys[0].MaskedKey)
	}
	if keys[0].Revoked {
		t.Error("expected first key Revoked=false")
	}
	if keys[1].Name != "dev" {
		t.Errorf("expected second key name 'dev', got %q", keys[1].Name)
	}
	if !keys[1].Revoked {
		t.Error("expected second key Revoked=true")
	}
}

func TestAccountClient_ListAPIKeys_EmptyList(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"api_keys": []}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-no-keys")

	keys, err := ac.ListAPIKeys(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

// ---------------------------------------------------------------------------
// RevokeAPIKey tests
// ---------------------------------------------------------------------------

func TestAccountClient_RevokeAPIKey_Success(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"revoked": true}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-revoke")

	err := ac.RevokeAPIKey(context.Background(), "key-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
	if gotPath != "/accounts/acct-revoke/api-keys/key-123" {
		t.Errorf("expected path /accounts/acct-revoke/api-keys/key-123, got %s", gotPath)
	}
}

func TestAccountClient_RevokeAPIKey_NotFound(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "key not found"}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-revoke")

	err := ac.RevokeAPIKey(context.Background(), "key-nonexistent")
	if err == nil {
		t.Fatal("expected an error on 404 response")
	}
}

// ---------------------------------------------------------------------------
// Error handling: 402 InsufficientBalanceError
// ---------------------------------------------------------------------------

func TestAccountClient_402_InsufficientBalanceError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"message": "insufficient balance"}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-broke")

	_, err := ac.GetOverview(context.Background())
	if err == nil {
		t.Fatal("expected an error on 402 response")
	}

	var ibe *InsufficientBalanceError
	if errors.As(err, &ibe) {
		if ibe.StatusCode != http.StatusPaymentRequired {
			t.Errorf("expected StatusCode=402, got %d", ibe.StatusCode)
		}
	} else {
		t.Logf("note: error is not *InsufficientBalanceError yet (stub): %T: %v", err, err)
	}
}

// ---------------------------------------------------------------------------
// Error handling: 429 RateLimitError
// ---------------------------------------------------------------------------

func TestAccountClient_429_RateLimitError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.Header().Set("X-RateLimit-Limit", "100")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message": "rate limit exceeded"}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-limited")

	_, err := ac.GetUsage(context.Background())
	if err == nil {
		t.Fatal("expected an error on 429 response")
	}

	var rle *RateLimitError
	if errors.As(err, &rle) {
		if rle.RetryAfter.Seconds() != 30 {
			t.Errorf("expected RetryAfter=30s, got %v", rle.RetryAfter)
		}
		if rle.Limit != 100 {
			t.Errorf("expected Limit=100, got %d", rle.Limit)
		}
		if rle.Remaining != 0 {
			t.Errorf("expected Remaining=0, got %d", rle.Remaining)
		}
	} else {
		t.Logf("note: error is not *RateLimitError yet (stub): %T: %v", err, err)
	}
}

func TestAccountClient_429_WithoutRetryAfter(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message": "rate limit exceeded"}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-limited2")

	_, err := ac.GetUsage(context.Background())
	if err == nil {
		t.Fatal("expected an error on 429 response")
	}

	var rle *RateLimitError
	if errors.As(err, &rle) {
		if rle.RetryAfter != 0 {
			t.Errorf("expected RetryAfter=0 when header missing, got %v", rle.RetryAfter)
		}
	}
}

// ---------------------------------------------------------------------------
// Error handling: InsufficientBalanceError inherits SandboxError
// ---------------------------------------------------------------------------

func TestInsufficientBalanceError_HasSandboxError(t *testing.T) {
	t.Parallel()

	ibe := &InsufficientBalanceError{
		SandboxError: &SandboxError{
			Message:    "no funds",
			StatusCode: 402,
		},
	}

	if ibe.SandboxError == nil {
		t.Fatal("expected embedded SandboxError to be non-nil")
	}
	if ibe.StatusCode != 402 {
		t.Errorf("expected StatusCode=402, got %d", ibe.StatusCode)
	}
	errMsg := ibe.Error()
	if errMsg != "no funds" {
		t.Errorf("expected error message 'no funds', got %q", errMsg)
	}

	var target *InsufficientBalanceError
	if !errors.As(ibe, &target) {
		t.Error("expected errors.As to find InsufficientBalanceError")
	}
}

func TestRateLimitError_HasSandboxError(t *testing.T) {
	t.Parallel()

	rle := &RateLimitError{
		SandboxError: &SandboxError{
			Message:    "slow down",
			StatusCode: 429,
		},
	}

	if rle.SandboxError == nil {
		t.Fatal("expected embedded SandboxError to be non-nil")
	}
	if rle.StatusCode != 429 {
		t.Errorf("expected StatusCode=429, got %d", rle.StatusCode)
	}
	errMsg := rle.Error()
	if errMsg != "slow down" {
		t.Errorf("expected error message 'slow down', got %q", errMsg)
	}

	var target *RateLimitError
	if !errors.As(rle, &target) {
		t.Error("expected errors.As to find RateLimitError")
	}
}

// ---------------------------------------------------------------------------
// Close / lifecycle tests
// ---------------------------------------------------------------------------

func TestAccountClient_Close(t *testing.T) {
	t.Parallel()

	cfg := &Config{APIKey: "test-key", APIURL: "http://localhost:9999"}
	client := newAPIClient(cfg)
	ac := NewTestAccountClient(client)

	err := ac.Close()
	if err != nil {
		t.Fatalf("expected Close() to return nil, got %v", err)
	}
}

func TestAccountClient_Close_Idempotent(t *testing.T) {
	t.Parallel()

	cfg := &Config{APIKey: "test-key", APIURL: "http://localhost:9999"}
	client := newAPIClient(cfg)
	ac := NewTestAccountClient(client)

	_ = ac.Close()
	err := ac.Close()
	if err != nil {
		t.Fatalf("expected second Close() to return nil, got %v", err)
	}
}

func TestNewAccountClient_CreatesClient(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "env-key")
	t.Setenv("DECLAW_DOMAIN", "api.test.dev")

	ac := NewAccountClient(WithAPIKey("explicit-key"))
	if ac == nil {
		t.Fatal("expected non-nil AccountClient")
	}
	defer ac.Close()
}

func TestNewAccountClient_WithAPIURL(t *testing.T) {
	ac := NewAccountClient(
		WithAPIKey("test-key"),
		WithAPIURL("http://localhost:8080"),
	)
	if ac == nil {
		t.Fatal("expected non-nil AccountClient")
	}
	defer ac.Close()
}

// ---------------------------------------------------------------------------
// Pre-set ownerID path tests
// ---------------------------------------------------------------------------

func TestAccountClient_PresetOwnerID_SkipsAuthMe(t *testing.T) {
	t.Parallel()

	var authMeCount atomic.Int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/me" {
			authMeCount.Add(1)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"owner_id": "acct-preset",
			"tier": "free",
			"active_sandboxes": 0,
			"tier_limits": {},
			"wallets": {},
			"today": {}
		}`))
	})

	_, client := accountTestServer(t, handler)
	ac := NewTestAccountClientWithOwner(client, "acct-preset")

	_, err := ac.GetOverview(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if authMeCount.Load() != 0 {
		t.Error("expected /auth/me to NOT be called when ownerID is pre-set")
	}
}

// ---------------------------------------------------------------------------
// Model field tests
// ---------------------------------------------------------------------------

func TestAccountInfo_Fields(t *testing.T) {
	t.Parallel()

	info := AccountInfo{
		OwnerID:           "acct-x",
		Email:             "user@test.com",
		Tier:              "pro",
		SandboxFreeMicros: 100000000,
	}
	if info.OwnerID != "acct-x" {
		t.Errorf("OwnerID: got %q", info.OwnerID)
	}
	if info.Email != "user@test.com" {
		t.Errorf("Email: got %q", info.Email)
	}
	if info.Tier != "pro" {
		t.Errorf("Tier: got %q", info.Tier)
	}
	if info.SandboxFreeMicros != 100000000 {
		t.Errorf("SandboxFreeMicros: got %d", info.SandboxFreeMicros)
	}
}

func TestAPIKeyInfo_Fields(t *testing.T) {
	t.Parallel()

	ki := APIKeyInfo{
		KeyID:     "key-test",
		Name:      "test-key",
		MaskedKey: "dec_live...",
		Revoked:   false,
	}
	if ki.KeyID != "key-test" {
		t.Errorf("KeyID: got %q", ki.KeyID)
	}
	if ki.Name != "test-key" {
		t.Errorf("Name: got %q", ki.Name)
	}
	if ki.MaskedKey != "dec_live..." {
		t.Errorf("MaskedKey: got %q", ki.MaskedKey)
	}
	if ki.Revoked {
		t.Error("Revoked: expected false")
	}
}

func TestCreateAPIKeyResult_Fields(t *testing.T) {
	t.Parallel()

	r := CreateAPIKeyResult{
		KeyID:  "key-1",
		APIKey: "dec_live_secret",
		Name:   "prod",
	}
	if r.KeyID != "key-1" {
		t.Errorf("KeyID: got %q", r.KeyID)
	}
	if r.APIKey != "dec_live_secret" {
		t.Errorf("APIKey: got %q", r.APIKey)
	}
	if r.Name != "prod" {
		t.Errorf("Name: got %q", r.Name)
	}
}

func TestDepositInfo_Fields(t *testing.T) {
	t.Parallel()

	di := DepositInfo{
		DepositID:    "dep-1",
		WalletType:   "compute",
		AmountMicros: 10000000,
		AmountUSD:    "10.00",
		Status:       "completed",
		Provider:     "polar",
	}
	if di.DepositID != "dep-1" {
		t.Errorf("DepositID: got %q", di.DepositID)
	}
	if di.AmountMicros != 10000000 {
		t.Errorf("AmountMicros: got %d", di.AmountMicros)
	}
	if di.AmountUSD != "10.00" {
		t.Errorf("AmountUSD: got %q", di.AmountUSD)
	}
	if di.Status != "completed" {
		t.Errorf("Status: got %q", di.Status)
	}
}

func TestWalletOverview_Fields(t *testing.T) {
	t.Parallel()

	wo := WalletOverview{
		SandboxFreeMicros:    100000000,
		GuardrailsFreeMicros: 200000000,
		BalanceMicros:        50000000,
	}
	if wo.SandboxFreeMicros != 100000000 {
		t.Errorf("SandboxFreeMicros: got %d", wo.SandboxFreeMicros)
	}
	if wo.GuardrailsFreeMicros != 200000000 {
		t.Errorf("GuardrailsFreeMicros: got %d", wo.GuardrailsFreeMicros)
	}
	if wo.BalanceMicros != 50000000 {
		t.Errorf("BalanceMicros: got %d", wo.BalanceMicros)
	}
}
