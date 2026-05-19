package declaw

import (
	"context"
	"io"
	"net/http"
	"time"
)

// ExportedErrorFromResponse exposes the internal errorFromResponse function for testing.
var ExportedErrorFromResponse = func(resp *http.Response, body []byte, sandboxID string) error {
	return errorFromResponse(resp, body, sandboxID)
}

// NewTestAPIClient exposes newAPIClient for testing.
func NewTestAPIClient(config *Config) *apiClient {
	return newAPIClient(config)
}

// TestableGet exposes the internal get method for testing.
func (c *apiClient) TestableGet(ctx context.Context, path string) ([]byte, error) {
	return c.get(ctx, path)
}

// TestablePost exposes the internal post method for testing.
func (c *apiClient) TestablePost(ctx context.Context, path string, body interface{}) ([]byte, error) {
	return c.post(ctx, path, body)
}

// TestablePatch exposes the internal patch method for testing.
func (c *apiClient) TestablePatch(ctx context.Context, path string, body interface{}) ([]byte, error) {
	return c.patch(ctx, path, body)
}

// TestablePut exposes the internal put method for testing.
func (c *apiClient) TestablePut(ctx context.Context, path string, body interface{}) ([]byte, error) {
	return c.put(ctx, path, body)
}

// TestablePostRaw exposes the internal postRaw method for testing.
func (c *apiClient) TestablePostRaw(ctx context.Context, path string, body []byte) ([]byte, error) {
	return c.postRaw(ctx, path, body)
}

// TestableDelete exposes the internal delete method for testing.
func (c *apiClient) TestableDelete(ctx context.Context, path string) ([]byte, error) {
	return c.delete(ctx, path)
}

// TestableStream exposes the internal stream method for testing.
func (c *apiClient) TestableStream(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	return c.stream(ctx, method, path, body)
}

// ExportedConfigFromSandboxOpts exposes configFromSandboxOpts for testing.
func ExportedConfigFromSandboxOpts(opts *sandboxOpts) *Config {
	return configFromSandboxOpts(opts)
}

// ExportedResolveSandboxOpts exposes resolveSandboxOpts for testing.
func ExportedResolveSandboxOpts(opts []SandboxOption) *sandboxOpts {
	return resolveSandboxOpts(opts)
}

// NewTestSandbox creates a Sandbox for testing with a given client and ID.
func NewTestSandbox(id string, client *apiClient) *Sandbox {
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

// GetClientConfig returns the config from an apiClient (for test assertions).
func (c *apiClient) GetClientConfig() *Config {
	return c.config
}

// GetMaxRetries returns the maxRetries from an apiClient (for test assertions).
func (c *apiClient) GetMaxRetries() int {
	return c.maxRetries
}

// GetRetryDelay returns the retryDelay from an apiClient (for test assertions).
func (c *apiClient) GetRetryDelay() time.Duration {
	return c.retryDelay
}

// SandboxOptsTemplate returns the Template field from sandboxOpts.
func (o *sandboxOpts) SandboxOptsTemplate() string { return o.Template }

// SandboxOptsTimeout returns the Timeout field from sandboxOpts.
func (o *sandboxOpts) SandboxOptsTimeout() int { return o.Timeout }

// SandboxOptsMetadata returns the Metadata field from sandboxOpts.
func (o *sandboxOpts) SandboxOptsMetadata() map[string]string { return o.Metadata }

// SandboxOptsEnvs returns the Envs field from sandboxOpts.
func (o *sandboxOpts) SandboxOptsEnvs() map[string]string { return o.Envs }

// SandboxOptsSecure returns the Secure field from sandboxOpts.
func (o *sandboxOpts) SandboxOptsSecure() *bool { return o.Secure }

// SandboxOptsNetwork returns the Network field from sandboxOpts.
func (o *sandboxOpts) SandboxOptsNetwork() *SandboxNetworkOpts { return o.Network }

// SandboxOptsSecurity returns the Security field from sandboxOpts.
func (o *sandboxOpts) SandboxOptsSecurity() *SecurityPolicy { return o.Security }

// SandboxOptsLifecycle returns the Lifecycle field from sandboxOpts.
func (o *sandboxOpts) SandboxOptsLifecycle() *SandboxLifecycle { return o.Lifecycle }

// SandboxOptsVolumes returns the Volumes field from sandboxOpts.
func (o *sandboxOpts) SandboxOptsVolumes() []VolumeAttachment { return o.Volumes }

// SandboxOptsAPIKey returns the APIKey field from sandboxOpts.
func (o *sandboxOpts) SandboxOptsAPIKey() string { return o.APIKey }

// SandboxOptsDomain returns the Domain field from sandboxOpts.
func (o *sandboxOpts) SandboxOptsDomain() string { return o.Domain }

// SandboxOptsAPIURL returns the APIURL field from sandboxOpts.
func (o *sandboxOpts) SandboxOptsAPIURL() string { return o.APIURL }

// NewTestCommands creates a Commands instance for testing with a given sandbox ID and client.
func NewTestCommands(sandboxID string, client *apiClient) *Commands {
	return &Commands{sandboxID: sandboxID, client: client}
}

// NewTestFilesystem creates a Filesystem instance for testing with a given sandbox ID and client.
func NewTestFilesystem(sandboxID string, client *apiClient) *Filesystem {
	return &Filesystem{sandboxID: sandboxID, client: client}
}

// NewTestCommandHandle creates a CommandHandle for testing with a given PID, sandbox ID, and client.
func NewTestCommandHandle(pid int, sandboxID string, client *apiClient) *CommandHandle {
	return &CommandHandle{PID: pid, sandboxID: sandboxID, client: client}
}

// NewTestPTY creates a PTY instance for testing with a given sandbox ID and client.
func NewTestPTY(sandboxID string, client *apiClient) *PTY {
	return &PTY{sandboxID: sandboxID, client: client}
}

// NewTestPtyHandle creates a PtyHandle for testing with a given PID, sandbox ID, and client.
func NewTestPtyHandle(pid int, sandboxID string, client *apiClient) *PtyHandle {
	return &PtyHandle{PID: pid, sandboxID: sandboxID, client: client}
}

// NewTestAccountClient creates an AccountClient for testing with a given apiClient.
func NewTestAccountClient(client *apiClient) *AccountClient {
	return &AccountClient{client: client}
}

// NewTestAccountClientWithOwner creates an AccountClient for testing with a pre-set ownerID.
func NewTestAccountClientWithOwner(client *apiClient, ownerID string) *AccountClient {
	return &AccountClient{client: client, ownerID: ownerID}
}

// GetOwnerID returns the ownerID cached on an AccountClient (for test assertions).
func (a *AccountClient) GetOwnerID() string {
	return a.ownerID
}

// SandboxOptWithAPIKey returns a SandboxOption that sets the API key (for external test packages).
func SandboxOptWithAPIKey(key string) SandboxOption {
	return func(o *sandboxOpts) {
		o.APIKey = key
	}
}

// SandboxOptWithAPIURL returns a SandboxOption that sets the API URL (for external test packages).
func SandboxOptWithAPIURL(url string) SandboxOption {
	return func(o *sandboxOpts) {
		o.APIURL = url
	}
}
