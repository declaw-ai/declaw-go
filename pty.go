package declaw

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// PTY provides pseudo-terminal access to a sandbox.
// Obtain a PTY instance from the Sandbox.PTY field.
type PTY struct {
	sandboxID string
	client    *apiClient
}

// Create creates a new pseudo-terminal inside the sandbox.
// An optional PtySize can be provided to set the initial terminal dimensions.
// If no size is provided, the default 80x24 is used.
func (p *PTY) Create(ctx context.Context, size ...PtySize) (*PtyHandle, error) {
	cols := 80
	rows := 24
	if len(size) > 0 {
		cols = size[0].Cols
		rows = size[0].Rows
	}

	body := map[string]interface{}{
		"size": map[string]interface{}{
			"cols": cols,
			"rows": rows,
		},
		"user":    "user",
		"timeout": 3600,
	}

	path := fmt.Sprintf("/sandboxes/%s/pty", p.sandboxID)
	respBody, err := p.client.post(ctx, path, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		PID int `json:"pid"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing pty create response: %w", err)
	}

	return &PtyHandle{
		PID:       result.PID,
		sandboxID: p.sandboxID,
		client:    p.client,
	}, nil
}

// PtyHandle represents an active pseudo-terminal session inside a sandbox.
type PtyHandle struct {
	// PID is the process ID of the shell process backing the PTY.
	PID int

	sandboxID string
	client    *apiClient
}

// Kill terminates the PTY session.
func (h *PtyHandle) Kill(ctx context.Context) error {
	path := fmt.Sprintf("/sandboxes/%s/pty/%d", h.sandboxID, h.PID)
	_, err := h.client.delete(ctx, path)
	return err
}

// SendInput sends raw input data to the PTY.
func (h *PtyHandle) SendInput(ctx context.Context, data []byte) error {
	body := map[string]interface{}{
		"data": string(data),
	}

	path := fmt.Sprintf("/sandboxes/%s/pty/%d/stdin", h.sandboxID, h.PID)
	_, err := h.client.post(ctx, path, body)
	return err
}

// SetSize resizes the PTY to the given dimensions.
func (h *PtyHandle) SetSize(ctx context.Context, cols, rows int) error {
	body := map[string]interface{}{
		"size": map[string]interface{}{
			"cols": cols,
			"rows": rows,
		},
	}

	path := fmt.Sprintf("/sandboxes/%s/pty/%d", h.sandboxID, h.PID)
	_, err := h.client.patch(ctx, path, body)
	return err
}

// Stream returns a read-only channel that receives output data from the PTY.
// The channel is closed when the PTY session ends or the context is canceled.
func (h *PtyHandle) Stream(ctx context.Context) (<-chan []byte, error) {
	path := fmt.Sprintf("/sandboxes/%s/pty/%d/stream", h.sandboxID, h.PID)
	resp, err := h.client.stream(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, errorFromResponse(resp, body, h.sandboxID)
	}

	ch := make(chan []byte)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		var eventType string

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimPrefix(line, "event: ")
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				dataStr := strings.TrimPrefix(line, "data: ")

				if eventType == "exit" {
					return
				}

				if eventType == "data" {
					var payload struct {
						Data string `json:"data"`
					}
					if err := json.Unmarshal([]byte(dataStr), &payload); err != nil {
						continue
					}

					decoded, err := base64.StdEncoding.DecodeString(payload.Data)
					if err != nil {
						continue
					}

					select {
					case ch <- decoded:
					case <-ctx.Done():
						return
					}
				}
				continue
			}

			// Empty line or other content, reset event type
			if line == "" {
				eventType = ""
			}
		}
	}()

	return ch, nil
}
