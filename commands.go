package declaw

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Commands provides command execution inside a sandbox.
// Obtain a Commands instance from the Sandbox.Commands field.
type Commands struct {
	sandboxID string
	client    *apiClient
}

// commandRequest is the JSON body sent to POST /sandboxes/{id}/commands.
type commandRequest struct {
	Cmd        string            `json:"cmd"`
	Background bool              `json:"background"`
	Stdin      bool              `json:"stdin,omitempty"`
	User       string            `json:"user,omitempty"`
	Cwd        string            `json:"cwd,omitempty"`
	Envs       map[string]string `json:"envs,omitempty"`
	Timeout    float64           `json:"timeout,omitempty"`
}

// commandResponse is the JSON response from POST /sandboxes/{id}/commands.
type commandResponse struct {
	PID      int    `json:"pid"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// resolveRunOpts applies all RunOption functions and returns the resolved options.
func resolveRunOpts(opts []RunOption) *runOpts {
	o := &runOpts{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// buildCommandRequest creates a commandRequest from the command string, options, and background flag.
func buildCommandRequest(cmd string, opts *runOpts, background bool) *commandRequest {
	req := &commandRequest{
		Cmd:        cmd,
		Background: background,
	}
	if opts.User != "" {
		req.User = opts.User
	}
	if opts.Cwd != "" {
		req.Cwd = opts.Cwd
	}
	if len(opts.Envs) > 0 {
		req.Envs = opts.Envs
	}
	if opts.Stdin {
		req.Stdin = true
	}
	if opts.Timeout > 0 {
		req.Timeout = opts.Timeout.Seconds()
	}
	return req
}

// invokeCallbacks splits stdout/stderr by newline and calls OnStdout/OnStderr for each non-empty line.
func invokeCallbacks(result *CommandResult, opts *runOpts) {
	if opts.OnStdout != nil && result.Stdout != "" {
		lines := strings.Split(result.Stdout, "\n")
		for _, line := range lines {
			if line != "" {
				opts.OnStdout(line)
			}
		}
	}
	if opts.OnStderr != nil && result.Stderr != "" {
		lines := strings.Split(result.Stderr, "\n")
		for _, line := range lines {
			if line != "" {
				opts.OnStderr(line)
			}
		}
	}
}

// Run executes a command inside the sandbox and waits for it to complete.
// Returns the command result including stdout, stderr, and exit code.
func (c *Commands) Run(ctx context.Context, cmd string, opts ...RunOption) (*CommandResult, error) {
	o := resolveRunOpts(opts)
	req := buildCommandRequest(cmd, o, false)

	path := fmt.Sprintf("/sandboxes/%s/commands", c.sandboxID)
	data, err := c.client.post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var resp commandResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decoding command response: %w", err)
	}

	result := &CommandResult{
		PID:      resp.PID,
		ExitCode: resp.ExitCode,
		Stdout:   resp.Stdout,
		Stderr:   resp.Stderr,
	}

	invokeCallbacks(result, o)

	if result.ExitCode != 0 {
		return result, &CommandExitError{
			SandboxError: &SandboxError{
				Message:    fmt.Sprintf("command exited with code %d", result.ExitCode),
				SandboxID:  c.sandboxID,
				StatusCode: 0,
			},
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
		}
	}

	return result, nil
}

// Start starts a command inside the sandbox without waiting for it to complete.
// Returns a CommandHandle that can be used to wait for completion, send stdin, or kill the process.
func (c *Commands) Start(ctx context.Context, cmd string, opts ...RunOption) (*CommandHandle, error) {
	o := resolveRunOpts(opts)
	req := buildCommandRequest(cmd, o, true)

	path := fmt.Sprintf("/sandboxes/%s/commands", c.sandboxID)
	data, err := c.client.post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var resp struct {
		PID int `json:"pid"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decoding start response: %w", err)
	}

	return &CommandHandle{
		PID:       resp.PID,
		sandboxID: c.sandboxID,
		client:    c.client,
	}, nil
}

// List returns information about all running processes inside the sandbox.
func (c *Commands) List(ctx context.Context) ([]ProcessInfo, error) {
	path := fmt.Sprintf("/sandboxes/%s/commands", c.sandboxID)
	data, err := c.client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var procs []ProcessInfo
	if err := json.Unmarshal(data, &procs); err != nil {
		return nil, fmt.Errorf("decoding process list: %w", err)
	}

	return procs, nil
}

// CommandHandle represents a running command inside a sandbox.
// It is returned by Commands.Start and provides methods to interact
// with the running process.
type CommandHandle struct {
	// PID is the process ID of the running command.
	PID int

	sandboxID string
	client    *apiClient
}

// Wait blocks until the command completes and returns its result.
func (h *CommandHandle) Wait(ctx context.Context) (*CommandResult, error) {
	path := fmt.Sprintf("/sandboxes/%s/commands/%d/wait", h.sandboxID, h.PID)
	data, err := h.client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var resp commandResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decoding wait response: %w", err)
	}

	result := &CommandResult{
		PID:      resp.PID,
		ExitCode: resp.ExitCode,
		Stdout:   resp.Stdout,
		Stderr:   resp.Stderr,
	}

	if result.ExitCode != 0 {
		return result, &CommandExitError{
			SandboxError: &SandboxError{
				Message:    fmt.Sprintf("command exited with code %d", result.ExitCode),
				SandboxID:  h.sandboxID,
				StatusCode: 0,
			},
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
		}
	}

	return result, nil
}

// Kill terminates the running command.
func (h *CommandHandle) Kill(ctx context.Context) error {
	path := fmt.Sprintf("/sandboxes/%s/commands/%d", h.sandboxID, h.PID)
	_, err := h.client.delete(ctx, path)
	return err
}

// SendStdin sends data to the command's standard input.
// The command must have been started with WithStdin().
//
// NOTE: Not yet implemented. Returns an error until server-side support is available.
func (h *CommandHandle) SendStdin(ctx context.Context, data string) error {
	return fmt.Errorf("command stdin is not yet implemented")
}
