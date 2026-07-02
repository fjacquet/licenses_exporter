# vmware_licenses_exporter — Design

**Date:** 2026-07-02
**Status:** Approved (brainstorming)
**Family:** third sub-project of the `licenses_exporter` split (after `licenses-exporter-core` v0.1.0 and `m365_licenses_exporter`).

## Purpose

A standalone VMware vSphere license exporter for the Prometheus/Grafana stack — the
VMware sibling in the `licenses_exporter` family. It consumes the shared engine
`github.com/fjacquet/licenses-exporter-core` and keeps only the VMware collector
(vSphere `LicenseManager` via govmomi). Because every family exporter emits the identical
`license_` schema (built solely through core constructors), VMware license series land in
the same Prometheus and the same cross-vendor Grafana / alerting view as the M365 exporter.

This is also core's **second independent consumer** — the trigger, per core's versioning
rule, to promote `licenses-exporter-core` from `v0.1.0` to `v1.0.0`.

## Architecture

Structurally identical to `m365_licenses_exporter`:

- A thin `main.go` (cobra) delegates the whole lifecycle — `--once`, or serve `/metrics` +
  `/health` with signal + file-watch hot reload — to `core.Main`.
- A consumer `Config` embeds `core.Base` (collection + otlp) inline, plus a vendor `vmware:`
  block.
- The repo owns only `internal/vmware` (the govmomi `LicenseManager` collector) and the
  consumer `Config`. The engine (schema, snapshot store, collection loop, dual export,
  reload server) lives entirely in core.

```
main.go ── core.Main(App{Load}) ──> core engine (snapshot / dual export / reload)
              │
              └─ Load: LoadYAML → Base.Validate → vmware.NewSources(cfg.VMware) → []core.Source
                                                        │
                                                        └─ internal/vmware (govmomi LicenseManager)
```

## Repo bootstrap (new repo, not an in-place conversion)

- New repo `~/Projects/vmware_licenses_exporter`; module
  `github.com/fjacquet/vmware_licenses_exporter`; Go 1.26.x; `require
  github.com/fjacquet/licenses-exporter-core v0.1.0` with **no `replace`**.
- **Scaffolding** copied from the freshly-renamed `m365_licenses_exporter` (the most complete
  core-consumer template): `Makefile`, `.goreleaser.yaml`, `Dockerfile`,
  `Dockerfile.goreleaser`, `docker-compose.yml`, `docker-compose.ghcr.yml`, `.golangci.yml`,
  `.gitignore`, `LICENSE`, CI workflows under `.github/`, `mkdocs.yml`, and the `docs/`
  skeleton — with `m365`→`vmware` renamed throughout. **Default port `9106`** (m365's `9105`
  + 1) so the two exporters co-locate on one host without a `--web.listen-address` override;
  a shared Prometheus scrapes each on its own address. Update the `--web.listen-address`
  default, `docker-compose*.yml` port mappings, `prometheus.yml` scrape target, and docs to
  `9106`.
- **Collector source:** `internal/vmware/{parse,source,vmware}.go` and `{parse,source}_test.go`
  are taken from `licenses_exporter` **main** (still intact there — the m365 conversion that
  deletes them is only on an unmerged PR branch). Rewire: imports `internal/license` → `core`
  and `internal/config` → `core`; move the config types in as `VMwareConfig` / `VCenterConfig`
  (below); `NewSources(cfg VMwareConfig) ([]core.Source, error)`; every sample built via
  `core.SeatSample` / `core.ExpirationSample` with `core.MetricSeatsTotal` / `core.MetricSeatsUsed`.

## Config

```go
type Config struct {
    core.Base `yaml:",inline"`               // collection.interval + otlp.{endpoint,insecure}
    VMware    vmware.VMwareConfig `yaml:"vmware"`
}

// in internal/vmware:
type VMwareConfig struct {
    Enabled  bool            `yaml:"enabled"`
    VCenters []VCenterConfig `yaml:"vcenters"`
}
type VCenterConfig struct {
    Instance           string `yaml:"instance"`
    Host               string `yaml:"host"`
    Username           string `yaml:"username"`
    Password           string `yaml:"password"`
    PasswordFile       string `yaml:"passwordFile"`
    InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
}
```

