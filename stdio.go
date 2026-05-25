package declaw

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// StdioSession provides interactive stdio access to the sandbox.
// Obtain a StdioSession instance from the Sandbox.Stdio field.
type StdioSession struct {
	sandboxID string
	client    *apiClient
}

// stdioCreateRequest is the JSON body sent to POST /sandboxes/{id}/stdio.
type stdioCreateRequest struct {
	Cmd  string            `json:"cmd"`
	User string            `json:"user,omitempty"`
	Cwd  string            `json:"cwd,omitempty"`
	Envs map[string]string `json:"envs,omitempty"`
}

// StdioStartOpts configures a stdio process start.
type StdioStartOpts struct {
	User string
	Cwd  string
	Envs map[string]string
}

// StdioResult holds the exit code of a completed stdio process.
type StdioResult struct {
	ExitCode int
}

// StdioStreamOpts configures output streaming callbacks.
type StdioStreamOpts struct {
	OnStdout func(data []byte)
	OnStderr func(data []byte)
}

// StdioHandle is a handle to a running interactive subprocess with stdin pipe.
type StdioHandle struct {
	CmdID     string
	sandboxID string
	client    *apiClient
}

// Start launches an interactive subprocess with an open stdin pipe.
func (s *StdioSession) Start(ctx context.Context, cmd string, opts *StdioStartOpts) (*StdioHandle, error) {
	req := stdioCreateRequest{Cmd: cmd}
	if opts != nil {
		req.User = opts.User
		req.Cwd = opts.Cwd
		req.Envs = opts.Envs
	}
	if req.User == "" {
		req.User = "user"
	}

	path := fmt.Sprintf("/sandboxes/%s/stdio", s.sandboxID)
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.post(ctx, path, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		CmdID string `json:"cmd_id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &StdioHandle{
		CmdID:     result.CmdID,
		sandboxID: s.sandboxID,
		client:    s.client,
	}, nil
}

// SendStdin sends data to the process's stdin.
func (h *StdioHandle) SendStdin(ctx context.Context, data []byte) error {
	encoded := base64.StdEncoding.EncodeToString(data)
	body, err := json.Marshal(map[string]string{"data": encoded})
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/sandboxes/%s/stdio/%s/stdin", h.sandboxID, h.CmdID)
	_, err = h.client.post(ctx, path, body)
	return err
}

// CloseStdin closes the process's stdin, sending EOF.
func (h *StdioHandle) CloseStdin(ctx context.Context) error {
	path := fmt.Sprintf("/sandboxes/%s/stdio/%s/stdin/close", h.sandboxID, h.CmdID)
	_, err := h.client.post(ctx, path, nil)
	return err
}

// Kill terminates the process.
func (h *StdioHandle) Kill(ctx context.Context) error {
	path := fmt.Sprintf("/sandboxes/%s/stdio/%s", h.sandboxID, h.CmdID)
	_, err := h.client.delete(ctx, path)
	return err
}

// Stream connects to the SSE output stream and delivers stdout/stderr
// chunks via callbacks. Blocks until the process exits or the context
// is cancelled. Returns the exit code.
func (h *StdioHandle) Stream(ctx context.Context, opts *StdioStreamOpts) (*StdioResult, error) {
	path := fmt.Sprintf("/sandboxes/%s/stdio/%s/stream", h.sandboxID, h.CmdID)
	resp, err := h.client.stream(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var event string
	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		line := scanner.Text()
		if line == "" {
			event = ""
			continue
		}
		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(line[6:])
			continue
		}
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(line[5:])
			if event == "exit" {
				var exitData struct {
					ExitCode int `json:"exit_code"`
				}
				exitData.ExitCode = -1
				_ = json.Unmarshal([]byte(payload), &exitData)
				return &StdioResult{ExitCode: exitData.ExitCode}, nil
			}
			if opts != nil && (event == "stdout" || event == "stderr") {
				var chunk struct {
					Data string `json:"data"`
				}
				if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
					continue
				}
				decoded, err := base64.StdEncoding.DecodeString(chunk.Data)
				if err != nil {
					continue
				}
				if event == "stdout" && opts.OnStdout != nil {
					opts.OnStdout(decoded)
				} else if event == "stderr" && opts.OnStderr != nil {
					opts.OnStderr(decoded)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &StdioResult{ExitCode: -1}, nil
}

// Wait blocks until the process exits and returns the result.
func (h *StdioHandle) Wait(ctx context.Context) (*StdioResult, error) {
	return h.Stream(ctx, nil)
}
