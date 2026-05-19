package declaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"
)

// AccountInfo contains information about a Declaw Cloud account.
type AccountInfo struct {
	OwnerID              string
	Email                string
	Tier                 string
	CreatedAt            time.Time
	SandboxFreeMicros    int64
	GuardrailsFreeMicros int64
	BalanceMicros        int64
}

// AccountOverview provides a high-level summary of an account.
type AccountOverview struct {
	OwnerID         string
	Tier            string
	ActiveSandboxes int
	TierLimits      TierLimits
	Wallets         WalletOverview
	Today           DailySpend
}

// TierLimits describes the resource limits for the account's tier.
type TierLimits struct {
	MaxConcurrent int
	MaxVCPUs      int
	MaxMemoryMB   int
}

// WalletOverview contains wallet balances using the waterfall credit model.
type WalletOverview struct {
	SandboxFreeMicros    int64
	GuardrailsFreeMicros int64
	BalanceMicros        int64
}

// DailySpend contains today's spending breakdown.
type DailySpend struct {
	ComputeCostMicros    int64
	GuardrailsCostMicros int64
	TotalCostMicros      int64
}

// UsageSummary provides aggregate usage for a time period.
type UsageSummary struct {
	OwnerID                          string
	Since                            time.Time
	TotalCostMicros                  int64
	TotalCostUSD                     string
	SandboxCount                     int
	TotalSeconds                     float64
	SandboxBalanceRemainingMicros    int64
	BalanceRemainingMicros           int64
	GuardrailsCostMicros             int64
	GuardrailsCostUSD                string
	GuardrailsBalanceRemainingMicros int64
}

// DepositInfo describes a single deposit transaction.
type DepositInfo struct {
	DepositID    string
	WalletType   string
	AmountMicros int64
	AmountUSD    string
	Status       string
	Provider     string
	CreatedAt    time.Time
	CompletedAt  *time.Time
}

// APIKeyInfo describes an API key returned by ListAPIKeys.
type APIKeyInfo struct {
	KeyID     string
	Name      string
	MaskedKey string
	CreatedAt time.Time
	Revoked   bool
}

// CreateAPIKeyResult is the result of creating a new API key.
// APIKey contains the raw key value and is only returned once at creation time.
type CreateAPIKeyResult struct {
	KeyID  string
	APIKey string
	Name   string
}

// AccountClient provides operations for managing Declaw Cloud accounts,
// wallets, usage, and API keys.
type AccountClient struct {
	client  *apiClient
	ownerID string
	ownerMu sync.Mutex
}

// NewAccountClient creates a new AccountClient with the given configuration options.
func NewAccountClient(opts ...ConfigOption) *AccountClient {
	cfg := NewConfig(opts...)
	return &AccountClient{
		client: newAPIClient(cfg),
	}
}

// ensureOwnerID fetches and caches the owner ID by calling /auth/me.
func (a *AccountClient) ensureOwnerID(ctx context.Context) error {
	a.ownerMu.Lock()
	defer a.ownerMu.Unlock()

	if a.ownerID != "" {
		return nil
	}

	respBody, err := a.client.get(ctx, "/auth/me")
	if err != nil {
		return err
	}

	var raw struct {
		OwnerID string `json:"owner_id"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return fmt.Errorf("parsing auth/me response: %w", err)
	}

	if raw.OwnerID == "" {
		return fmt.Errorf("auth/me returned empty owner_id")
	}

	a.ownerID = raw.OwnerID
	return nil
}

// GetAccount returns information about the authenticated account.
func (a *AccountClient) GetAccount(ctx context.Context) (*AccountInfo, error) {
	if err := a.ensureOwnerID(ctx); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/accounts/%s", a.ownerID)
	respBody, err := a.client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var raw struct {
		OwnerID              string `json:"owner_id"`
		Email                string `json:"email"`
		Tier                 string `json:"tier"`
		CreatedAt            string `json:"created_at"`
		SandboxFreeMicros    int64  `json:"sandbox_free_micros"`
		GuardrailsFreeMicros int64  `json:"guardrails_free_micros"`
		BalanceMicros        int64  `json:"balance_micros"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("parsing account info: %w", err)
	}

	info := &AccountInfo{
		OwnerID:              raw.OwnerID,
		Email:                raw.Email,
		Tier:                 raw.Tier,
		SandboxFreeMicros:    raw.SandboxFreeMicros,
		GuardrailsFreeMicros: raw.GuardrailsFreeMicros,
		BalanceMicros:        raw.BalanceMicros,
	}
	if raw.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, raw.CreatedAt); err == nil {
			info.CreatedAt = t
		}
	}

	return info, nil
}

