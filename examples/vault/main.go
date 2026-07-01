// Package main demonstrates the Declaw credential vault using the Go SDK.
//
// Secrets are stored and referenced by name — the vault keeps the value
// server-side (in OpenBao) and injects it into outbound HTTP requests at the
// egress proxy, so the raw value never enters the sandbox VM. This example:
//
//  1. Store a secret scoped to postman-echo.com with bearer injection.
//  2. Boot a Python sandbox that references the secret by name via WithVaultRefs.
//  3. Inside the VM, printenv DEMO_TOKEN shows "declaw:vault-managed" — the
//     placeholder, not the value. A curl to postman-echo.com reflects the
//     Authorization header ("Bearer demo-secret-value") injected by the proxy.
//  4. List secrets, rotate the secret value, browse the preset catalog.
//  5. Delete the secret, kill the sandbox.
//
// Usage:
//
//	DECLAW_API_KEY=dk_... DECLAW_DOMAIN=api.declaw.ai go run ./examples/vault
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	vc := declaw.NewVaultClient()
	defer vc.Close()

	// ------------------------------------------------------------------
	// 1. Store a secret, scoped to postman-echo.com with bearer injection.
	//
	// Secrets are addressed by name; team/environment are assigned
	// automatically. The "~" prefix on the domain selects regex matching at
	// the egress proxy (without it the value is an exact-hostname match).
	// "demo-secret-value" is used so this example runs without a real key.
	// ------------------------------------------------------------------
	const secretName = "demo-token"
	fmt.Println("=== Step 1: create secret ===")
	secret, err := vc.CreateSecret(ctx, declaw.CreateSecretInput{
		Name:  secretName,
		Value: "demo-secret-value",
		Scopes: []declaw.VaultScope{
			{DomainRegex: `~^postman-echo\.com$`, InjectionType: "bearer"},
		},
		RotationIntervalDays: 90,
	})
	if err != nil {
		return fmt.Errorf("create secret: %w", err)
	}
	fmt.Printf("Secret created: %s (id=%s)\n\n", secret.Name, secret.SecretID)

	// ------------------------------------------------------------------
	// 2. Boot a sandbox that references the secret BY NAME.
	//
	// WithVaultRefs maps DEMO_TOKEN (the VM env var) to the secret name
	// "demo-token"; the SDK expands it to a full vault ref. The VM sees only
	// the placeholder "declaw:vault-managed"; the proxy injects the real value.
	// ------------------------------------------------------------------
	fmt.Println("=== Step 2: create sandbox with vault reference ===")
	sbx, err := declaw.Create(ctx,
		declaw.WithTemplate("python"),
		declaw.WithTimeout(120),
		declaw.WithVaultRefs(map[string]string{"DEMO_TOKEN": secretName}),
		declaw.WithNetwork(declaw.SandboxNetworkOpts{AllowOut: []string{"postman-echo.com"}}),
	)
	if err != nil {
		return fmt.Errorf("sandbox create failed: %w", err)
	}
	fmt.Printf("Sandbox created: %s\n\n", sbx.ID)
	defer func() {
		fmt.Printf("\n=== Cleanup: kill sandbox %s ===\n", sbx.ID)
		if killErr := sbx.Kill(context.Background()); killErr != nil {
			fmt.Fprintf(os.Stderr, "warning: kill sandbox: %v\n", killErr)
		} else {
			fmt.Printf("Sandbox %s killed.\n", sbx.ID)
		}
	}()

	// ------------------------------------------------------------------
	// 3. Isolation proof: placeholder in the VM env, real value injected.
	// ------------------------------------------------------------------
	fmt.Println("=== Step 3a: isolation proof — env var inside VM ===")
	env, err := sbx.Commands.Run(ctx, "printenv DEMO_TOKEN")
	if err != nil {
		return fmt.Errorf("printenv: %w", err)
	}
	got := strings.TrimSpace(env.Stdout)
	fmt.Printf("DEMO_TOKEN inside VM: %q\n", got)
	if got == "declaw:vault-managed" {
		fmt.Println("PASS: secret value did NOT enter the VM environment.")
	} else {
		fmt.Printf("WARNING: expected the placeholder, got %q\n", got)
	}

	fmt.Println("\n=== Step 3b: isolation proof — bearer injection at egress proxy ===")
	curl, err := sbx.Commands.Run(ctx, "curl -s --max-time 15 https://postman-echo.com/get")
	if err != nil {
		fmt.Printf("warning: curl: %v\n", err)
	} else if strings.Contains(curl.Stdout, "Bearer demo-secret-value") {
		fmt.Println("PASS: \"Bearer demo-secret-value\" reflected — injected at the proxy, not from the VM env.")
	} else {
		fmt.Println("NOTE: expected \"Bearer demo-secret-value\" in the echo body.")
	}

	// ------------------------------------------------------------------
	// 4. List, rotate, and browse presets.
	// ------------------------------------------------------------------
	fmt.Println("\n=== Step 4: list secrets ===")
	secrets, err := vc.ListSecrets(ctx)
	if err != nil {
		return fmt.Errorf("list secrets: %w", err)
	}
	for _, s := range secrets {
		fmt.Printf("  - %s (id=%s, scopes=%d, rotation_due=%v)\n", s.Name, s.SecretID, len(s.Scopes), s.RotationDue)
	}

	fmt.Println("=== Step 5: rotate secret ===")
	if err := vc.RotateSecret(ctx, secretName, "demo-secret-value-v2"); err != nil {
		return fmt.Errorf("rotate: %w", err)
	}
	fmt.Println("Rotated. The new value is injected on the next outbound request.")

	fmt.Println("=== Step 6: list presets (first 5) ===")
	presets, err := vc.ListPresets(ctx)
	if err != nil {
		return fmt.Errorf("list presets: %w", err)
	}
	fmt.Printf("Preset catalog: %d providers\n", len(presets))
	for i, p := range presets {
		if i >= 5 {
			fmt.Println("  ... and more. Use the key as Provider in CreateSecretInput.")
			break
		}
		fmt.Printf("  key=%-18s name=%-22s category=%s\n", p.Key, p.Name, p.Category)
	}

	// ------------------------------------------------------------------
	// 7. Delete the secret (by name).
	// ------------------------------------------------------------------
	fmt.Println("\n=== Step 7: delete secret ===")
	if err := vc.DeleteSecret(ctx, secretName); err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}
	fmt.Printf("Secret %q deleted.\n", secretName)
	return nil
}
