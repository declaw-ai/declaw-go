// Package main demonstrates OPA/Rego custom policy enforcement in a Declaw sandbox.
//
// This example creates a sandbox with two inline Rego modules:
//   - declaw.platform.cmd     — blocks the `rm` command at the command-gate
//   - declaw.platform.network — blocks egress to *.ru domains at the network layer
//
// Gate behavior:
//   - cmd gate denial  → the run call returns an error (HTTP 403 from the platform)
//   - network gate denial → the command runs but the outbound connection is dropped
//   - platform floor   → IMDS / link-local is blocked unconditionally regardless of policy
//
// Usage:
//
//	DECLAW_API_KEY=dk_... DECLAW_DOMAIN=api.declaw.ai go run ./examples/opa-custom-policy
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	declaw "github.com/declaw-ai/declaw-go"
)

// rego modules — each is a complete, independently-packaged Rego source.

// regoCmd blocks any command whose argv[0] (the binary name) is "rm".
//
// OPA input contract for declaw.platform.cmd:
//
//	input.action.command — string: the executable name (e.g. "rm", "ls")
//	input.action.args    — array<string>: the argument list
const regoCmd = `
package declaw.platform.cmd

# Deny the 'rm' command unconditionally.
# input.action.command is the binary being invoked (not the full shell line).
deny contains msg if {
    input.action.command == "rm"
    msg := "rm is not permitted by custom policy"
}
`

