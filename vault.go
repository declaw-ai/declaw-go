package declaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

// defaultTeamName / defaultEnvName are the hidden tenancy defaults. Teams,
// members, and environments are not yet exposed in the client surface (RBAC
// isn't built); every secret is stored under a single auto-provisioned "default"
// team and "prod" environment, and referenced by bare name. The backend keeps
// the full tenancy model, so this is purely a client-side simplification.
const (
	defaultTeamName = "default"
	defaultEnvName  = "prod"
)

// defaultTeamCache memoizes the resolved "default" team id per (baseURL, apiKey)
// so the resolution (a GET /teams, occasionally a create) happens once per
// account per process, not on every vault call or sandbox create.
var defaultTeamCache sync.Map // string -> string

// VaultClient is the control-plane client for the credential vault. Secrets are
// stored and referenced by name; the team/environment scoping is handled
// automatically (a single "default" team + "prod" environment per account). The
// secret value is written to the server (which stores it in OpenBao) and is
// never returned after create.
type VaultClient struct {
	client *apiClient
}

// NewVaultClient builds a VaultClient from the usual config options
// (WithAPIKey, WithDomain, WithAPIURL, …); env vars apply by default.
func NewVaultClient(opts ...ConfigOption) *VaultClient {
	return &VaultClient{client: newAPIClient(NewConfig(opts...))}
}

// Close releases client resources. It is safe to call multiple times.
func (v *VaultClient) Close() error { return nil }

// --- Types (wire shapes mirror the control-plane API) ---

// VaultScope is one per-destination injection rule on a secret. The egress
// proxy matches a request host against DomainRegex and injects the secret as
// InjectionType (bearer|header|basic|query|sigv4|oidc|hmac|redis|postgres|
// mysql|smtp|mongodb). The optional fields express a provider's full contract:
// ValuePrefix (scheme word for header), BasicUsername (for basic auth from a
// raw key), ExtraHeaders + QueryParams (static, non-secret).
type VaultScope struct {
	DomainRegex   string            `json:"domain_regex"`
	InjectionType string            `json:"injection_type,omitempty"`
	HeaderName    string            `json:"header_name,omitempty"`
	ValuePrefix   string            `json:"value_prefix,omitempty"`
	BasicUsername string            `json:"basic_username,omitempty"`
	ExtraHeaders  map[string]string `json:"extra_headers,omitempty"`
	QueryParams   map[string]string `json:"query_params,omitempty"`
}

// VaultSecret is the metadata for a stored secret — never the value. The value
// lives server-side (OpenBao) and is not returned after create.
type VaultSecret struct {
	SecretID             string       `json:"secret_id"`
	Name                 string       `json:"name"`
	Scopes               []VaultScope `json:"scopes,omitempty"`
	CreatedAt            time.Time    `json:"created_at"`
	UpdatedAt            time.Time    `json:"updated_at"`
	RotatedAt            *time.Time   `json:"rotated_at,omitempty"`
	RotationIntervalDays int          `json:"rotation_interval_days,omitempty"`
	RotationDue          bool         `json:"rotation_due,omitempty"`

	// teamID/envID are populated from the wire but not part of the public
	// secrets-by-name surface; they're used internally for addressing.
	teamID string
	envID  string
}

