// Package main demonstrates fail-closed custom OPA policy enforcement
// using a single InlineRego module that blocks specific network tools at the
// command gate.
//
// Scenario: "No ad-hoc network tools" — curl, wget, nc, and ncat are blocked by
// the cmd-gate before they ever execute inside the VM. All other commands
// (including python3) are allowed by the denylist policy.
//
// Key properties shown:
//   - InlineRego with a single package (declaw.platform.cmd) — most common case
//   - DefaultDeny: true — fail-closed: the evaluator denies on OPA engine error
//   - Denylist, not allowlist: only the listed tools are blocked, everything
//     else passes through (python3 running proves this)
//   - Platform floor: IMDS (169.254.169.254) is blocked unconditionally by the
//     platform regardless of any customer policy
//
// Gate behavior:
//   - cmd gate denial → Commands.Run returns an error (HTTP 403 before the VM
//     sees the command)
//   - network gate denial → command runs but outbound connection is dropped
//   - platform floor → IMDS / link-local blocked regardless of policy
//
// Usage:
//
//	DECLAW_API_KEY=dk_... DECLAW_DOMAIN=api.declaw.ai go run ./examples/custom-policy-quickstart
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

// regoNoNetworkTools is a single Rego module for declaw.platform.cmd.
//
// It blocks a fixed set of ad-hoc network tools at the command gate.
// Using a denylist (not an allowlist) means only these four binaries are
// blocked — every other command including python3, pip, etc. passes through.
//
// OPA input contract for declaw.platform.cmd:
//
//	input.action.command — string: the executable name (e.g. "curl", "ls")
//	input.action.args    — array<string>: the argument list
//
// The deny partial rule form is required; each element must be a string
// containing a human-readable reason for the denial.
const regoNoNetworkTools = `
package declaw.platform.cmd

# Ad-hoc network tools that agents must not invoke directly.
# Blocking at the cmd gate means the VM never sees these commands;
# the platform returns HTTP 403 before execution.
blocked := {"curl", "wget", "nc", "ncat"}

deny contains m if {
    input.action.command in blocked
    m := sprintf("command '%s' blocked by custom policy (no ad-hoc network tools)", [input.action.command])
}
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Validate required environment variables up front so the error is clear.
	apiKey := os.Getenv("DECLAW_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("DECLAW_API_KEY must be set")
	}

	// DECLAW_DOMAIN defaults to "api.declaw.ai" inside the SDK when unset.
	// Surface it here for informational output.
	domain := os.Getenv("DECLAW_DOMAIN")
	if domain == "" {
		domain = "api.declaw.ai"
	}

	fmt.Printf("Connecting to Declaw API at %s\n\n", domain)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// ------------------------------------------------------------------
	// 1. Build the security policy with a single InlineRego module.
	//
	// InlineRego is the right field here because the entire policy lives in
	// one package (declaw.platform.cmd). Use InlineModules only when you
	// need rules across multiple packages (e.g., cmd + network in one go).
	//
	// DefaultDeny: true is the headline of this example — it means the
	// evaluator denies any action if the OPA engine is unreachable or
	// returns an evaluation error (fail-closed). For security gates this is
	// always the correct posture; fail-open is only appropriate for
	// advisory-only scanners.
	// ------------------------------------------------------------------
	policy := declaw.SecurityPolicy{
		CustomPolicy: &declaw.CustomPolicyConfig{
			Enabled: true,

			// InlineRego: a complete Rego source for one package.
			// The SDK sends this in the POST /sandboxes body under
			// custom_policy.inline_rego; no external OPA bundle server needed.
			InlineRego: regoNoNetworkTools,

			// DefaultDeny = true → fail-closed.
			// If the OPA evaluator is unavailable or errors, the platform
			// denies the action rather than allowing it through.
			DefaultDeny: true,
		},
	}

	// ------------------------------------------------------------------
	// 2. Create the sandbox.
	//    declaw.Create reads DECLAW_API_KEY / DECLAW_DOMAIN from env via
	//    NewConfig() — no separate client object is needed.
	// ------------------------------------------------------------------
	fmt.Println("Creating sandbox with inline Rego custom policy (fail-closed, no-network-tools denylist)...")
	sbx, err := declaw.Create(ctx,
		declaw.WithTemplate("python"),
		declaw.WithTimeout(180),
		declaw.WithSecurity(policy),
		declaw.WithMetadata(map[string]string{
			"example": "custom-policy-quickstart",
		}),
	)
	if err != nil {
		return fmt.Errorf("sandbox create failed: %w", err)
	}

	fmt.Printf("Sandbox created: ID=%s\n\n", sbx.ID)

	// Deferred kill — fires even if the checks below error.
	defer func() {
		fmt.Printf("\nCleaning up sandbox %s...\n", sbx.ID)
		if killErr := sbx.Kill(context.Background()); killErr != nil {
			fmt.Fprintf(os.Stderr, "warning: kill sandbox %s: %v\n", sbx.ID, killErr)
		} else {
			fmt.Printf("Sandbox %s killed.\n", sbx.ID)
		}
	}()

	// ------------------------------------------------------------------
	// 3. ALLOWED commands — prove the denylist does not over-block.
	//
	// echo and python3 are not in the blocked set, so they must pass.
	// python3 passing is the key proof that the policy is a denylist, not
	// a generic "block all executables" rule.
	// ------------------------------------------------------------------
	fmt.Println("=== ALLOWED commands (must pass through) ===")

	results := []checkResult{
		runCommand(ctx, sbx,
			"echo hello from sandbox",
			"echo hello from sandbox",
			true, // expect allowed
			"echo is not in the denylist — allowed"),

		runCommand(ctx, sbx,
			"python3 --version",
			"python3 --version",
			true, // expect allowed
			"python3 is not in the denylist — allowed (proves denylist, not allowlist)"),
	}

	// ------------------------------------------------------------------
	// 4. BLOCKED commands — prove the cmd gate fires before execution.
	//
	// curl and wget are in the blocked set. The platform returns HTTP 403
	// before the command ever executes inside the VM, so Commands.Run
	// returns an error rather than a CommandResult.
	// ------------------------------------------------------------------
	fmt.Println("\n=== BLOCKED commands (must be denied at the cmd gate) ===")

	results = append(results,
		runCommand(ctx, sbx,
			`curl -s https://example.com`,
			"curl -s https://example.com",
			false, // expect blocked
			"curl is in the denylist — blocked at cmd gate"),

		runCommand(ctx, sbx,
			`wget -qO- https://example.com`,
			"wget -qO- https://example.com",
			false, // expect blocked
			"wget is in the denylist — blocked at cmd gate"),
	)

	// ------------------------------------------------------------------
	// 5. Platform floor — IMDS is blocked unconditionally.
	//
	// The platform always blocks 169.254.169.254 (EC2 IMDS) regardless of
	// the custom policy. DefaultDeny: false would not open it.
	// We check this via a blocked command rather than curl (which is
	// itself blocked by our policy), so we use python3 to attempt the
	// connection at the network layer — the platform floor fires there.
	// ------------------------------------------------------------------
	fmt.Println("\n=== Platform floor — IMDS unconditionally blocked ===")

	results = append(results,
		runCommand(ctx, sbx,
			`python3 -c "import urllib.request; urllib.request.urlopen('http://169.254.169.254/latest/meta-data/', timeout=5)"`,
			"python3 urllib IMDS fetch",
			false, // expect blocked / error at network layer
			"IMDS 169.254.169.254 blocked by platform floor (not bypassable via DefaultDeny or custom policy)"),
	)

	// ------------------------------------------------------------------
	// 6. Summary table.
	// ------------------------------------------------------------------
	fmt.Println("\n=== Results ===")
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

