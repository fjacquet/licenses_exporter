# 10. Consume licenses-exporter-core instead of an in-repo engine

Date: 2026-07-02

## Status
Accepted

## Context
The unified exporter's vendor-neutral engine (schema, snapshot store, collection
loop, dual export, hot-reload server) was extracted into the reusable library
`github.com/fjacquet/licenses-exporter-core` v0.1.0. Keeping a private copy here
would let the `license_` schema drift between per-vendor exporters — the exact
outcome the split exists to prevent.

## Decision
This exporter depends on `licenses-exporter-core` and builds every sample through
its constructors. `main.go` delegates the whole lifecycle to `core.Main`; the repo
owns only `internal/m365` (the Graph collector) and the consumer `Config`.

## Consequences
- Schema identity is guaranteed by construction — no local `license_` metric code.
- Build time drops (no vSphere/govmomi tree; the engine compiles once, upstream).
- Engine bugfixes/features arrive via a core version bump, not a local edit.
- A core API change can require a coordinated bump here (acceptable during core's
  0.x settling window).
- Startup is now fatal on an unbuildable-but-valid config (core behaviour); see the
  core CHANGELOG.
