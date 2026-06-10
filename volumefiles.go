package declaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// VolumeFileEntry describes a single entry inside a file-granular volume.
type VolumeFileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
	Mode    int    `json:"mode"`
}

// VolumeFileInfo is the result of an Info/Stat call on a volume file. It embeds
// VolumeFileEntry and adds the CAS Version token to round-trip into a guarded
// write via WithIfVersion.
type VolumeFileInfo struct {
	VolumeFileEntry
	// Version is the optimistic-concurrency (CAS) token. Pass it to a write via
	// WithIfVersion to make the write conditional on the file being unchanged.
	Version string `json:"version"`
}

// writeFileOpts holds the resolved options for a volume file write.
type writeFileOpts struct {
	ifVersion string
	hasIf     bool
}

// WriteFileOption configures a volume file write.
type WriteFileOption func(*writeFileOpts)

// WithIfVersion makes a volume file write conditional (optimistic concurrency).
// The write only succeeds if the file's current CAS version matches version
// (obtained from VolumeFileInfo.Version). On mismatch the server returns 409 and
// the write fails with a *VersionMismatchError.
func WithIfVersion(version string) WriteFileOption {
	return func(o *writeFileOpts) {
		o.ifVersion = version
		o.hasIf = true
	}
}

// VolumeFiles is a handle to the file API of a single file-granular volume.
// Obtain one with VolumeFilesFor(volumeID). All methods require the volume to be
// on the file-granular backend; the server returns 409 for tarball-backed
// volumes and 503 when no file accessor is configured.
type VolumeFiles struct {
	volumeID string
	client   *apiClient
}

// VolumeFilesFor returns a file API handle for the given volume.
func VolumeFilesFor(volumeID string, opts ...SandboxOption) *VolumeFiles {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	return &VolumeFiles{volumeID: volumeID, client: newAPIClient(cfg)}
}

// Write writes raw bytes to path inside the volume. With WithIfVersion the write
// is conditional (CAS); a version mismatch returns a *VersionMismatchError.
func (f *VolumeFiles) Write(ctx context.Context, path string, data []byte, opts ...WriteFileOption) error {
	wo := &writeFileOpts{}
	for _, opt := range opts {
		opt(wo)
	}

	q := url.Values{}
	q.Set("path", path)
	if wo.hasIf {
		q.Set("if_version", wo.ifVersion)
	}
	reqPath := fmt.Sprintf("/volumes/%s/files/raw?%s", url.PathEscape(f.volumeID), q.Encode())

	_, err := f.client.putRaw(ctx, reqPath, data)
	if err != nil {
		if wo.hasIf {
			if ce, ok := err.(*ConflictError); ok {
				return &VersionMismatchError{ConflictError: ce}
			}
		}
		return err
	}
	return nil
}

// Read reads the raw bytes of the file at path inside the volume.
func (f *VolumeFiles) Read(ctx context.Context, path string) ([]byte, error) {
	reqPath := fmt.Sprintf("/volumes/%s/files/raw?path=%s", url.PathEscape(f.volumeID), url.QueryEscape(path))
	return f.client.get(ctx, reqPath)
}

// List lists the entries of the directory at path inside the volume.
func (f *VolumeFiles) List(ctx context.Context, path string) ([]VolumeFileEntry, error) {
	reqPath := fmt.Sprintf("/volumes/%s/files/list?path=%s", url.PathEscape(f.volumeID), url.QueryEscape(path))
	body, err := f.client.get(ctx, reqPath)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Entries []VolumeFileEntry `json:"entries"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("parsing volume file list: %w", err)
	}
	return wrapper.Entries, nil
}

// Info stats the file at path inside the volume, returning its metadata and CAS
// version token.
func (f *VolumeFiles) Info(ctx context.Context, path string) (*VolumeFileInfo, error) {
	reqPath := fmt.Sprintf("/volumes/%s/files/info?path=%s", url.PathEscape(f.volumeID), url.QueryEscape(path))
	body, err := f.client.get(ctx, reqPath)
	if err != nil {
		return nil, err
	}
	var info VolumeFileInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parsing volume file info: %w", err)
	}
	return &info, nil
}

// Exists reports whether path exists inside the volume.
func (f *VolumeFiles) Exists(ctx context.Context, path string) (bool, error) {
	reqPath := fmt.Sprintf("/volumes/%s/files/exists?path=%s", url.PathEscape(f.volumeID), url.QueryEscape(path))
	body, err := f.client.get(ctx, reqPath)
	if err != nil {
		return false, err
	}
	var wrapper struct {
		Exists bool `json:"exists"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return false, fmt.Errorf("parsing volume file exists: %w", err)
	}
	return wrapper.Exists, nil
}

