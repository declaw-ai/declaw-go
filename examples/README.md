# Declaw Go SDK — Examples

## vault

Demonstrates the Declaw credential vault end-to-end using the Go SDK's
`VaultClient` and `WithVaultRefs`. The example:

- Creates a vault team and a `prod` environment.
- Stores a secret scoped to `postman-echo.com` with `bearer` injection
  (the raw value lives in OpenBao server-side; it is never returned after
  create and never enters the sandbox VM).
- Boots a Python sandbox that maps `DEMO_TOKEN` to the secret via
  `WithVaultRefs`. Inside the VM `printenv DEMO_TOKEN` prints
  `declaw:vault-managed` — the placeholder, not the value.
- Runs `curl https://postman-echo.com/get` from inside the VM; the egress
  proxy injects `Authorization: Bearer demo-secret-value` so the reflected
  headers confirm the secret is active for outbound calls.
- Lists secrets, rotates the secret value, and browses the built-in
  provider preset catalog (`ListPresets`).
- Cleans up: deletes the secret, kills the sandbox, deletes the team
  (which cascades to the environment).

### How to run

```
DECLAW_API_KEY=dk_... DECLAW_DOMAIN=api.declaw.ai go run ./examples/vault
```

Or with a local dev server:

```
DECLAW_API_KEY=dk_... DECLAW_API_URL=http://localhost:8080 go run ./examples/vault
```

The program reads credentials from the environment exactly as
`declaw.NewConfig()` does: `DECLAW_API_KEY`, `DECLAW_DOMAIN`,
`DECLAW_API_URL`.

### Vault ref format

```
vault://<teamID>/<environmentName>/<secretName>
```

Pass the ref as a value in the `map[string]string` given to `WithVaultRefs`:

```go
declaw.WithVaultRefs(map[string]string{
    "DEMO_TOKEN": "vault://<teamID>/prod/demo-token",
})
```

The sandbox environment variable holds only `declaw:vault-managed`. The
egress proxy resolves the OpenBao path and injects the real value into the
matching outbound request header — at no point does the secret transit the
VM.

### VaultScope injection types

| `InjectionType` | Effect                                              |
|-----------------|-----------------------------------------------------|
| `bearer`        | `Authorization: Bearer <value>`                     |
| `header`        | Custom header named by `HeaderName`                 |
| `basic`         | `Authorization: Basic base64(BasicUsername:value)`  |
| `query`         | URL query parameter named by `HeaderName`           |
| `sigv4`         | AWS Signature Version 4 signing                     |
| `oidc`          | OIDC token exchange                                 |
| `hmac`          | HMAC request signing                                |
| `redis` / `postgres` / `mysql` / `smtp` / `mongodb` | Socket-level credential injection |

### Provider presets

`VaultClient.ListPresets` returns the built-in catalog (~38 providers).
Pass a preset's `Key` as `Provider` in `CreateSecretInput` to skip writing
explicit `Scopes` — the control plane fills them in:

```go
vc.CreateSecret(ctx, teamID, declaw.CreateSecretInput{
    Environment: "prod",
    Provider:    "openai",        // preset key
    Value:       os.Getenv("OPENAI_API_KEY"),
})
```

---

## injection-defense

Demonstrates prompt-injection defense with **domain scoping** using
`InjectionDefenseConfig`. The headline is the opt-in rule: injection scanning
runs **only** on the destination hosts listed in `Domains`. An empty/nil
`Domains` list means **no injection scanning runs** — unlike PII/toxicity,
where an empty list means "scan all egress". The example:

- Builds a `SecurityPolicy` whose `InjectionDefense.Domains` scopes scanning to
  the agent's model endpoint(s) (`api.openai.com`, `*.anthropic.com`).
- Creates a Python sandbox with that policy and prints the applied config.
- Runs a benign command to confirm the sandbox is live (no model credentials
  required).

`Domains` entries accept exact hosts (`api.openai.com`), `*.suffix.com`
wildcards, and `~regex` patterns.

### How to run

```
DECLAW_API_KEY=dk_... DECLAW_DOMAIN=api.declaw.ai go run ./examples/injection-defense
```

Or with a local dev server:

```
DECLAW_API_KEY=dk_... DECLAW_API_URL=http://localhost:8080 go run ./examples/injection-defense
```

---

## custom-policy-quickstart

The fastest path to a working OPA custom policy: a single `InlineRego` module
that blocks four ad-hoc network tools (`curl`, `wget`, `nc`, `ncat`) at the
command gate, fail-closed. The example:

- Creates a sandbox (template `python`) with `CustomPolicyConfig{Enabled: true,
  DefaultDeny: true, InlineRego: <rego>}` — one field, one package, no bundle
  server.
- Runs `echo hello from sandbox` and `python3 --version` and verifies both
  succeed — proving the policy is a **denylist** (not an allowlist).
- Runs `curl -s https://example.com` and `wget -qO- https://example.com` and
  verifies both are rejected at the cmd gate (HTTP 403 before the VM sees the
  command).
