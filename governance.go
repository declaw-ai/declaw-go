package declaw

import (
	"context"
	"encoding/json"
	"fmt"
)

// GovernanceControl describes an enforced security control within a governance pack.
// Each control maps to a specific gate and provides a remediation rule and playbook.
type GovernanceControl struct {
	Control  string `json:"control"`
	Gate     string `json:"gate"`
	Rule     string `json:"rule"`
	Playbook string `json:"playbook"`
}

// GovernanceAdvisory describes a security control that is advisory (not enforced)
// within a governance pack, with a reason it is not enforced.
type GovernanceAdvisory struct {
	Control string `json:"control"`
	Reason  string `json:"reason"`
}

// GovernancePack describes a pre-built security governance pack that can be
// applied to sandboxes. Packs bundle a set of enforced and advisory controls
// aligned to a compliance framework (e.g. OWASP Top 10 for LLM Applications).
type GovernancePack struct {
	Name        string               `json:"name"`
	Version     string               `json:"version"`
	Framework   string               `json:"framework"`
	Description string               `json:"description"`
	Gates       []string             `json:"gates"`
	Enforces    []GovernanceControl  `json:"enforces"`
	Advisory    []GovernanceAdvisory `json:"advisory"`
	PolicyRef   string               `json:"policy_ref"`
	Seeded      bool                 `json:"seeded"`
}

// ListGovernancePacks returns all available governance packs from the platform.
// The endpoint is public; authentication headers are still included if an API
// key is configured (matching the SDK's standard behaviour for other requests).
func ListGovernancePacks(ctx context.Context, opts ...SandboxOption) ([]GovernancePack, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	respBody, err := client.get(ctx, "/governance/packs")
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Packs []GovernancePack `json:"packs"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return nil, fmt.Errorf("parsing governance packs list: %w", err)
	}

	return wrapper.Packs, nil
}

// GetGovernancePack returns a single governance pack by name.
func GetGovernancePack(ctx context.Context, name string, opts ...SandboxOption) (*GovernancePack, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	path := fmt.Sprintf("/governance/packs/%s", name)
	respBody, err := client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var pack GovernancePack
	if err := json.Unmarshal(respBody, &pack); err != nil {
		return nil, fmt.Errorf("parsing governance pack: %w", err)
	}

	return &pack, nil
}
