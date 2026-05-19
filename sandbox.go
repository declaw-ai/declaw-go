package declaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Sandbox represents a running Declaw sandbox instance.
// Use the package-level Create or Connect functions to obtain a Sandbox.
//
// Sub-objects Commands, Files, and PTY provide access to the sandbox's
// command execution, filesystem, and pseudo-terminal capabilities.
type Sandbox struct {
	// ID is the unique identifier for this sandbox.
	ID string

	// Commands provides command execution inside the sandbox.
	Commands *Commands

	// Files provides filesystem operations inside the sandbox.
	Files *Filesystem

	// PTY provides pseudo-terminal access to the sandbox.
	PTY *PTY

	client             *apiClient
	envdAccessToken    string
	sandboxDomain      string
	trafficAccessToken string
}

// newSandbox creates a fully wired Sandbox with sub-objects.
func newSandbox(id string, client *apiClient) *Sandbox {
	return &Sandbox{
		ID:     id,
		client: client,
		Commands: &Commands{
			sandboxID: id,
			client:    client,
		},
		Files: &Filesystem{
			sandboxID: id,
			client:    client,
		},
		PTY: &PTY{
			sandboxID: id,
			client:    client,
		},
	}
}

// validateSandboxID checks that a sandbox ID is safe to use in URL paths.
func validateSandboxID(id string) error {
	if id == "" {
		return fmt.Errorf("sandbox ID must not be empty")
	}
	if strings.Contains(id, "..") {
		return fmt.Errorf("sandbox ID contains invalid characters: %q", id)
	}
	if strings.Contains(id, "/") {
		return fmt.Errorf("sandbox ID contains invalid characters: %q", id)
	}
	if strings.Contains(id, " ") {
		return fmt.Errorf("sandbox ID contains invalid characters: %q", id)
	}
	return nil
}

// createRequest is the JSON body sent to POST /sandboxes.
type createRequest struct {
	Template  string                 `json:"template"`
	Timeout   int                    `json:"timeout"`
	Metadata  map[string]string      `json:"metadata,omitempty"`
	Envs      map[string]string      `json:"envs,omitempty"`
	Secure    *bool                  `json:"secure,omitempty"`
	Network   *SandboxNetworkOpts    `json:"network,omitempty"`
	Security  map[string]interface{} `json:"security,omitempty"`
	Lifecycle *SandboxLifecycle      `json:"lifecycle,omitempty"`
	Volumes   []VolumeAttachment     `json:"volumes,omitempty"`
}

// createResponse is the JSON response from POST /sandboxes.
type createResponse struct {
	SandboxID          string `json:"sandbox_id"`
	EnvdAccessToken    string `json:"envd_access_token"`
	SandboxDomain      string `json:"sandbox_domain"`
	TrafficAccessToken string `json:"traffic_access_token"`
}

// Create creates a new sandbox with the given options.
// The sandbox is ready for use when this function returns.
func Create(ctx context.Context, opts ...SandboxOption) (*Sandbox, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	// Apply defaults.
	template := o.Template
	if template == "" {
		template = "base"
	}
	timeout := o.Timeout
	if timeout == 0 {
		timeout = 300
	}

	body := createRequest{
		Template:  template,
		Timeout:   timeout,
		Metadata:  o.Metadata,
		Envs:      o.Envs,
		Secure:    o.Secure,
		Network:   o.Network,
		Lifecycle: o.Lifecycle,
		Volumes:   o.Volumes,
	}
	if o.Security != nil {
		body.Security = o.Security.ToJSON()
	}

	data, err := client.post(ctx, "/sandboxes", body)
	if err != nil {
		return nil, err
	}

	var resp createResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing create response: %w", err)
	}

	sbx := newSandbox(resp.SandboxID, client)
	sbx.envdAccessToken = resp.EnvdAccessToken
	sbx.sandboxDomain = resp.SandboxDomain
	sbx.trafficAccessToken = resp.TrafficAccessToken
	return sbx, nil
}

// connectResponse is the JSON response from GET /sandboxes/{id}.
type connectResponse struct {
	SandboxID          string            `json:"sandbox_id"`
	TemplateID         string            `json:"template_id"`
	Name               string            `json:"name"`
	Metadata           map[string]string `json:"metadata"`
	State              SandboxState      `json:"state"`
	EnvdAccessToken    string            `json:"envd_access_token"`
	SandboxDomain      string            `json:"sandbox_domain"`
	TrafficAccessToken string            `json:"traffic_access_token"`
}