// GetOverview returns a high-level overview of the authenticated account.
func (a *AccountClient) GetOverview(ctx context.Context) (*AccountOverview, error) {
	if err := a.ensureOwnerID(ctx); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/accounts/%s/overview", a.ownerID)
	respBody, err := a.client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var raw struct {
		OwnerID         string `json:"owner_id"`
		Tier            string `json:"tier"`
		ActiveSandboxes int    `json:"active_sandboxes"`
		TierLimits      struct {
			MaxConcurrent int `json:"max_concurrent"`
			MaxVCPUs      int `json:"max_vcpus"`
			MaxMemoryMB   int `json:"max_memory_mb"`
		} `json:"tier_limits"`
		Wallets struct {
			SandboxFreeMicros    int64 `json:"sandbox_free_micros"`
			GuardrailsFreeMicros int64 `json:"guardrails_free_micros"`
			BalanceMicros        int64 `json:"balance_micros"`
		} `json:"wallets"`
		Today struct {
			ComputeCostMicros    int64 `json:"compute_cost_micros"`
			GuardrailsCostMicros int64 `json:"guardrails_cost_micros"`
			TotalCostMicros      int64 `json:"total_cost_micros"`
		} `json:"today"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("parsing account overview: %w", err)
	}

	return &AccountOverview{
		OwnerID:         raw.OwnerID,
		Tier:            raw.Tier,
		ActiveSandboxes: raw.ActiveSandboxes,
		TierLimits: TierLimits{
			MaxConcurrent: raw.TierLimits.MaxConcurrent,
			MaxVCPUs:      raw.TierLimits.MaxVCPUs,
			MaxMemoryMB:   raw.TierLimits.MaxMemoryMB,
		},
		Wallets: WalletOverview{
			SandboxFreeMicros:    raw.Wallets.SandboxFreeMicros,
			GuardrailsFreeMicros: raw.Wallets.GuardrailsFreeMicros,
			BalanceMicros:        raw.Wallets.BalanceMicros,
		},
		Today: DailySpend{
			ComputeCostMicros:    raw.Today.ComputeCostMicros,
			GuardrailsCostMicros: raw.Today.GuardrailsCostMicros,
			TotalCostMicros:      raw.Today.TotalCostMicros,
		},
	}, nil
}

// GetUsage returns a usage summary for the authenticated account.
func (a *AccountClient) GetUsage(ctx context.Context) (*UsageSummary, error) {
	if err := a.ensureOwnerID(ctx); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/accounts/%s/usage", a.ownerID)
	respBody, err := a.client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var raw struct {
		OwnerID                          string  `json:"owner_id"`
		Since                            string  `json:"since"`
		TotalCostMicros                  int64   `json:"total_cost_micros"`
		TotalCostUSD                     string  `json:"total_cost_usd"`
		SandboxCount                     int     `json:"sandbox_count"`
		TotalSeconds                     float64 `json:"total_seconds"`
		SandboxBalanceRemainingMicros    int64   `json:"sandbox_balance_remaining_micros"`
		BalanceRemainingMicros           int64   `json:"balance_remaining_micros"`
		GuardrailsCostMicros             int64   `json:"guardrails_cost_micros"`
		GuardrailsCostUSD                string  `json:"guardrails_cost_usd"`
		GuardrailsBalanceRemainingMicros int64   `json:"guardrails_balance_remaining_micros"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("parsing usage summary: %w", err)
	}

	usage := &UsageSummary{
		OwnerID:                          raw.OwnerID,
		TotalCostMicros:                  raw.TotalCostMicros,
		TotalCostUSD:                     raw.TotalCostUSD,
		SandboxCount:                     raw.SandboxCount,
		TotalSeconds:                     raw.TotalSeconds,
		SandboxBalanceRemainingMicros:    raw.SandboxBalanceRemainingMicros,
		BalanceRemainingMicros:           raw.BalanceRemainingMicros,
		GuardrailsCostMicros:             raw.GuardrailsCostMicros,
		GuardrailsCostUSD:                raw.GuardrailsCostUSD,
		GuardrailsBalanceRemainingMicros: raw.GuardrailsBalanceRemainingMicros,
	}
	if raw.Since != "" {
		if t, err := time.Parse(time.RFC3339, raw.Since); err == nil {
			usage.Since = t
		}
	}

	return usage, nil
}

