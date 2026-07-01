# Changelog

All notable changes to the Declaw Go SDK are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
(Go module rules: `v0.x` — no stability promise yet).

## [v0.5.0] — 2026-07

_2026-07 train: credential vault client + injection domain scoping._

### Added

- Credential vault client — `VaultClient` (via `NewVaultClient`) for managing
  secrets **by name**: `CreateSecret`, `ListSecrets`, `RotateSecret`,
  `DeleteSecret`, `UpdateScopes`, and `ListPresets`. Secret values are
  write-only (never returned after create). Attach secrets to a sandbox with
  `WithVaultRefs`; the value is injected at the egress proxy and never enters
  the sandbox (#386, #399, #408, #456).
- `Domains` on `FullInjectionDefenseOptions` / `InjectionDefenseConfig` — opt-in
  scoping of injection scanning to specific destination hosts.

## [v0.4.0] — 2026-06

_2026-06 train: file-granular volumes, OPA governance._

### Added

- Mode-based volumes: write-back, file-granular backend, mount mode —
  detached `volume.files.*` / `empty` / `ingest` API surface (#344).
- OPA custom-policy support for AI agents: `content_gate` field,
  `policy_ref` resolution, out-of-box AI governance packs (#279, #345).

## [v0.3.1] — 2026-05

### Fixed

- Copyright year in LICENSE.
- Minor fixes following the v0.3.0 stdio/mcp wave.

## [v0.3.0] — 2026-05

### Added

- Native interactive stdio for sandboxed processes (#320).
- Inbound HTTP port proxy for sandboxes (#315).

### Fixed

- Stdio double-encoding (found during `declaw mcp` work, #328).

## [v0.2.0] — 2026-04

### Added

- Snapshots, account management, full security-policy surface parity.

### Fixed

- Bugs found during CLI E2E testing.

## [v0.1.0] — 2026-04

### Added

- Initial release: full API coverage — `Sandbox.Create`, commands, files,
  PTY, volumes, templates. Zero dependencies (#297).

[Unreleased]: https://github.com/declaw-ai/declaw-go/compare/v0.3.1...HEAD
[v0.3.1]: https://github.com/declaw-ai/declaw-go/compare/v0.3.0...v0.3.1
[v0.3.0]: https://github.com/declaw-ai/declaw-go/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/declaw-ai/declaw-go/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/declaw-ai/declaw-go/releases/tag/v0.1.0