// Connect connects to an existing running sandbox by its ID.
func Connect(ctx context.Context, sandboxID string, opts ...SandboxOption) (*Sandbox, error) {
	if err := validateSandboxID(sandboxID); err != nil {
		return nil, err
	}

	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	path := fmt.Sprintf("/sandboxes/%s", sandboxID)
	data, err := client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var resp connectResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing connect response: %w", err)
	}

	sbx := newSandbox(resp.SandboxID, client)
	// The server redacts envd_access_token and traffic_access_token on GET
	// (only Create returns them). sandbox_domain is always present.
	sbx.sandboxDomain = resp.SandboxDomain
	return sbx, nil
}

// resolveListOpts applies all ListOption functions and returns the resolved options.
func resolveListOpts(opts []ListOption) *listOpts {
	o := &listOpts{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// listResponse is the JSON response from GET /sandboxes.
type listResponse struct {
	Sandboxes []sandboxInfoJSON `json:"sandboxes"`
	Total     int               `json:"total"`
}

// sandboxInfoJSON is the JSON representation of sandbox info from the API.
type sandboxInfoJSON struct {
	SandboxID  string            `json:"sandbox_id"`
	TemplateID string            `json:"template_id"`
	Name       string            `json:"name"`
	Metadata   map[string]string `json:"metadata"`
	StartedAt  *time.Time        `json:"started_at"`
	EndAt      *time.Time        `json:"end_at"`
	State      SandboxState      `json:"state"`
}

// configFromListOpts creates a Config by merging list options with defaults.
func configFromListOpts(o *listOpts) *Config {
	cfg := NewConfig()
	if o.APIKey != "" {
		cfg.APIKey = o.APIKey
	}
	if o.APIURL != "" {
		cfg.APIURL = o.APIURL
	}
	return cfg
}

// ListSandboxes returns a paginated list of sandboxes matching the given filters.
func ListSandboxes(ctx context.Context, opts ...ListOption) (*SandboxPage, error) {
	o := resolveListOpts(opts)
	cfg := configFromListOpts(o)
	client := newAPIClient(cfg)

	params := url.Values{}
	if o.State != "" {
		params.Set("state", string(o.State))
	}
	if o.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", o.Limit))
	}
	if o.Offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", o.Offset))
	}

	path := "/sandboxes"
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}

	data, err := client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var resp listResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing list response: %w", err)
	}

	page := &SandboxPage{
		Total: resp.Total,
	}
	for _, s := range resp.Sandboxes {
		page.Sandboxes = append(page.Sandboxes, SandboxInfo{
			SandboxID:  s.SandboxID,
			TemplateID: s.TemplateID,
			Name:       s.Name,
			Metadata:   s.Metadata,
			StartedAt:  s.StartedAt,
			EndAt:      s.EndAt,
			State:      s.State,
		})
	}
	return page, nil
}

// KillSandbox terminates a sandbox by its ID.
func KillSandbox(ctx context.Context, sandboxID string, opts ...SandboxOption) error {
	if err := validateSandboxID(sandboxID); err != nil {
		return err
	}

	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	path := fmt.Sprintf("/sandboxes/%s?async=true", sandboxID)
	_, err := client.delete(ctx, path)
	return err
}

// killManyRequest is the JSON body sent to POST /sandboxes/kill-many.
type killManyRequest struct {
	SandboxIDs []string `json:"sandbox_ids"`
}

// killManyResponse is the JSON response from POST /sandboxes/kill-many.
// The server returns a map keyed by sandbox ID, e.g.:
//
//	{"results": {"sbx-1": {"killed": true}, "sbx-2": {"error": "not found"}}}
type killManyResponse struct {
	Results map[string]killManyResultJSON `json:"results"`
}

// killManyResultJSON is a single result in the kill-many response map.
type killManyResultJSON struct {
	Killed bool   `json:"killed,omitempty"`
	Queued bool   `json:"queued,omitempty"`
	Error  string `json:"error,omitempty"`
}

