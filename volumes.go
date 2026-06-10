package declaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// CreateVolume creates a new persistent volume with the given name and initial data.
// The server expects a gzip-compressed tar archive as the body. If data is non-nil and
// non-empty, it must begin with the gzip magic bytes (0x1F 0x8B); otherwise the server
// will reject the request.
func CreateVolume(ctx context.Context, name string, data []byte, opts ...SandboxOption) (*VolumeInfo, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	path := "/volumes?name=" + url.QueryEscape(name)

	var body []byte
	if data != nil {
		body = data
	} else {
		body = []byte{}
	}

	if len(body) > 0 && (len(body) < 2 || body[0] != 0x1F || body[1] != 0x8B) {
		return nil, fmt.Errorf("volume data must be a gzip-compressed tar archive (expected gzip magic bytes 0x1F 0x8B)")
	}

	respBody, err := client.postGzip(ctx, path, body)
	if err != nil {
		return nil, err
	}

	return parseVolumeInfo(respBody)
}

// CommitVolume captures the subtree at the attached volume's mount path inside
// the given sandbox and creates a NEW volume from it. The source volume is left
// unchanged. If name is empty, the server names the new volume "<source-name>-commit".
// It returns the VolumeInfo of the newly created volume.
func CommitVolume(ctx context.Context, sandboxID, volumeID, name string, opts ...SandboxOption) (*VolumeInfo, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	path := fmt.Sprintf("/sandboxes/%s/volumes/%s/commit", sandboxID, volumeID)
	if name != "" {
		path += "?name=" + url.QueryEscape(name)
	}

	respBody, err := client.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	return parseVolumeInfo(respBody)
}

// Volume attach modes for VolumeAttachment.Mode.
const (
	// VolumeModeCopy hydrates the volume into the sandbox at boot (default).
	VolumeModeCopy = "copy"
	// VolumeModeMount attaches the volume as a read-write live NFS mount.
	VolumeModeMount = "mount"
	// VolumeModeMountRO attaches the volume as a read-only live mount.
	VolumeModeMountRO = "mount-ro"
)

// SnapshotVolume captures an arbitrary absolute path inside the given sandbox
// into a NEW volume. path must be an absolute in-sandbox path; synthetic paths
// such as /proc, /sys and /dev are rejected by the server. If name is empty the
// server names the volume "snapshot". It returns the VolumeInfo of the newly
// created volume.
func SnapshotVolume(ctx context.Context, sandboxID, path, name string, opts ...SandboxOption) (*VolumeInfo, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	q := url.Values{}
	q.Set("path", path)
	if name != "" {
		q.Set("name", name)
	}
	reqPath := fmt.Sprintf("/sandboxes/%s/volumes/snapshot?%s", sandboxID, q.Encode())

	respBody, err := client.post(ctx, reqPath, nil)
	if err != nil {
		return nil, err
	}
	return parseVolumeInfo(respBody)
}

// CreateEmptyVolume creates a new empty file-granular volume with the given
// name. The server returns 503 if the file-granular backend is not configured.
func CreateEmptyVolume(ctx context.Context, name string, opts ...SandboxOption) (*VolumeInfo, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	path := "/volumes/empty?name=" + url.QueryEscape(name)
	respBody, err := client.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	return parseVolumeInfo(respBody)
}

// IngestVolume creates a new file-granular volume from a gzip-compressed tar
// archive (data must begin with the gzip magic bytes 0x1F 0x8B). The server
// returns 413 if the quota is exceeded, 400 for a bad archive, and 503 if the
// file-granular backend is not configured.
func IngestVolume(ctx context.Context, name string, data []byte, opts ...SandboxOption) (*VolumeInfo, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	var body []byte
	if data != nil {
		body = data
	} else {
		body = []byte{}
	}
	if len(body) > 0 && (len(body) < 2 || body[0] != 0x1F || body[1] != 0x8B) {
		return nil, fmt.Errorf("volume data must be a gzip-compressed tar archive (expected gzip magic bytes 0x1F 0x8B)")
	}

	path := "/volumes/ingest?name=" + url.QueryEscape(name)
	respBody, err := client.postGzip(ctx, path, body)
	if err != nil {
		return nil, err
	}
	return parseVolumeInfo(respBody)
}

// ListVolumes returns all volumes owned by the authenticated user.
func ListVolumes(ctx context.Context, opts ...SandboxOption) ([]VolumeInfo, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	respBody, err := client.get(ctx, "/volumes")
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Volumes []volumeInfoJSON `json:"volumes"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return nil, fmt.Errorf("parsing volumes list: %w", err)
	}

	volumes := make([]VolumeInfo, len(wrapper.Volumes))
	for i, r := range wrapper.Volumes {
		volumes[i] = r.toVolumeInfo()
	}
	return volumes, nil
}

// GetVolume returns information about a specific volume.
func GetVolume(ctx context.Context, volumeID string, opts ...SandboxOption) (*VolumeInfo, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	path := fmt.Sprintf("/volumes/%s", volumeID)
	respBody, err := client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	return parseVolumeInfo(respBody)
}

// DownloadVolume downloads the contents of a volume as raw bytes.
func DownloadVolume(ctx context.Context, volumeID string, opts ...SandboxOption) ([]byte, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	path := fmt.Sprintf("/volumes/%s/download", volumeID)
	return client.get(ctx, path)
}

// DeleteVolume deletes a volume by its ID.
func DeleteVolume(ctx context.Context, volumeID string, opts ...SandboxOption) error {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	path := fmt.Sprintf("/volumes/%s", volumeID)
	_, err := client.delete(ctx, path)
	return err
}

// volumeInfoJSON is the JSON representation of VolumeInfo from the API.
type volumeInfoJSON struct {
	VolumeID    string            `json:"volume_id"`
	OwnerID     string            `json:"owner_id"`
	Name        string            `json:"name"`
	BlobKey     string            `json:"blob_key"`
	SizeBytes   int64             `json:"size_bytes"`
	ContentType string            `json:"content_type"`
	CreatedAt   string            `json:"created_at"`
	Metadata    map[string]string `json:"metadata"`
	Backend     string            `json:"backend"`
	QuotaBytes  int64             `json:"quota_bytes"`
	UpdatedAt   string            `json:"updated_at"`
}

func (r *volumeInfoJSON) toVolumeInfo() VolumeInfo {
	return VolumeInfo{
		VolumeID:    r.VolumeID,
		OwnerID:     r.OwnerID,
		Name:        r.Name,
		BlobKey:     r.BlobKey,
		SizeBytes:   r.SizeBytes,
		ContentType: r.ContentType,
		CreatedAt:   r.CreatedAt,
		Metadata:    r.Metadata,
		Backend:     r.Backend,
		QuotaBytes:  r.QuotaBytes,
		UpdatedAt:   r.UpdatedAt,
	}
}

func parseVolumeInfo(data []byte) (*VolumeInfo, error) {
	var raw volumeInfoJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing volume info: %w", err)
	}
	v := raw.toVolumeInfo()
	return &v, nil
}