- Attempts to reach IMDS (`169.254.169.254`) via `python3 urllib` to show the
  non-bypassable platform floor.
- Prints a `PASS`/`FAIL` table and exits non-zero on any failure.

### How to run

```
DECLAW_API_KEY=dk_... DECLAW_DOMAIN=api.declaw.ai go run ./examples/custom-policy-quickstart
```

Or with a local dev server:

```
DECLAW_API_KEY=dk_... DECLAW_API_URL=http://localhost:8080 go run ./examples/custom-policy-quickstart
```

### InlineRego vs InlineModules — when to use which

| Field           | Use case                                                                      |
|-----------------|-------------------------------------------------------------------------------|
| `InlineRego`    | Single `declaw.platform.cmd` (or `network`) policy — fits in one package     |
| `InlineModules` | Rules that must live in *different* packages (e.g., `cmd` **and** `network`) |

This example uses `InlineRego` because all rules are in `declaw.platform.cmd`.
See `opa-custom-policy` for the `InlineModules` form.

### DefaultDeny: fail-closed explained

```go
CustomPolicyConfig{
    Enabled:     true,
    DefaultDeny: true,   // <-- deny on OPA engine error, not allow
    InlineRego:  regoNoNetworkTools,
}
```

`DefaultDeny: true` means that if the OPA evaluator is unreachable or returns
an evaluation error, the platform denies the action. Use this for all security
gates. `DefaultDeny: false` (fail-open) is only appropriate for advisory-only
scanners where uptime matters more than strictness.

---

## opa-custom-policy

Demonstrates how to attach OPA/Rego custom policies to a Declaw sandbox using
the Go SDK's `CustomPolicyConfig` and `InlineModules` fields. The example:

- Creates a sandbox with two independent Rego modules (`InlineModules`) and
  `DefaultDeny: true` (fail-closed).
- Runs five commands that exercise both the cmd-gate and the network-gate, plus
  the non-bypassable platform floor.
- Prints a `PASS`/`FAIL` result for each check and exits non-zero on any failure.

### How to run

```
DECLAW_API_KEY=dk_... DECLAW_DOMAIN=api.declaw.ai go run ./examples/opa-custom-policy
```

Or with an explicit API URL (e.g. local dev):

```
DECLAW_API_KEY=dk_... DECLAW_API_URL=http://localhost:8080 go run ./examples/opa-custom-policy
```

The program reads credentials from the environment exactly as `declaw.NewConfig()`
does: `DECLAW_API_KEY`, `DECLAW_DOMAIN`, `DECLAW_API_URL`. No flags are needed.

### Rego contract

| Package                      | Input field                        | Type           | Purpose                                   |
|------------------------------|------------------------------------|----------------|-------------------------------------------|
| `declaw.platform.cmd`        | `input.action.command`             | `string`       | Executable name being invoked (no args)   |
| `declaw.platform.cmd`        | `input.action.args`                | `[]string`     | Argument list                             |
| `declaw.platform.network`    | `input.action.destination`         | `string`       | Egress target hostname or `host:port`     |
| `declaw.platform.content`    | `input.attributes.scan_results.*`  | object         | Guardrails scanner results (PII, etc.)    |
| `declaw.platform.content`    | `input.attributes.model`           | `string`       | Model identifier for content checks      |

Rules must use the `deny` partial rule form:

```rego
deny contains msg if {
    <condition>
    msg := "<human-readable reason>"
}
```

Custom policy rules are **additive**: they can only tighten the platform-default
floor, never relax it. A rule may only produce additional denies.

### Gate behavior: cmd denial vs. network denial

| Gate          | Enforcement point           | Observed effect                                                       |
|---------------|-----------------------------|-----------------------------------------------------------------------|
| `cmd`         | Before the command executes | `Commands.Run` returns an error (platform returns HTTP 403 before the VM sees the command) |
| `network`     | Egress connection layer     | The command starts and runs normally inside the VM, but the outbound TCP connection is silently dropped; `curl` exits non-zero (connection refused / timed out) |
| Platform floor | Always-on, not configurable | Link-local (169.254.x.x / IMDS), loopback-only services, and other unconditionally-denied destinations are blocked regardless of any customer policy — `DefaultDeny: false` does not open them |

### InlineRego vs. InlineModules

| Field           | Use case                                                           |
|-----------------|--------------------------------------------------------------------|
| `InlineRego`    | A single Rego policy that fits in one package (most common case)  |
| `InlineModules` | Multiple independent packages — each element must have its own `package` declaration; the evaluator loads them as separate modules |

Using `InlineModules` is required whenever cmd and network rules need to live in
`declaw.platform.cmd` and `declaw.platform.network` respectively, because those
are different package namespaces and cannot be merged into a single string.

### Policy serialization

`SecurityPolicy.ToJSON()` serializes `InlineModules` as a JSON array of strings
under the `custom_policy.inline_modules` key. The SDK sends this in the
`POST /sandboxes` request body. No external OPA bundle server is required for
inline policies.