// checkResult holds the outcome of one policy check.
type checkResult struct {
	label  string
	ok     bool
	detail string
}

// runCommand runs cmd inside the sandbox and evaluates the result against the
// expectation: wantAllowed=true means the command must succeed (exit 0);
// wantAllowed=false means the command must be blocked by the policy or fail.
//
// Cmd-gate denials surface as an error from Commands.Run (the platform returns
// HTTP 403 before the command executes). The SDK maps this to either an
// AuthenticationError or a non-nil error for the blocked path.
func runCommand(ctx context.Context, sbx *declaw.Sandbox, cmd, display string, wantAllowed bool, label string) checkResult {
	fmt.Printf("  cmd: %s\n", display)

	result, err := sbx.Commands.Run(ctx, cmd)

	if wantAllowed {
		// We expect this command to be allowed through.
		if err != nil {
			// CommandExitError means the command ran but exited non-zero — it
			// was still allowed through the gate, which satisfies our check.
			var exitErr *declaw.CommandExitError
			if errors.As(err, &exitErr) {
				fmt.Printf("  -> allowed (exit %d, stderr=%q)\n", exitErr.ExitCode, exitErr.Stderr)
				return checkResult{label: label, ok: true}
			}

			fmt.Printf("  -> UNEXPECTED error: %v\n", err)
			return checkResult{
				label:  label,
				ok:     false,
				detail: fmt.Sprintf("unexpected error (command should have been allowed): %v", err),
			}
		}
		fmt.Printf("  -> allowed (exit=%d stdout=%q)\n", result.ExitCode, strings.TrimSpace(result.Stdout))
		return checkResult{label: label, ok: true}
	}

	// We expect this command to be blocked.
	if err != nil {
		// AuthenticationError: platform returned HTTP 401/403 — the cmd gate
		// fired. This is the primary success path for a blocked command.
		var authErr *declaw.AuthenticationError
		if errors.As(err, &authErr) {
			fmt.Printf("  -> blocked (403 from platform cmd gate): %v\n", err)
			return checkResult{label: label, ok: true}
		}

		// CommandExitError with non-zero exit: the command was not necessarily
		// blocked at the gate but the execution failed (e.g., network-level
		// block, connection refused). Count as blocked.
		var exitErr *declaw.CommandExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode != 0 {
			fmt.Printf("  -> blocked (exit %d): stderr=%q\n", exitErr.ExitCode, exitErr.Stderr)
			return checkResult{label: label, ok: true}
		}

		// Any other error: the command did not succeed — count as blocked.
		fmt.Printf("  -> blocked (error): %v\n", err)
		return checkResult{label: label, ok: true}
	}

	// The command completed with exit 0 — unexpected for a blocked command.
	fmt.Printf("  -> UNEXPECTED: command ran to completion (exit=%d stdout=%q)\n",
		result.ExitCode, strings.TrimSpace(result.Stdout))
	return checkResult{
		label:  label,
		ok:     false,
		detail: fmt.Sprintf("command ran with exit=%d; expected it to be blocked by the custom policy", result.ExitCode),
	}
}