`config.yaml` shape: `collection` / `otlp` / `vmware` (with `${VC_*}` env refs or
`passwordFile` for secrets; `enabled: true`; one or more `vcenters`).

## Metrics (ported as-is — behaviour parity)

Unchanged `license_` schema via core constructors:

- `license_seats_total{vendor="vmware",product,unit,instance}` — **omitted** when
  `Total <= 0` (an unlimited key emits only seats_used — absent-not-zero).
- `license_seats_used{vendor="vmware",product,unit,instance}` — always.
- `license_expiration_timestamp_seconds{vendor="vmware",product,instance}` — from the
  `expirationDate` license property; **absent** for perpetual licenses (no `9999` sentinel).
- `license_up` / `license_build_info` and the collector health metrics — from core.

`vendor = "vmware"`; `unit = info.CostUnit` (falls back to `"unit"` when empty); `product =
info.Name`. No exporter-computed `days_to_expiration` / `compliance_status` — derive in PromQL.

## Auth & reliability (ported)

- **govmomi**, **stateless per collection cycle**: login → query `LicenseManager` → logout
  (on a fresh background context so it runs even if the cycle context is cancelled) → close.
  No persisted session/cookie. A logout failure is a `logrus` warn, not a cycle failure.
- Secrets are `${ENV}` refs or `passwordFile` only, resolved via `core.ResolveSecret`; never
  hardcoded or logged. `insecureSkipVerify` is per-vCenter.
- A source failure degrades to `license_up{vendor="vmware",instance}=0` — never crashes the
  cycle (core guarantees this).
- `--trace` logs only repo-owned request/response bodies; it **never** enables govmomi SDK
  debug modes (which would leak the session cookie / credentials).

## Client decision (ADR)

**Keep govmomi** — the official, mature vSphere Go SDK. Per the family's client rule it is
both available *and* useful (session-based login, models the `LicenseManager` view and the
license capacity/usage/expiration fields we export). No hand-rolled `resty` client and no
regression, so no "SDK-not-useful" ADR is warranted. The repo records one ADR: *consume
licenses-exporter-core + retain govmomi* (mirrors m365's ADR-0010).

## Testing / correctness oracle

- The ported `internal/vmware/parse_test.go` and `source_test.go` pass **unchanged** (they
  reference no deleted packages) — the byte-for-byte proof that VMware sample output is
  identical to the unified exporter's. `source_test.go` exercises the collector via its
  seam (the existing test double / `vcsim` path); `parse_test.go` covers `licensesToSamples`
  including the `Total<=0` unlimited case and the expiration-property extraction.
- A `main` config-load test proves the consumer `Config` wires `core.Base` + the `vmware`
  block and that `loadConfig` builds one `core.Source` per enabled vCenter.
- The green gate is the family CI: `make ci` (gofmt, vet, golangci-lint, `-race` tests,
  govulncheck, build) + `make release-snapshot` + `mkdocs build --strict` + semgrep 0.

## Core v1.0.0 promotion (final task)

1. Build vmware against core **v0.1.0**; achieve the full green gate.
2. The successful second-consumer compile + passing tests are the evidence the API is stable
   → tag `licenses-exporter-core` **v1.0.0** (update its README versioning note + CHANGELOG),
   and bump vmware's `require` from `v0.1.0` to `v1.0.0`; re-verify `make ci`.
3. Then cut vmware's own first release.

## Finish / release

New **public** repo `github.com/fjacquet/vmware_licenses_exporter` (visibility confirmed at
publish, matching the family). Execution: subagent-driven development, Sonnet 5
implementers/reviewers, Opus final whole-branch review.

## Out of scope

- `veeam_licenses_exporter` (needs a Veeam API research pass first) — a later sub-project.
- Any change to the VMware collector's logic beyond the import rewire (behaviour parity is
  binding; the oracle tests must pass unchanged).
- Mapping socket↔core licensing semantics (VCF/VVF): the collector emits the raw
  `LicenseManager` `Total`/`Used` with the vendor-reported `CostUnit`; any core-vs-socket
  interpretation is a downstream PromQL/dashboard concern, not exporter logic.