// Remove deletes the entry at path inside the volume. With recursive=true a
// non-empty directory is removed.
func (f *VolumeFiles) Remove(ctx context.Context, path string, recursive bool) error {
	q := url.Values{}
	q.Set("path", path)
	if recursive {
		q.Set("recursive", "true")
	} else {
		q.Set("recursive", "false")
	}
	reqPath := fmt.Sprintf("/volumes/%s/files?%s", url.PathEscape(f.volumeID), q.Encode())
	_, err := f.client.delete(ctx, reqPath)
	return err
}

// Rename renames oldPath to newPath inside the volume.
func (f *VolumeFiles) Rename(ctx context.Context, oldPath, newPath string) error {
	reqPath := fmt.Sprintf("/volumes/%s/files", url.PathEscape(f.volumeID))
	body := map[string]string{"old_path": oldPath, "new_path": newPath}
	_, err := f.client.patch(ctx, reqPath, body)
	return err
}

// Mkdir creates the directory at path inside the volume.
func (f *VolumeFiles) Mkdir(ctx context.Context, path string) error {
	reqPath := fmt.Sprintf("/volumes/%s/files/mkdir", url.PathEscape(f.volumeID))
	body := map[string]string{"path": path}
	_, err := f.client.post(ctx, reqPath, body)
	return err
}

// ---------------------------------------------------------------------------
// Locks (advisory leases over a (volume, path))
// ---------------------------------------------------------------------------

// VolumeLock is the result of acquiring a lease on a volume path.
type VolumeLock struct {
	Token      string `json:"token"`
	TTLSeconds int    `json:"ttl_seconds"`
	ExpiresAt  string `json:"expires_at"`
}

// VolumeLockStatus reports whether a lease is currently held on a volume path.
type VolumeLockStatus struct {
	Held        bool  `json:"held"`
	ExpiresInMs int64 `json:"expires_in_ms"`
}

// VolumeLocks is a handle to the advisory-lock API of a single volume. Obtain
// one with VolumeLocksFor(volumeID).
type VolumeLocks struct {
	volumeID string
	client   *apiClient
}

// VolumeLocksFor returns a lock API handle for the given volume.
func VolumeLocksFor(volumeID string, opts ...SandboxOption) *VolumeLocks {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	return &VolumeLocks{volumeID: volumeID, client: newAPIClient(cfg)}
}

// Acquire takes an advisory lease on path. ttlSeconds <= 0 omits the TTL and
// lets the server pick its default. Returns a *ConflictError if the path is
// already locked.
func (l *VolumeLocks) Acquire(ctx context.Context, path string, ttlSeconds int) (*VolumeLock, error) {
	reqPath := fmt.Sprintf("/volumes/%s/locks", url.PathEscape(l.volumeID))
	body := map[string]interface{}{"path": path}
	if ttlSeconds > 0 {
		body["ttl_seconds"] = ttlSeconds
	}
	respBody, err := l.client.post(ctx, reqPath, body)
	if err != nil {
		return nil, err
	}
	var lock VolumeLock
	if err := json.Unmarshal(respBody, &lock); err != nil {
		return nil, fmt.Errorf("parsing volume lock: %w", err)
	}
	return &lock, nil
}

// Release releases the lease on path held under token. Returns a *ConflictError
// if the caller is not the current holder.
func (l *VolumeLocks) Release(ctx context.Context, path, token string) (bool, error) {
	reqPath := fmt.Sprintf("/volumes/%s/locks", url.PathEscape(l.volumeID))
	body := map[string]string{"path": path, "token": token}
	respBody, err := l.client.deleteJSON(ctx, reqPath, body)
	if err != nil {
		return false, err
	}
	var wrapper struct {
		Released bool `json:"released"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return false, fmt.Errorf("parsing volume lock release: %w", err)
	}
	return wrapper.Released, nil
}

// Renew extends the lease on path held under token. ttlSeconds <= 0 omits the
// TTL. Returns a *ConflictError if the caller is not the current holder.
func (l *VolumeLocks) Renew(ctx context.Context, path, token string, ttlSeconds int) error {
	reqPath := fmt.Sprintf("/volumes/%s/locks/renew", url.PathEscape(l.volumeID))
	body := map[string]interface{}{"path": path, "token": token}
	if ttlSeconds > 0 {
		body["ttl_seconds"] = ttlSeconds
	}
	_, err := l.client.post(ctx, reqPath, body)
	return err
}

// Status reports whether path is currently leased and the remaining TTL.
func (l *VolumeLocks) Status(ctx context.Context, path string) (*VolumeLockStatus, error) {
	reqPath := fmt.Sprintf("/volumes/%s/locks?path=%s", url.PathEscape(l.volumeID), url.QueryEscape(path))
	respBody, err := l.client.get(ctx, reqPath)
	if err != nil {
		return nil, err
	}
	var status VolumeLockStatus
	if err := json.Unmarshal(respBody, &status); err != nil {
		return nil, fmt.Errorf("parsing volume lock status: %w", err)
	}
	return &status, nil
}