// ListDeposits returns deposit transactions for the authenticated account.
func (a *AccountClient) ListDeposits(ctx context.Context) ([]DepositInfo, error) {
	if err := a.ensureOwnerID(ctx); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/accounts/%s/deposits", a.ownerID)
	respBody, err := a.client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Entries []struct {
			DepositID    string `json:"deposit_id"`
			WalletType   string `json:"wallet_type"`
			AmountMicros int64  `json:"amount_micros"`
			AmountUSD    string `json:"amount_usd"`
			Status       string `json:"status"`
			Provider     string `json:"provider"`
			CreatedAt    string `json:"created_at"`
			CompletedAt  string `json:"completed_at"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("parsing deposits list: %w", err)
	}

	deposits := make([]DepositInfo, len(raw.Entries))
	for i, r := range raw.Entries {
		deposits[i] = DepositInfo{
			DepositID:    r.DepositID,
			WalletType:   r.WalletType,
			AmountMicros: r.AmountMicros,
			AmountUSD:    r.AmountUSD,
			Status:       r.Status,
			Provider:     r.Provider,
		}
		if r.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339, r.CreatedAt); err == nil {
				deposits[i].CreatedAt = t
			}
		}
		if r.CompletedAt != "" {
			if t, err := time.Parse(time.RFC3339, r.CompletedAt); err == nil {
				deposits[i].CompletedAt = &t
			}
		}
	}

	return deposits, nil
}

// CreateAPIKey creates a new API key with the given name.
// The returned CreateAPIKeyResult contains the raw API key which is only shown once.
func (a *AccountClient) CreateAPIKey(ctx context.Context, name string) (*CreateAPIKeyResult, error) {
	if err := a.ensureOwnerID(ctx); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/accounts/%s/api-keys", a.ownerID)
	body := map[string]interface{}{
		"name": name,
	}

	respBody, err := a.client.post(ctx, path, body)
	if err != nil {
		return nil, err
	}

	var raw struct {
		KeyID  string `json:"key_id"`
		APIKey string `json:"api_key"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("parsing api key response: %w", err)
	}

	return &CreateAPIKeyResult{
		KeyID:  raw.KeyID,
		APIKey: raw.APIKey,
		Name:   raw.Name,
	}, nil
}

// ListAPIKeys returns all API keys for the authenticated account.
func (a *AccountClient) ListAPIKeys(ctx context.Context) ([]APIKeyInfo, error) {
	if err := a.ensureOwnerID(ctx); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/accounts/%s/api-keys", a.ownerID)
	respBody, err := a.client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var raw struct {
		APIKeys []struct {
			KeyID     string `json:"key_id"`
			Name      string `json:"name"`
			MaskedKey string `json:"masked_key"`
			CreatedAt string `json:"created_at"`
			Revoked   bool   `json:"revoked"`
		} `json:"api_keys"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("parsing api keys list: %w", err)
	}

	keys := make([]APIKeyInfo, len(raw.APIKeys))
	for i, r := range raw.APIKeys {
		keys[i] = APIKeyInfo{
			KeyID:     r.KeyID,
			Name:      r.Name,
			MaskedKey: r.MaskedKey,
			Revoked:   r.Revoked,
		}
		if r.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339, r.CreatedAt); err == nil {
				keys[i].CreatedAt = t
			}
		}
	}

	return keys, nil
}

// RevokeAPIKey revokes an API key by its ID.
func (a *AccountClient) RevokeAPIKey(ctx context.Context, keyID string) error {
	if err := a.ensureOwnerID(ctx); err != nil {
		return err
	}

	path := fmt.Sprintf("/accounts/%s/api-keys/%s", a.ownerID, url.PathEscape(keyID))
	_, err := a.client.delete(ctx, path)
	return err
}

// Close releases any resources held by the AccountClient.
func (a *AccountClient) Close() error {
	return nil
}
