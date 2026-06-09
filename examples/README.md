# Declaw Go SDK — Examples

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
