// Package main demonstrates prompt-injection defense with domain scoping using
// the Declaw Go SDK.
//
// Headline: injection scanning is OPT-IN PER DOMAIN. It runs ONLY on the
// destination hosts listed in InjectionDefenseConfig.Domains. An empty/nil
// Domains list means NO injection scanning runs — this differs from PII and
// toxicity scanning, where an empty list means "scan all egress". Scope
// Domains to the model/LLM endpoint(s) your agent actually calls.
//
// Each Domains entry can be:
//   - an exact host:      "api.openai.com"
//   - a wildcard:         "*.anthropic.com"  (any subdomain, not the apex)
//   - a regex (~ prefix): "~.*\\.anthropic\\.com$"
//
// This example creates a sandbox with injection defense scoped to a model
// endpoint, prints the applied configuration, and runs a benign command to
// show the sandbox is live. It does not require model credentials.
//
// Usage:
//
//	DECLAW_API_KEY=dk_... DECLAW_DOMAIN=api.declaw.ai go run ./examples/injection-defense
//
// Or with a local dev server:
//
//	DECLAW_API_KEY=dk_... DECLAW_API_URL=http://localhost:8080 go run ./examples/injection-defense
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	declaw "github.com/declaw-ai/declaw-go"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if os.Getenv("DECLAW_API_KEY") == "" {
		return fmt.Errorf("DECLAW_API_KEY must be set")
	}

	domain := os.Getenv("DECLAW_DOMAIN")
	if domain == "" {
		domain = "api.declaw.ai"
	}
	fmt.Printf("Connecting to Declaw API at %s\n\n", domain)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// ------------------------------------------------------------------
	// 1. Build a security policy with injection defense scoped to the model
	//    endpoint(s) the agent calls.
	//
	//    Injection is OPT-IN per domain: scanning runs ONLY on the hosts in
	//    Domains. Leaving Domains empty/nil disables injection scanning
	//    entirely (unlike PII/toxicity, where empty = all egress). Entries
	//    accept exact hosts, "*.suffix.com" wildcards, and "~regex" patterns.
	// ------------------------------------------------------------------
	injectionDomains := []string{"api.openai.com", "*.anthropic.com"}
	policy := declaw.SecurityPolicy{
		InjectionDefense: &declaw.InjectionDefenseConfig{
			Enabled:     true,
			Sensitivity: declaw.InjectionSensitivityMedium,
			Action:      declaw.InjectionActionBlock,
			Domains:     injectionDomains,
		},
	}

	// ------------------------------------------------------------------
	// 2. Create the sandbox. declaw.Create reads DECLAW_API_KEY /
	//    DECLAW_DOMAIN / DECLAW_API_URL from the environment via NewConfig().
	// ------------------------------------------------------------------
	fmt.Println("Creating sandbox with injection defense scoped to the model endpoint...")
	sbx, err := declaw.Create(ctx,
		declaw.WithTemplate("python"),
		declaw.WithTimeout(180),
		declaw.WithSecurity(policy),
		declaw.WithMetadata(map[string]string{
			"example": "injection-defense",
		}),
	)
	if err != nil {
		return fmt.Errorf("sandbox create failed: %w", err)
	}
	fmt.Printf("Sandbox created: ID=%s\n\n", sbx.ID)

	defer func() {
		fmt.Printf("\nCleaning up sandbox %s...\n", sbx.ID)
		if killErr := sbx.Kill(context.Background()); killErr != nil {
			fmt.Fprintf(os.Stderr, "warning: kill sandbox %s: %v\n", sbx.ID, killErr)
		} else {
			fmt.Printf("Sandbox %s killed.\n", sbx.ID)
		}
	}()

	// ------------------------------------------------------------------
	// 3. Show the applied injection-defense configuration.
	// ------------------------------------------------------------------
	fmt.Println("Injection defense configuration:")
	fmt.Printf("  enabled:     %t\n", policy.InjectionDefense.Enabled)
	fmt.Printf("  sensitivity: %s\n", policy.InjectionDefense.Sensitivity)
	fmt.Printf("  action:      %s\n", policy.InjectionDefense.Action)
	fmt.Printf("  domains:     %v  <- scanning runs ONLY on these hosts\n", policy.InjectionDefense.Domains)
	fmt.Println()

	// ------------------------------------------------------------------
	// 4. Run a benign command to confirm the sandbox is live. Outbound
	//    requests to the scoped hosts have their bodies inspected for
	//    injection; traffic to any other host is not injection-scanned.
	// ------------------------------------------------------------------
	fmt.Println("Running a benign command inside the sandbox...")
	result, err := sbx.Commands.Run(ctx, "python3 --version")
	if err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	fmt.Printf("  -> %s\n", strings.TrimSpace(result.Stdout+result.Stderr))

	fmt.Println("\nWith injection defense enabled and scoped, the egress proxy")
	fmt.Println("inspects request/response bodies for the listed domains and blocks")
	fmt.Println("detected prompt-injection attempts before they reach the model.")
	return nil
}