// KillManySandboxes terminates multiple sandboxes by their IDs.
// It returns a KillResult for each sandbox, including any errors.
func KillManySandboxes(ctx context.Context, ids []string, opts ...SandboxOption) ([]KillResult, error) {
	if len(ids) == 0 {
		return []KillResult{}, nil
	}

	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	body := killManyRequest{SandboxIDs: ids}
	data, err := client.post(ctx, "/sandboxes/kill-many?async=true", body)
	if err != nil {
		return nil, err
	}

	var resp killManyResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing kill-many response: %w", err)
	}

	results := make([]KillResult, 0, len(ids))
	for _, id := range ids {
		kr := KillResult{SandboxID: id}
		if r, ok := resp.Results[id]; ok && r.Error != "" {
			kr.Error = fmt.Errorf("%s", r.Error)
		}
		results = append(results, kr)
	}
	return results, nil
}

// resolveRestoreOpts applies all RestoreOption functions and returns the resolved options.
func resolveRestoreOpts(opts []RestoreOption) *restoreOpts {
	o := &restoreOpts{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// configFromRestoreOpts creates a Config by merging restore options with defaults.
func configFromRestoreOpts(o *restoreOpts) *Config {
	cfg := NewConfig()
	if o.APIKey != "" {
		cfg.APIKey = o.APIKey
	}
	if o.APIURL != "" {
		cfg.APIURL = o.APIURL
	}
	return cfg
}

// Restore restores a sandbox from a snapshot.
func Restore(ctx context.Context, sandboxID string, opts ...RestoreOption) (*Sandbox, error) {
	if err := validateSandboxID(sandboxID); err != nil {
		return nil, err
	}

	o := resolveRestoreOpts(opts)
	cfg := configFromRestoreOpts(o)
	client := newAPIClient(cfg)

	path := fmt.Sprintf("/sandboxes/%s/restore", sandboxID)
	if o.SnapshotID != "" {
		path += "?snapshot_id=" + url.QueryEscape(o.SnapshotID)
	}

	data, err := client.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		SandboxID  string `json:"sandbox_id"`
		NodeID     string `json:"node_id"`
		SnapshotID string `json:"snapshot_id"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing restore response: %w", err)
	}

	// Restore does not return tokens or domain — only Create does.
	sbx := newSandbox(resp.SandboxID, client)
	return sbx, nil
}

// Kill terminates this sandbox.
func (s *Sandbox) Kill(ctx context.Context) error {
	path := fmt.Sprintf("/sandboxes/%s?async=true", s.ID)
	_, err := s.client.delete(ctx, path)
	return err
}

// statusResponse is the JSON response from GET /sandboxes/{id}/status.
type statusResponse struct {
	IsRunning bool `json:"is_running"`
}

// IsRunning returns true if the sandbox is currently in a running/live state.
func (s *Sandbox) IsRunning(ctx context.Context) (bool, error) {
	path := fmt.Sprintf("/sandboxes/%s/status", s.ID)
	data, err := s.client.get(ctx, path)
	if err != nil {
		return false, err
	}

	var resp statusResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return false, fmt.Errorf("parsing status response: %w", err)
	}

	return resp.IsRunning, nil
}

// timeoutRequest is the JSON body sent to PATCH /sandboxes/{id}/timeout.
type timeoutRequest struct {
	Timeout int `json:"timeout"`
}

// SetTimeout updates the sandbox timeout. The sandbox will be killed or paused
// (depending on lifecycle configuration) after the specified number of seconds.
func (s *Sandbox) SetTimeout(ctx context.Context, seconds int) error {
	path := fmt.Sprintf("/sandboxes/%s/timeout", s.ID)
	body := timeoutRequest{Timeout: seconds}
	_, err := s.client.patch(ctx, path, body)
	return err
}

// sandboxInfoResponse is the JSON response from GET /sandboxes/{id} for GetInfo.
type sandboxInfoResponse struct {
	SandboxID  string            `json:"sandbox_id"`
	TemplateID string            `json:"template_id"`
	Name       string            `json:"name"`
	Metadata   map[string]string `json:"metadata"`
	StartedAt  *time.Time        `json:"started_at"`
	EndAt      *time.Time        `json:"end_at"`
	State      SandboxState      `json:"state"`
}

// GetInfo returns metadata about this sandbox.
func (s *Sandbox) GetInfo(ctx context.Context) (*SandboxInfo, error) {
	path := fmt.Sprintf("/sandboxes/%s", s.ID)
	data, err := s.client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var resp sandboxInfoResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing info response: %w", err)
	}

	return &SandboxInfo{
		SandboxID:  resp.SandboxID,
		TemplateID: resp.TemplateID,
		Name:       resp.Name,
		Metadata:   resp.Metadata,
		StartedAt:  resp.StartedAt,
		EndAt:      resp.EndAt,
		State:      resp.State,
	}, nil
}

// metricsResponse is the JSON response from GET /sandboxes/{id}/metrics.
type metricsResponse struct {
	Timestamp       time.Time `json:"timestamp"`
	CPUUsagePercent float64   `json:"cpu_usage_percent"`
	MemoryUsageMB   float64   `json:"memory_usage_mb"`
	DiskUsageMB     float64   `json:"disk_usage_mb"`
}

// GetMetrics returns current resource usage metrics for this sandbox.
func (s *Sandbox) GetMetrics(ctx context.Context) (*SandboxMetrics, error) {
	path := fmt.Sprintf("/sandboxes/%s/metrics", s.ID)
	data, err := s.client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var resp metricsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing metrics response: %w", err)
	}

	return &SandboxMetrics{
		Timestamp:       resp.Timestamp,
		CPUUsagePercent: resp.CPUUsagePercent,
		MemoryUsageMB:   resp.MemoryUsageMB,
		DiskUsageMB:     resp.DiskUsageMB,
	}, nil
}

// Pause pauses this sandbox, taking a snapshot of its state.
func (s *Sandbox) Pause(ctx context.Context) error {
	path := fmt.Sprintf("/sandboxes/%s/pause", s.ID)
	_, err := s.client.post(ctx, path, nil)
	return err
}

// Resume resumes a previously paused sandbox.
func (s *Sandbox) Resume(ctx context.Context) error {
	path := fmt.Sprintf("/sandboxes/%s/resume", s.ID)
	_, err := s.client.post(ctx, path, nil)
	return err
}

// snapshotResponse is the JSON response from POST /sandboxes/{id}/snapshot.
type snapshotResponse struct {
	SnapshotID string     `json:"snapshot_id"`
	SandboxID  string     `json:"sandbox_id"`
	CreatedAt  *time.Time `json:"created_at"`
}

// CreateSnapshot creates a snapshot of this sandbox's current state.
func (s *Sandbox) CreateSnapshot(ctx context.Context) (*SnapshotInfo, error) {
	path := fmt.Sprintf("/sandboxes/%s/snapshot", s.ID)
	data, err := s.client.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	var resp snapshotResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing snapshot response: %w", err)
	}

	return &SnapshotInfo{
		SnapshotID: resp.SnapshotID,
		SandboxID:  resp.SandboxID,
		CreatedAt:  resp.CreatedAt,
	}, nil
}

// ListSnapshots returns all snapshots for this sandbox.
func (s *Sandbox) ListSnapshots(ctx context.Context) ([]SnapshotInfo, error) {
	path := fmt.Sprintf("/sandboxes/%s/snapshots", s.ID)
	data, err := s.client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Snapshots []snapshotResponse `json:"snapshots"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("parsing snapshots response: %w", err)
	}

	result := make([]SnapshotInfo, len(wrapper.Snapshots))
	for i, snap := range wrapper.Snapshots {
		result[i] = SnapshotInfo{
			SnapshotID: snap.SnapshotID,
			SandboxID:  snap.SandboxID,
			CreatedAt:  snap.CreatedAt,
		}
	}
	return result, nil
}

// DeleteSnapshot deletes a snapshot by its ID.
func (s *Sandbox) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	if snapshotID == "" {
		return fmt.Errorf("snapshot ID must not be empty")
	}
	if strings.Contains(snapshotID, "..") || strings.Contains(snapshotID, "/") || strings.Contains(snapshotID, " ") {
		return fmt.Errorf("snapshot ID contains invalid characters: %q", snapshotID)
	}
	path := fmt.Sprintf("/sandboxes/%s/snapshots/%s", s.ID, snapshotID)
	_, err := s.client.delete(ctx, path)
	return err
}
