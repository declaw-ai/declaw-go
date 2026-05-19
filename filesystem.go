package declaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"
)

// Filesystem provides file operations inside a sandbox.
// Obtain a Filesystem instance from the Sandbox.Files field.
type Filesystem struct {
	sandboxID string
	client    *apiClient
}

// resolveFileOpts applies all FileOption functions and returns the resolved options.
func resolveFileOpts(opts []FileOption) *fileOpts {
	o := &fileOpts{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// buildFileQuery constructs a URL path with query parameters for file operations.
func buildFileQuery(basePath, filePath string, fo *fileOpts) string {
	params := url.Values{}
	params.Set("path", filePath)
	if fo.User != "" {
		params.Set("username", fo.User)
	}
	return basePath + "?" + params.Encode()
}

// Read reads a file as a UTF-8 string from the sandbox filesystem.
func (f *Filesystem) Read(ctx context.Context, path string, opts ...FileOption) (string, error) {
	fo := resolveFileOpts(opts)
	endpoint := fmt.Sprintf("/sandboxes/%s/files", f.sandboxID)
	reqPath := buildFileQuery(endpoint, path, fo)

	data, err := f.client.get(ctx, reqPath)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// ReadBytes reads a file as raw bytes from the sandbox filesystem.
func (f *Filesystem) ReadBytes(ctx context.Context, path string, opts ...FileOption) ([]byte, error) {
	fo := resolveFileOpts(opts)
	endpoint := fmt.Sprintf("/sandboxes/%s/files/raw", f.sandboxID)
	reqPath := buildFileQuery(endpoint, path, fo)

	data, err := f.client.get(ctx, reqPath)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// Write writes a UTF-8 string to a file in the sandbox filesystem.
func (f *Filesystem) Write(ctx context.Context, path string, data string, opts ...FileOption) (*WriteInfo, error) {
	fo := resolveFileOpts(opts)
	endpoint := fmt.Sprintf("/sandboxes/%s/files", f.sandboxID)

	body := map[string]string{
		"path": path,
		"data": data,
	}
	if fo.User != "" {
		body["username"] = fo.User
	}

	respData, err := f.client.post(ctx, endpoint, body)
	if err != nil {
		return nil, err
	}

	var info WriteInfo
	if err := json.Unmarshal(respData, &info); err != nil {
		return nil, fmt.Errorf("decoding write info: %w", err)
	}

	return &info, nil
}

// WriteBytes writes raw bytes to a file in the sandbox filesystem.
func (f *Filesystem) WriteBytes(ctx context.Context, path string, data []byte, opts ...FileOption) (*WriteInfo, error) {
	fo := resolveFileOpts(opts)
	endpoint := fmt.Sprintf("/sandboxes/%s/files/raw", f.sandboxID)
	reqPath := buildFileQuery(endpoint, path, fo)

	respData, err := f.client.put(ctx, reqPath, data)
	if err != nil {
		return nil, err
	}

	var info WriteInfo
	if err := json.Unmarshal(respData, &info); err != nil {
		return nil, fmt.Errorf("decoding write info: %w", err)
	}

	return &info, nil
}

// batchFileEntry is a single file in a batch write request.
type batchFileEntry struct {
	Path     string      `json:"path"`
	Data     interface{} `json:"data"`
	Username string      `json:"username,omitempty"`
}

// WriteFiles writes multiple files to the sandbox filesystem in a single operation.
// Each entry's Data must be a string. Use WriteBytes for binary data.
func (f *Filesystem) WriteFiles(ctx context.Context, entries []WriteEntry, opts ...FileOption) error {
	if len(entries) == 0 {
		return nil
	}

	fo := resolveFileOpts(opts)
	endpoint := fmt.Sprintf("/sandboxes/%s/files/batch", f.sandboxID)

	files := make([]batchFileEntry, len(entries))
	for i, e := range entries {
		switch e.Data.(type) {
		case string:
		default:
			return fmt.Errorf("WriteFiles entry %d (%s): Data must be a string, got %T; use WriteBytes for binary data", i, e.Path, e.Data)
		}
		files[i] = batchFileEntry{
			Path:     e.Path,
			Data:     e.Data,
			Username: fo.User,
		}
	}

	body := map[string]interface{}{
		"files": files,
	}

	_, err := f.client.post(ctx, endpoint, body)
	return err
}

// List returns the contents of a directory in the sandbox filesystem.
func (f *Filesystem) List(ctx context.Context, path string, opts ...FileOption) ([]EntryInfo, error) {
	fo := resolveFileOpts(opts)
	endpoint := fmt.Sprintf("/sandboxes/%s/files/list", f.sandboxID)
	reqPath := buildFileQuery(endpoint, path, fo)

	data, err := f.client.get(ctx, reqPath)
	if err != nil {
		return nil, err
	}

	var entries []EntryInfo
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("decoding directory listing: %w", err)
	}

	return entries, nil
}

// Exists returns true if the given path exists in the sandbox filesystem.
func (f *Filesystem) Exists(ctx context.Context, path string, opts ...FileOption) (bool, error) {
	fo := resolveFileOpts(opts)
	endpoint := fmt.Sprintf("/sandboxes/%s/files/exists", f.sandboxID)
	reqPath := buildFileQuery(endpoint, path, fo)

	data, err := f.client.get(ctx, reqPath)
	if err != nil {
		return false, err
	}

	var resp struct {
		Exists bool `json:"exists"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return false, fmt.Errorf("decoding exists response: %w", err)
	}

	return resp.Exists, nil
}

// GetInfo returns metadata about a file or directory in the sandbox filesystem.
func (f *Filesystem) GetInfo(ctx context.Context, path string, opts ...FileOption) (*EntryInfo, error) {
	fo := resolveFileOpts(opts)
	endpoint := fmt.Sprintf("/sandboxes/%s/files/info", f.sandboxID)
	reqPath := buildFileQuery(endpoint, path, fo)

	data, err := f.client.get(ctx, reqPath)
	if err != nil {
		return nil, err
	}

	var info EntryInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("decoding entry info: %w", err)
	}

	return &info, nil
}

// Remove deletes a file or directory from the sandbox filesystem.
func (f *Filesystem) Remove(ctx context.Context, path string, opts ...FileOption) error {
	fo := resolveFileOpts(opts)
	endpoint := fmt.Sprintf("/sandboxes/%s/files", f.sandboxID)
	reqPath := buildFileQuery(endpoint, path, fo)

	_, err := f.client.delete(ctx, reqPath)
	return err
}

// Rename moves or renames a file or directory in the sandbox filesystem.
func (f *Filesystem) Rename(ctx context.Context, oldPath, newPath string, opts ...FileOption) error {
	fo := resolveFileOpts(opts)
	endpoint := fmt.Sprintf("/sandboxes/%s/files", f.sandboxID)

	body := map[string]string{
		"old_path": oldPath,
		"new_path": newPath,
	}
	if fo.User != "" {
		body["username"] = fo.User
	}

	_, err := f.client.patch(ctx, endpoint, body)
	return err
}

// MakeDir creates a directory (and any necessary parents) in the sandbox filesystem.
func (f *Filesystem) MakeDir(ctx context.Context, path string, opts ...FileOption) error {
	fo := resolveFileOpts(opts)
	endpoint := fmt.Sprintf("/sandboxes/%s/files/mkdir", f.sandboxID)

	body := map[string]string{
		"path": path,
	}
	if fo.User != "" {
		body["username"] = fo.User
	}

	_, err := f.client.post(ctx, endpoint, body)
	return err
}

// Watch starts watching a path for filesystem changes. Returns a WatchHandle
// that provides a channel of filesystem events.
//
// NOTE: Not yet implemented. Returns an error until server-side streaming support is available.
func (f *Filesystem) Watch(ctx context.Context, path string, opts ...FileOption) (*WatchHandle, error) {
	return nil, fmt.Errorf("filesystem watch is not yet implemented")
}

// WriteEntry describes a file to write in a batch WriteFiles operation.
// Data must be a string. Use WriteBytes for binary data.
type WriteEntry struct {
	Path string
	Data interface{}
}

// WatchHandle represents an active filesystem watch.
// Call Stop to end the watch and close the events channel.
type WatchHandle struct {
	events  chan FilesystemEvent
	done    chan struct{}
	stopMu  sync.Once
}

// Events returns a read-only channel that receives filesystem events.
func (w *WatchHandle) Events() <-chan FilesystemEvent {
	return w.events
}

// Stop stops watching for filesystem events and closes the events channel.
func (w *WatchHandle) Stop() {
	w.stopMu.Do(func() {
		close(w.done)
		close(w.events)
	})
}

// FilesystemEvent represents a change to a file or directory.
type FilesystemEvent struct {
	Type      FilesystemEventType
	Path      string
	Timestamp time.Time
}

// FilesystemEventType identifies the kind of filesystem change.
type FilesystemEventType string

const (
	// EventCreated indicates a file or directory was created.
	EventCreated FilesystemEventType = "created"

	// EventModified indicates a file or directory was modified.
	EventModified FilesystemEventType = "modified"

	// EventDeleted indicates a file or directory was deleted.
	EventDeleted FilesystemEventType = "deleted"
)
