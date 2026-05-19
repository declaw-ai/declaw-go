package declaw

import "time"

// SandboxState represents the lifecycle state of a sandbox.
type SandboxState string

const (
	// StateLive indicates the sandbox is live and ready.
	StateLive SandboxState = "live"

	// StatePaused indicates the sandbox is paused (snapshot taken).
	StatePaused SandboxState = "paused"

	// StateKilled indicates the sandbox has been terminated.
	StateKilled SandboxState = "killed"
)

// SandboxInfo contains metadata about a sandbox instance.
type SandboxInfo struct {
	SandboxID  string
	TemplateID string
	Name       string
	Metadata   map[string]string
	StartedAt  *time.Time
	EndAt      *time.Time
	State      SandboxState
}

// SandboxMetrics contains resource usage metrics for a sandbox.
type SandboxMetrics struct {
	Timestamp       time.Time
	CPUUsagePercent float64
	MemoryUsageMB   float64
	DiskUsageMB     float64
}

// CommandResult is the output of a completed command execution.
// PID is only populated for background commands started via Commands.Start.
type CommandResult struct {
	PID      int    `json:"pid"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// ProcessInfo describes a running process inside a sandbox.
type ProcessInfo struct {
	PID   int               `json:"pid"`
	Cmd   string            `json:"cmd"`
	IsPty bool              `json:"is_pty"`
	Envs  map[string]string `json:"envs,omitempty"`
}

// FileType represents the type of a filesystem entry.
type FileType string

const (
	// FileTypeFile is a regular file.
	FileTypeFile FileType = "file"

	// FileTypeDirectory is a directory.
	FileTypeDirectory FileType = "dir"

	// FileTypeSymlink is a symbolic link.
	FileTypeSymlink FileType = "symlink"

	// FileTypeOther is any other type of filesystem entry.
	FileTypeOther FileType = "other"
)

// EntryInfo describes a filesystem entry inside a sandbox.
type EntryInfo struct {
	Path string   `json:"path"`
	Type FileType `json:"type"`
	Size int64    `json:"size"`
}

// WriteInfo is the result of a file write operation.
type WriteInfo struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// SnapshotInfo contains metadata about a sandbox snapshot.
type SnapshotInfo struct {
	SnapshotID string
	SandboxID  string
	CreatedAt  *time.Time
}

// Snapshot contains detailed information about a sandbox snapshot including
// internal blob storage keys and performance metrics.
type Snapshot struct {
	SnapshotID      string
	SandboxID       string
	Source          string // "periodic", "pause", or "manual"
	MemBlobKey      string
	VMStateBlobKey  string
	MemSizeBytes    *int64
	PauseDurationMs *int64
	CreatedAt       string
}

// PtySize specifies the dimensions of a pseudo-terminal.
type PtySize struct {
	Cols int
	Rows int
}

// SandboxLifecycle configures sandbox behavior on timeout.
type SandboxLifecycle struct {
	// OnTimeout specifies what happens when the sandbox times out: "kill" or "pause".
	OnTimeout string `json:"on_timeout,omitempty"`

	// AutoResume specifies whether a paused sandbox should auto-resume on API access.
	AutoResume bool `json:"auto_resume,omitempty"`
}

// VolumeAttachment describes a volume mounted to a sandbox.
type VolumeAttachment struct {
	VolumeID  string `json:"volume_id"`
	MountPath string `json:"mount_path"`
}

// VolumeInfo contains metadata about a persistent volume.
type VolumeInfo struct {
	VolumeID    string
	OwnerID     string
	Name        string
	BlobKey     string
	SizeBytes   int64
	ContentType string
	CreatedAt   string
	Metadata    map[string]string
}

// TemplateSpec defines how to build a sandbox template.
type TemplateSpec struct {
	// BaseImage is the base Docker/rootfs image name.
	BaseImage string

	// RunCmds are commands executed during the build.
	RunCmds []string

	// Copies are files to copy into the template.
	Copies []CopyItem

	// Envs are environment variables baked into the template.
	Envs map[string]string

	// AptPackages are apt packages to install during build.
	AptPackages []string

	// StartCmd is the command run when the sandbox starts.
	StartCmd string

	// Dockerfile is a raw Dockerfile to use instead of the structured fields.
	Dockerfile string

	// DiskMB is the disk size in megabytes for the template.
	DiskMB int
}

// CopyItem describes a file to copy into a template during build.
type CopyItem struct {
	Src  string
	Dst  string
	Mode string
}

// BuildInfo describes the status of a template build.
type BuildInfo struct {
	BuildID    string
	Status     string // "queued", "building", "success", or "failed"
	TemplateID string
}

// TemplateInfo contains metadata about a template.
type TemplateInfo struct {
	TemplateID string
	Alias      string
	CreatedAt  string
}

// SandboxPage is a paginated list of sandboxes.
type SandboxPage struct {
	Sandboxes []SandboxInfo
	Total     int
}

// SnapshotPage is a paginated list of snapshots.
type SnapshotPage struct {
	Snapshots []SnapshotInfo
}

// KillResult is the result of killing a single sandbox in a batch operation.
type KillResult struct {
	SandboxID string
	Error     error
}
