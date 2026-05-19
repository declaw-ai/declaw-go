package declaw

import "time"

// sandboxOpts holds the resolved options for sandbox creation.
type sandboxOpts struct {
	Template       string
	Timeout        int
	Metadata       map[string]string
	Envs           map[string]string
	Secure         *bool
	Network        *SandboxNetworkOpts
	Security       *SecurityPolicy
	Lifecycle      *SandboxLifecycle
	Volumes        []VolumeAttachment
	APIKey         string
	Domain         string
	APIURL         string
	RequestTimeout time.Duration
}

// SandboxOption is a functional option for configuring sandbox creation.
type SandboxOption func(*sandboxOpts)

// WithTemplate sets the template to use when creating a sandbox.
func WithTemplate(t string) SandboxOption {
	return func(o *sandboxOpts) {
		o.Template = t
	}
}

// WithTimeout sets the sandbox timeout in seconds.
func WithTimeout(secs int) SandboxOption {
	return func(o *sandboxOpts) {
		o.Timeout = secs
	}
}

// WithMetadata sets key-value metadata on the sandbox.
func WithMetadata(m map[string]string) SandboxOption {
	return func(o *sandboxOpts) {
		o.Metadata = m
	}
}

// WithEnvs sets environment variables inside the sandbox.
func WithEnvs(e map[string]string) SandboxOption {
	return func(o *sandboxOpts) {
		o.Envs = e
	}
}

// WithSecure enables or disables the security pipeline for the sandbox.
func WithSecure(s bool) SandboxOption {
	return func(o *sandboxOpts) {
		o.Secure = &s
	}
}

// WithNetwork configures sandbox network access.
func WithNetwork(n SandboxNetworkOpts) SandboxOption {
	return func(o *sandboxOpts) {
		o.Network = &n
	}
}

// WithSecurity configures the full security policy for the sandbox.
func WithSecurity(sp SecurityPolicy) SandboxOption {
	return func(o *sandboxOpts) {
		o.Security = &sp
	}
}

// WithLifecycle configures sandbox lifecycle behavior (e.g., on-timeout action).
func WithLifecycle(lc SandboxLifecycle) SandboxOption {
	return func(o *sandboxOpts) {
		o.Lifecycle = &lc
	}
}

// WithVolumes attaches persistent volumes to the sandbox.
func WithVolumes(v []VolumeAttachment) SandboxOption {
	return func(o *sandboxOpts) {
		o.Volumes = v
	}
}

// WithSandboxAPIKey sets the API key for the sandbox operation.
func WithSandboxAPIKey(key string) SandboxOption {
	return func(o *sandboxOpts) {
		o.APIKey = key
	}
}

// WithSandboxAPIURL sets the API URL for the sandbox operation.
func WithSandboxAPIURL(url string) SandboxOption {
	return func(o *sandboxOpts) {
		o.APIURL = url
	}
}

// WithSandboxDomain sets the API domain for the sandbox operation.
func WithSandboxDomain(domain string) SandboxOption {
	return func(o *sandboxOpts) {
		o.Domain = domain
	}
}

// runOpts holds the resolved options for command execution.
type runOpts struct {
	Envs     map[string]string
	User     string
	Cwd      string
	Stdin    bool
	Timeout  time.Duration
	OnStdout func(line string)
	OnStderr func(line string)
}

// RunOption is a functional option for configuring command execution.
type RunOption func(*runOpts)

// WithRunEnvs sets environment variables for the command.
func WithRunEnvs(e map[string]string) RunOption {
	return func(o *runOpts) {
		o.Envs = e
	}
}

// WithUser sets the user to run the command as.
func WithUser(u string) RunOption {
	return func(o *runOpts) {
		o.User = u
	}
}

// WithCwd sets the working directory for the command.
func WithCwd(d string) RunOption {
	return func(o *runOpts) {
		o.Cwd = d
	}
}

// WithStdin enables stdin for the command, allowing data to be sent via SendStdin.
func WithStdin() RunOption {
	return func(o *runOpts) {
		o.Stdin = true
	}
}

// WithRunTimeout sets the timeout for the command execution.
func WithRunTimeout(d time.Duration) RunOption {
	return func(o *runOpts) {
		o.Timeout = d
	}
}

// WithOnStdout sets a callback invoked for each line of stdout.
func WithOnStdout(f func(string)) RunOption {
	return func(o *runOpts) {
		o.OnStdout = f
	}
}

// WithOnStderr sets a callback invoked for each line of stderr.
func WithOnStderr(f func(string)) RunOption {
	return func(o *runOpts) {
		o.OnStderr = f
	}
}

// listOpts holds the resolved options for listing sandboxes.
type listOpts struct {
	State  SandboxState
	Limit  int
	Offset int
	APIKey string
	APIURL string
	Domain string
}

// ListOption is a functional option for configuring sandbox listing.
type ListOption func(*listOpts)

// WithState filters listed sandboxes by state.
func WithState(state SandboxState) ListOption {
	return func(o *listOpts) {
		o.State = state
	}
}

// WithLimit sets the maximum number of sandboxes to return.
func WithLimit(n int) ListOption {
	return func(o *listOpts) {
		o.Limit = n
	}
}

// WithOffset sets the offset for paginating through sandbox results.
func WithOffset(n int) ListOption {
	return func(o *listOpts) {
		o.Offset = n
	}
}

// WithListAPIKey sets the API key for listing sandboxes.
func WithListAPIKey(key string) ListOption {
	return func(o *listOpts) {
		o.APIKey = key
	}
}

// WithListAPIURL sets the API URL for listing sandboxes.
func WithListAPIURL(url string) ListOption {
	return func(o *listOpts) {
		o.APIURL = url
	}
}

// WithListDomain sets the API domain for listing sandboxes.
func WithListDomain(domain string) ListOption {
	return func(o *listOpts) {
		o.Domain = domain
	}
}

// fileOpts holds the resolved options for file operations.
type fileOpts struct {
	User string
}

// FileOption is a functional option for configuring file operations.
type FileOption func(*fileOpts)

// WithFileUser sets the user for file operations.
func WithFileUser(u string) FileOption {
	return func(o *fileOpts) {
		o.User = u
	}
}

// restoreOpts holds the resolved options for sandbox restore.
type restoreOpts struct {
	SnapshotID string
	APIKey     string
	APIURL     string
}

// RestoreOption is a functional option for configuring sandbox restore.
type RestoreOption func(*restoreOpts)

// WithSnapshotID specifies which snapshot to restore from.
func WithSnapshotID(id string) RestoreOption {
	return func(o *restoreOpts) {
		o.SnapshotID = id
	}
}

// WithRestoreAPIKey sets the API key for restoring a sandbox.
func WithRestoreAPIKey(key string) RestoreOption {
	return func(o *restoreOpts) {
		o.APIKey = key
	}
}

// WithRestoreAPIURL sets the API URL for restoring a sandbox.
func WithRestoreAPIURL(url string) RestoreOption {
	return func(o *restoreOpts) {
		o.APIURL = url
	}
}