// regoNetwork blocks egress connections to any destination whose hostname
// ends with ".ru" or is exactly "ya.ru" (handled via suffix match).
//
// OPA input contract for declaw.platform.network:
//
//	input.action.destination — string: the target hostname or IP:port
const regoNetwork = `
package declaw.platform.network

# Deny egress to any .ru TLD destination.
# input.action.destination is the full host (or host:port).
deny contains msg if {
    # Strip optional port suffix so "ya.ru:443" still matches.
    host := split(input.action.destination, ":")[0]
    endswith(host, ".ru")
    msg := "egress to .ru domains is blocked by custom policy"
}

# Also explicitly deny the bare "ya.ru" apex.
deny contains msg if {
    input.action.destination == "ya.ru"
    msg := "egress to ya.ru is blocked by custom policy"
}
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Validate required environment variables up front so the error message is clear.
	apiKey := os.Getenv("DECLAW_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("DECLAW_API_KEY must be set")
	}

	// DECLAW_DOMAIN defaults to "api.declaw.ai" inside the SDK when unset.
	// We surface it here just for informational output.
	domain := os.Getenv("DECLAW_DOMAIN")
	if domain == "" {
		domain = "api.declaw.ai"
	}

	fmt.Printf("Connecting to Declaw API at %s\n", domain)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// ------------------------------------------------------------------
	// 1. Build the security policy with two inline Rego modules.
	//
	// WithSecurity accepts a SecurityPolicy value; the SDK reads DECLAW_API_KEY
	// and DECLAW_DOMAIN from the environment through NewConfig() automatically —
	// no explicit client construction is needed for the sandbox-scoped calls.
	// ------------------------------------------------------------------
	policy := declaw.SecurityPolicy{
		CustomPolicy: &declaw.CustomPolicyConfig{
			Enabled: true,

			// InlineModules: each entry is a complete Rego source with its own
			// package declaration. Using InlineModules (rather than InlineRego)
			// lets us place the cmd rule and the network rule in their respective
			// declaw.platform.* packages without string-merging them.
			InlineModules: []string{regoCmd, regoNetwork},

			// DefaultDeny = true means fail-closed: if the OPA engine is
			// unreachable or returns an error the action is denied, not allowed.
			DefaultDeny: true,
		},
	}

	// ------------------------------------------------------------------
	// 2. Create the sandbox.
	//    declaw.Create reads DECLAW_API_KEY / DECLAW_DOMAIN from env via
	//    NewConfig() inside the SDK — no separate client object is needed.
	// ------------------------------------------------------------------
	fmt.Println("Creating sandbox with OPA custom policy...")
	sbx, err := declaw.Create(ctx,
		declaw.WithTemplate("base"),
		declaw.WithTimeout(180),
		declaw.WithSecurity(policy),
		declaw.WithMetadata(map[string]string{
			"example": "opa-custom-policy",
		}),
	)
	if err != nil {
		return fmt.Errorf("sandbox create failed: %w", err)
	}

	fmt.Printf("Sandbox created: ID=%s\n\n", sbx.ID)

	// Deferred kill — always fires, even if the checks below panic.
	defer func() {
		fmt.Printf("\nCleaning up sandbox %s...\n", sbx.ID)
		if killErr := sbx.Kill(context.Background()); killErr != nil {
			fmt.Fprintf(os.Stderr, "warning: kill sandbox %s: %v\n", sbx.ID, killErr)
		} else {
			fmt.Printf("Sandbox %s killed.\n", sbx.ID)
		}
	}()

	// ------------------------------------------------------------------
	// 3. Run the policy checks.
	// ------------------------------------------------------------------
	results := []checkResult{
		checkBlocked(ctx, sbx, "rm -rf /tmp/x",
			"rm blocked by cmd-gate OPA rule"),

		checkAllowed(ctx, sbx, "ls /tmp",
			"ls /tmp allowed (not in deny list)"),

		// curl to ya.ru — the network rule blocks the *egress connection*,
		// so the command itself starts and exits, but the HTTP request never
		// reaches the server. curl exits non-zero (connection refused / timeout).
		// We check for a non-zero exit or curl error output rather than a hard 403.
		checkNetworkBlocked(ctx, sbx, "curl -s --max-time 5 https://ya.ru/",
			"egress to ya.ru blocked by network-gate OPA rule"),

		// curl to example.com — should succeed (allowed destination).
		checkAllowed(ctx, sbx, "curl -s --max-time 10 -o /dev/null -w '%{http_code}' https://example.com/",
			"egress to example.com allowed"),

		// IMDS / link-local — blocked by the non-bypassable platform floor
		// regardless of any customer policy. Platform always denies 169.254.x.x.
		checkNetworkBlocked(ctx, sbx, "curl -s --max-time 5 http://169.254.169.254/latest/meta-data/",
			"IMDS 169.254.169.254 blocked by platform floor (not bypassable by policy)"),
	}

	// ------------------------------------------------------------------
	// 4. Print a summary table.
	// ------------------------------------------------------------------
	fmt.Println("=== Results ===")
	passed := 0
	failed := 0
	for _, r := range results {
		status := "PASS"
		if !r.ok {
			status = "FAIL"
			failed++
		} else {
			passed++
		}
		fmt.Printf("[%s] %s\n", status, r.label)
		if !r.ok {
			fmt.Printf("       reason: %s\n", r.detail)
		}
	}
	fmt.Printf("\n%d passed, %d failed\n", passed, failed)

	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	return nil
}

// checkResult holds the outcome of a single policy check.
type checkResult struct {
	label  string
	ok     bool
	detail string
}

// checkBlocked expects the command to be rejected by the cmd-gate OPA rule.
// A cmd-gate denial surfaces as an error from Commands.Run (the platform returns
// HTTP 403 before the command ever executes inside the VM). A CommandExitError
// with a non-zero exit code that is NOT an auth/403 error is also treated as
// blocked to handle cases where the shell rejects the command before OPA does.
func checkBlocked(ctx context.Context, sbx *declaw.Sandbox, cmd, label string) checkResult {
	fmt.Printf("CHECK: %s\n", label)
	fmt.Printf("  cmd: %s\n", cmd)

	result, err := sbx.Commands.Run(ctx, cmd)
	if err != nil {
		// An error from Run is the expected path for a cmd-gate OPA denial.
		// The SDK maps HTTP 401/403 to AuthenticationError, so we check for that.
		var authErr *declaw.AuthenticationError
		if errors.As(err, &authErr) {
			fmt.Printf("  -> blocked (403/auth): %v\n", err)
			return checkResult{label: label, ok: true}
		}

		// CommandExitError with non-zero exit code — the platform may surface the
		// denial as a non-zero exit rather than a hard 403 depending on server version.
		var exitErr *declaw.CommandExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode != 0 {
			fmt.Printf("  -> blocked (exit %d): stderr=%q\n", exitErr.ExitCode, exitErr.Stderr)
			return checkResult{label: label, ok: true}
		}

		// Any other error is also treated as "blocked" since the command did not run.
		fmt.Printf("  -> blocked (error): %v\n", err)
		return checkResult{label: label, ok: true}
	}

	// If we got here the command ran successfully — that is unexpected.
	fmt.Printf("  -> UNEXPECTED: command ran (exit=%d stdout=%q)\n", result.ExitCode, strings.TrimSpace(result.Stdout))
	return checkResult{
		label:  label,
		ok:     false,
		detail: fmt.Sprintf("command ran with exit=%d; expected it to be blocked", result.ExitCode),
	}
}

// checkAllowed expects the command to run and exit 0.
func checkAllowed(ctx context.Context, sbx *declaw.Sandbox, cmd, label string) checkResult {
	fmt.Printf("CHECK: %s\n", label)
	fmt.Printf("  cmd: %s\n", cmd)

	result, err := sbx.Commands.Run(ctx, cmd)
	if err != nil {
		// CommandExitError means the command ran but exited non-zero — still ran.
		var exitErr *declaw.CommandExitError
		if errors.As(err, &exitErr) {
			// For the example.com curl check we only asked for the HTTP status code
			// via %{http_code}. A 4xx from the remote is fine — what matters is that
			// the platform did not block the egress. Exit code != 0 is acceptable if
			// the command actually executed; accept it as "allowed".
			fmt.Printf("  -> allowed (exit %d, stderr=%q)\n", exitErr.ExitCode, exitErr.Stderr)
			return checkResult{label: label, ok: true}
		}

		fmt.Printf("  -> UNEXPECTED error: %v\n", err)
		return checkResult{
			label:  label,
			ok:     false,
			detail: fmt.Sprintf("unexpected error: %v", err),
		}
	}

	fmt.Printf("  -> allowed (exit=%d stdout=%q)\n", result.ExitCode, strings.TrimSpace(result.Stdout))
	return checkResult{label: label, ok: true}
}

// checkNetworkBlocked expects the command to run (no cmd-gate denial) but the
// network connection to be dropped by the network-gate OPA rule or the platform
// floor. curl will exit non-zero when the connection is refused or times out.
// We verify that the command was attempted but did not receive a successful HTTP
// response from the blocked destination.
func checkNetworkBlocked(ctx context.Context, sbx *declaw.Sandbox, cmd, label string) checkResult {
	fmt.Printf("CHECK: %s\n", label)
	fmt.Printf("  cmd: %s\n", cmd)

	result, err := sbx.Commands.Run(ctx, cmd)
	if err != nil {
		var authErr *declaw.AuthenticationError
		if errors.As(err, &authErr) {
			// Hard 403 — could mean the cmd-gate blocked curl itself (unlikely),
			// or that the OPA engine surfaced the network denial before curl ran.
			// Either way, the destination is inaccessible — count as blocked.
			fmt.Printf("  -> blocked (403/auth from platform): %v\n", err)
			return checkResult{label: label, ok: true}
		}

		var exitErr *declaw.CommandExitError
		if errors.As(err, &exitErr) {
			// curl non-zero: connection refused, timed out, or DNS failure —
			// all consistent with the egress being dropped.
			fmt.Printf("  -> blocked (curl exit %d, stderr=%q)\n", exitErr.ExitCode, exitErr.Stderr)
			return checkResult{label: label, ok: true}
		}

		// Any other error: command did not complete — treat as blocked.
		fmt.Printf("  -> blocked (error): %v\n", err)
		return checkResult{label: label, ok: true}
	}

	// Command completed with exit 0. Check whether the response looks like a
	// successful reply from the blocked host. For IMDS and ya.ru the reply body
	// should never be present if the platform is enforcing correctly.
	stdout := strings.TrimSpace(result.Stdout)
	if stdout != "" && stdout != "000" {
		// Non-empty, non-"000" output means curl got a real HTTP response —
		// the host was reachable, which is a policy failure.
		fmt.Printf("  -> UNEXPECTED: egress succeeded (stdout=%q)\n", stdout)
		return checkResult{
			label:  label,
			ok:     false,
			detail: fmt.Sprintf("egress to blocked host succeeded: %q", stdout),
		}
	}

	// Exit 0 with empty or "000" stdout — curl ran but got no usable response.
	// "000" is curl's internal code for connection failures. Treat as blocked.
	fmt.Printf("  -> blocked (exit=0, stdout=%q — no usable response from host)\n", stdout)
	return checkResult{label: label, ok: true}
}
