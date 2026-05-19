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

	respBody, err := client.postRaw(ctx, path, body)
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
