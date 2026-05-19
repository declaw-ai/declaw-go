package declaw

import (
	"context"
	"encoding/json"
	"fmt"
)

// Template provides operations for managing sandbox templates.
// Templates define the base environment (packages, files, startup commands)
// for sandbox instances.
type Template struct{}

// buildTemplateRequest builds the request body for a template build.
func buildTemplateRequest(spec TemplateSpec, background bool) map[string]interface{} {
	body := map[string]interface{}{
		"background": background,
	}

	if spec.BaseImage != "" {
		body["base_image"] = spec.BaseImage
	}
	if len(spec.RunCmds) > 0 {
		body["run_cmds"] = spec.RunCmds
	}
	if len(spec.Copies) > 0 {
		copies := make([]map[string]interface{}, len(spec.Copies))
		for i, c := range spec.Copies {
			copies[i] = map[string]interface{}{
				"src":  c.Src,
				"dst":  c.Dst,
				"mode": c.Mode,
			}
		}
		body["copies"] = copies
	}
	if len(spec.Envs) > 0 {
		body["envs"] = spec.Envs
	}
	if len(spec.AptPackages) > 0 {
		body["apt_packages"] = spec.AptPackages
	}
	if spec.StartCmd != "" {
		body["start_cmd"] = spec.StartCmd
	}
	if spec.Dockerfile != "" {
		body["dockerfile"] = spec.Dockerfile
	}
	if spec.DiskMB > 0 {
		body["disk_mb"] = spec.DiskMB
	}

	return body
}

// parseBuildInfo parses a BuildInfo from a JSON response body.
func parseBuildInfo(data []byte) (*BuildInfo, error) {
	var raw struct {
		BuildID    string `json:"build_id"`
		Status     string `json:"status"`
		TemplateID string `json:"template_id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing build info: %w", err)
	}
	return &BuildInfo{
		BuildID:    raw.BuildID,
		Status:     raw.Status,
		TemplateID: raw.TemplateID,
	}, nil
}

// BuildTemplate builds a new template from the given specification and waits
// for the build to complete. Returns the build result including the template ID.
func BuildTemplate(ctx context.Context, spec TemplateSpec, opts ...SandboxOption) (*BuildInfo, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	body := buildTemplateRequest(spec, false)

	respBody, err := client.post(ctx, "/templates/build", body)
	if err != nil {
		return nil, err
	}

	return parseBuildInfo(respBody)
}

// BuildTemplateBackground starts a template build and returns immediately
// without waiting for completion. Use GetBuildStatus to poll for progress.
func BuildTemplateBackground(ctx context.Context, spec TemplateSpec, opts ...SandboxOption) (*BuildInfo, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	body := buildTemplateRequest(spec, true)

	respBody, err := client.post(ctx, "/templates/build", body)
	if err != nil {
		return nil, err
	}

	return parseBuildInfo(respBody)
}

// GetBuildStatus returns the current status of a template build.
func GetBuildStatus(ctx context.Context, buildID string, opts ...SandboxOption) (*BuildInfo, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	path := fmt.Sprintf("/templates/builds/%s", buildID)
	respBody, err := client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	return parseBuildInfo(respBody)
}

// ListTemplates returns all templates owned by the authenticated user.
func ListTemplates(ctx context.Context, opts ...SandboxOption) ([]TemplateInfo, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	respBody, err := client.get(ctx, "/templates")
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Templates []struct {
			TemplateID string `json:"template_id"`
			Alias      string `json:"alias"`
			CreatedAt  string `json:"created_at"`
		} `json:"templates"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return nil, fmt.Errorf("parsing templates list: %w", err)
	}
	raw := wrapper.Templates

	templates := make([]TemplateInfo, len(raw))
	for i, r := range raw {
		templates[i] = TemplateInfo{
			TemplateID: r.TemplateID,
			Alias:      r.Alias,
			CreatedAt:  r.CreatedAt,
		}
	}
	return templates, nil
}

// GetTemplate returns information about a specific template.
func GetTemplate(ctx context.Context, templateID string, opts ...SandboxOption) (*TemplateInfo, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	path := fmt.Sprintf("/templates/%s", templateID)
	respBody, err := client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var raw struct {
		TemplateID string `json:"template_id"`
		Alias      string `json:"alias"`
		CreatedAt  string `json:"created_at"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("parsing template info: %w", err)
	}

	return &TemplateInfo{
		TemplateID: raw.TemplateID,
		Alias:      raw.Alias,
		CreatedAt:  raw.CreatedAt,
	}, nil
}

// DeleteTemplate deletes a template by its ID.
func DeleteTemplate(ctx context.Context, templateID string, opts ...SandboxOption) error {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	path := fmt.Sprintf("/templates/%s", templateID)
	_, err := client.delete(ctx, path)
	return err
}