// UnmarshalJSON keeps team_id/env_id internal while parsing the wire shape.
func (s *VaultSecret) UnmarshalJSON(b []byte) error {
	var raw struct {
		SecretID             string       `json:"secret_id"`
		TeamID               string       `json:"team_id"`
		EnvID                string       `json:"env_id"`
		Name                 string       `json:"name"`
		Scopes               []VaultScope `json:"scopes,omitempty"`
		CreatedAt            time.Time    `json:"created_at"`
		UpdatedAt            time.Time    `json:"updated_at"`
		RotatedAt            *time.Time   `json:"rotated_at,omitempty"`
		RotationIntervalDays int          `json:"rotation_interval_days,omitempty"`
		RotationDue          bool         `json:"rotation_due,omitempty"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	s.SecretID, s.Name, s.Scopes = raw.SecretID, raw.Name, raw.Scopes
	s.CreatedAt, s.UpdatedAt, s.RotatedAt = raw.CreatedAt, raw.UpdatedAt, raw.RotatedAt
	s.RotationIntervalDays, s.RotationDue = raw.RotationIntervalDays, raw.RotationDue
	s.teamID, s.envID = raw.TeamID, raw.EnvID
	return nil
}

// VaultPreset is a built-in provider template (domain + injection rules), used
// so a caller can store a credential by naming the provider and supplying only
// the value. It carries no secret material.
type VaultPreset struct {
	Key      string       `json:"key"`
	Name     string       `json:"name"`
	Category string       `json:"category"`
	KeyHint  string       `json:"key_hint"`
	DocsURL  string       `json:"docs_url,omitempty"`
	Scopes   []VaultScope `json:"scopes"`
}

// CreateSecretInput is the payload for CreateSecret. Either Provider (a preset
// key, which supplies Scopes) or an explicit Scopes list is required. Name
// defaults to Provider when omitted. The team + environment are assigned
// automatically.
type CreateSecretInput struct {
	Name                 string       // secret name; defaults to Provider if empty
	Value                string       // the secret value (required; never returned)
	Provider             string       // optional preset key, e.g. "openai"
	Scopes               []VaultScope // required unless Provider is set
	RotationIntervalDays int          // optional rotation policy (0 = none)
}

// --- Internal tenancy resolution (team/env are not part of the public API) ---

type teamRec struct {
	TeamID    string    `json:"team_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type envRec struct {
	EnvID string `json:"env_id"`
	Name  string `json:"name"`
}

func cacheKey(c *apiClient) string { return c.config.BaseURL() + "\x00" + c.config.APIKey }

// resolveDefaultTeamID returns the account's "default" team id. When create is
// true it provisions the team if absent. Among duplicates (the backend doesn't
// enforce name-uniqueness) the oldest is chosen so all clients converge. Cached.
func resolveDefaultTeamID(ctx context.Context, c *apiClient, create bool) (string, error) {
	if v, ok := defaultTeamCache.Load(cacheKey(c)); ok {
		return v.(string), nil
	}
	b, err := c.get(ctx, "/teams")
	if err != nil {
		return "", err
	}
	var wrap struct {
		Teams []teamRec `json:"teams"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil {
		return "", fmt.Errorf("parsing teams: %w", err)
	}
	var best *teamRec
	for i := range wrap.Teams {
		if wrap.Teams[i].Name == defaultTeamName {
			if best == nil || wrap.Teams[i].CreatedAt.Before(best.CreatedAt) {
				best = &wrap.Teams[i]
			}
		}
	}
	if best != nil {
		defaultTeamCache.Store(cacheKey(c), best.TeamID)
		return best.TeamID, nil
	}
	if !create {
		return "", nil
	}
	tb, err := c.post(ctx, "/teams", map[string]any{"name": defaultTeamName})
	if err != nil {
		return "", err
	}
	var t teamRec
	if err := json.Unmarshal(tb, &t); err != nil {
		return "", fmt.Errorf("parsing team: %w", err)
	}
	defaultTeamCache.Store(cacheKey(c), t.TeamID)
	return t.TeamID, nil
}

// ensureDefaultEnv makes sure the "prod" environment exists on the team. A
// UNIQUE(team_id,name) conflict from a concurrent create is treated as success.
func ensureDefaultEnv(ctx context.Context, c *apiClient, teamID string) error {
	b, err := c.get(ctx, "/teams/"+url.PathEscape(teamID)+"/environments")
	if err != nil {
		return err
	}
	var wrap struct {
		Environments []envRec `json:"environments"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil {
		return fmt.Errorf("parsing environments: %w", err)
	}
	for _, e := range wrap.Environments {
		if e.Name == defaultEnvName {
			return nil
		}
	}
	if _, err := c.post(ctx, "/teams/"+url.PathEscape(teamID)+"/environments", map[string]any{"name": defaultEnvName}); err != nil {
		// A racing creator may have made it first; re-check before failing.
		b2, e2 := c.get(ctx, "/teams/"+url.PathEscape(teamID)+"/environments")
		if e2 == nil {
			var w2 struct {
				Environments []envRec `json:"environments"`
			}
			if json.Unmarshal(b2, &w2) == nil {
				for _, e := range w2.Environments {
					if e.Name == defaultEnvName {
						return nil
					}
				}
			}
		}
		return err
	}
	return nil
}

// --- Secrets (addressed by name) ---

// CreateSecret stores a secret's value (server-side, in OpenBao) plus its
// injection scopes, under the auto-provisioned default team + environment.
// Returns the metadata only — the value is never echoed.
func (v *VaultClient) CreateSecret(ctx context.Context, in CreateSecretInput) (*VaultSecret, error) {
	teamID, err := resolveDefaultTeamID(ctx, v.client, true)
	if err != nil {
		return nil, err
	}
	if err := ensureDefaultEnv(ctx, v.client, teamID); err != nil {
		return nil, err
	}
	body := map[string]any{
		"environment": defaultEnvName,
		"value":       in.Value,
	}
	if in.Name != "" {
		body["name"] = in.Name
	}
	if in.Provider != "" {
		body["provider"] = in.Provider
	}
	if len(in.Scopes) > 0 {
		body["scopes"] = in.Scopes
	}
	if in.RotationIntervalDays > 0 {
		body["rotation_interval_days"] = in.RotationIntervalDays
	}
	b, err := v.client.post(ctx, "/teams/"+url.PathEscape(teamID)+"/vault/secrets", body)
	if err != nil {
		return nil, err
	}
	var s VaultSecret
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parsing secret: %w", err)
	}
	return &s, nil
}

// ListSecrets returns the metadata for all stored secrets. Empty if nothing has
// been stored yet.
func (v *VaultClient) ListSecrets(ctx context.Context) ([]VaultSecret, error) {
	teamID, err := resolveDefaultTeamID(ctx, v.client, false)
	if err != nil {
		return nil, err
	}
	if teamID == "" {
		return []VaultSecret{}, nil
	}
	b, err := v.client.get(ctx, "/teams/"+url.PathEscape(teamID)+"/vault/secrets")
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Secrets []VaultSecret `json:"secrets"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil {
		return nil, fmt.Errorf("parsing secrets: %w", err)
	}
	return wrap.Secrets, nil
}

// resolveSecretID maps a secret name to its id within the default team.
func (v *VaultClient) resolveSecretID(ctx context.Context, teamID, name string) (string, error) {
	b, err := v.client.get(ctx, "/teams/"+url.PathEscape(teamID)+"/vault/secrets")
	if err != nil {
		return "", err
	}
	var wrap struct {
		Secrets []VaultSecret `json:"secrets"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil {
		return "", fmt.Errorf("parsing secrets: %w", err)
	}
	for _, s := range wrap.Secrets {
		if s.Name == name {
			return s.SecretID, nil
		}
	}
	return "", fmt.Errorf("vault secret %q not found", name)
}

// RotateSecret replaces a secret's value (by name); scopes are unchanged.
func (v *VaultClient) RotateSecret(ctx context.Context, name, value string) error {
	teamID, err := resolveDefaultTeamID(ctx, v.client, false)
	if err != nil {
		return err
	}
	if teamID == "" {
		return fmt.Errorf("vault secret %q not found", name)
	}
	id, err := v.resolveSecretID(ctx, teamID, name)
	if err != nil {
		return err
	}
	_, err = v.client.post(ctx,
		"/teams/"+url.PathEscape(teamID)+"/vault/secrets/"+url.PathEscape(id)+"/rotate",
		map[string]any{"value": value})
	return err
}

// DeleteSecret deletes a secret (by name) — metadata + stored value.
func (v *VaultClient) DeleteSecret(ctx context.Context, name string) error {
	teamID, err := resolveDefaultTeamID(ctx, v.client, false)
	if err != nil {
		return err
	}
	if teamID == "" {
		return fmt.Errorf("vault secret %q not found", name)
	}
	id, err := v.resolveSecretID(ctx, teamID, name)
	if err != nil {
		return err
	}
	_, err = v.client.delete(ctx, "/teams/"+url.PathEscape(teamID)+"/vault/secrets/"+url.PathEscape(id))
	return err
}

// UpdateScopes replaces a secret's injection scopes (by name); the value is
// unchanged. Use it to change a secret's destination(s) or injection format in
// place instead of delete + recreate. At least one scope is required.
func (v *VaultClient) UpdateScopes(ctx context.Context, name string, scopes []VaultScope) error {
	if len(scopes) == 0 {
		return fmt.Errorf("at least one scope is required")
	}
	teamID, err := resolveDefaultTeamID(ctx, v.client, false)
	if err != nil {
		return err
	}
	if teamID == "" {
		return fmt.Errorf("vault secret %q not found", name)
	}
	id, err := v.resolveSecretID(ctx, teamID, name)
	if err != nil {
		return err
	}
	_, err = v.client.post(ctx,
		"/teams/"+url.PathEscape(teamID)+"/vault/secrets/"+url.PathEscape(id)+"/scopes",
		map[string]any{"scopes": scopes})
	return err
}

// ListPresets returns the built-in provider catalog (templates only).
func (v *VaultClient) ListPresets(ctx context.Context) ([]VaultPreset, error) {
	b, err := v.client.get(ctx, "/vault/presets")
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Presets []VaultPreset `json:"presets"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil {
		return nil, fmt.Errorf("parsing presets: %w", err)
	}
	return wrap.Presets, nil
}

// expandVaultRefs rewrites bare secret names in a vault_refs map to full
// vault://<team>/<env>/<name> URIs the backend understands. Values already in
// vault:// form are passed through unchanged. Resolves the default team once.
func expandVaultRefs(ctx context.Context, c *apiClient, refs map[string]string) (map[string]string, error) {
	if len(refs) == 0 {
		return refs, nil
	}
	needsTeam := false
	for _, ref := range refs {
		if !strings.HasPrefix(ref, "vault://") {
			needsTeam = true
			break
		}
	}
	if !needsTeam {
		return refs, nil
	}
	teamID, err := resolveDefaultTeamID(ctx, c, false)
	if err != nil {
		return nil, err
	}
	if teamID == "" {
		return nil, fmt.Errorf("vault_refs given but no vault secrets exist for this account")
	}
	out := make(map[string]string, len(refs))
	for env, ref := range refs {
		if strings.HasPrefix(ref, "vault://") {
			out[env] = ref
			continue
		}
		out[env] = "vault://" + teamID + "/" + defaultEnvName + "/" + ref
	}
	return out, nil
}
