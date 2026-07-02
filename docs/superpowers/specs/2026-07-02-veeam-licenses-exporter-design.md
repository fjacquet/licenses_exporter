# veeam_licenses_exporter — Design

**Date:** 2026-07-02
**Status:** Approved (brainstorming)
**Family:** fourth sub-project of the `licenses_exporter` split (after core, m365, vmware).

## Purpose

A standalone Veeam license exporter for the Prometheus/Grafana stack — the Veeam sibling in
the `licenses_exporter` family. It consumes `github.com/fjacquet/licenses-exporter-core`
(v1.0.0) and adds a single vendor collector for Veeam licensing. Because every family
exporter emits the identical `license_` schema (built solely through core constructors),
Veeam license series land in the same Prometheus and the same cross-vendor Grafana / alerting
view as m365 and vmware.

## Key research finding (drives the whole design)

**Veeam license data is exposed only via the Veeam Backup Enterprise Manager (EM) REST API**
(`https://<em-host>:9398/api`), **not** the VBR REST API. The VBR REST API (`:9419`, v1.1/v1.2)
OpenAPI spec has **no** license endpoint (verified against the VeeamHub SDK OpenAPI spec).
Community Veeam exporters (peekjef72, RezaImany) likewise target Enterprise Manager. Therefore
this exporter **requires Enterprise Manager** to be installed and reachable — a common
component in licensed/multi-server deployments. This requirement is documented prominently in
the README. (User confirmed EM is available.)

## Architecture

Structurally identical to `m365_licenses_exporter` / `vmware_licenses_exporter`:

- A thin `main.go` (cobra) delegates the whole lifecycle — `--once`, or serve `/metrics` +
  `/health` with signal + file-watch hot reload — to `core.Main`.
- A consumer `Config` embeds `core.Base` (collection + otlp) inline, plus a vendor `veeam:` block.
- The repo owns only `internal/veeam` (the EM REST client + parser) and the consumer `Config`.
  The engine lives entirely in core.

```
main.go ── core.Main(App{Load}) ──> core engine (snapshot / dual export / reload)
              │
              └─ Load: LoadYAML → Base.Validate → veeam.NewSources(cfg.Veeam) → []core.Source
                                                        │
                                                        └─ internal/veeam (EM REST: session → GET /api/licensing)
```

## Client decision (ADR): hand-rolled resty, no SDK

There is **no official Veeam Go SDK**; the unofficial `VeeamHub/veeam-vbr-sdk-go` targets the
VBR REST API and **does not cover licensing** at all (confirmed — its OpenAPI spec has no
license schema). Per the family client rule (and matching the backup sibling `nbu_exporter`),
the collector is a **hand-rolled `resty/v2` client**. An ADR records this.

## Collector `internal/veeam`

Session auth is stateless per collection cycle (mirrors vmware's login→query→logout):

1. **Login:** `POST /api/sessionMngr/?v=latest` with HTTP Basic (`username:password`) →
   response carries the `X-RestSvcSessionId` header (the session token).
2. **Query:** `GET /api/licensing` with `Accept: application/json` and header
   `X-RestSvcSessionId: <token>` → the license object.
3. **Logout:** `DELETE /api/logonSessions/<id>` (or `DELETE` the session link) on a fresh
   bounded-context request so it runs even if the cycle context was cancelled; a logout
   failure is a `logrus` warn, never a cycle failure.

TLS: `insecureSkipVerify` per server (EM commonly uses a self-signed cert). The session token
is **never** logged; `--trace` logs only repo-owned request/response bodies and MUST redact the
`X-RestSvcSessionId` header and the Basic credential.

### JSON model isolation + field verification (important)

The exact JSON field names of the EM `/api/licensing` response cannot be verified here (no live
EM reachable). Therefore:

- The response model is **isolated in one small file** (`internal/veeam/model.go`) so field
  names / json tags can be corrected after verification against a live EM without touching the
  parser or client.
- The parser is **tolerant** (absent-not-zero): a missing/zero counter yields an **absent**
  sample, never a fake `0`; an unparseable expiration yields no expiration sample.
- Best-known field mapping (to be confirmed against the deployment's `em_rest` reference /
  a live `GET /api/licensing`): `LicensedInstancesNumber` → seats_total, `UsedInstancesNumber`
  → seats_used, `ExpirationDate` (RFC3339) → expiration, `Edition`/`LicensedTo` → `product`
  label. Because the mapping is unverified against a live instance, the **first release is
  `v0.1.0`** (an API-settling window), not `v1.0.0`.

## Config

```go
type Config struct {
    core.Base `yaml:",inline"`               // collection.interval + otlp.{endpoint,insecure}
    Veeam     veeam.VeeamConfig `yaml:"veeam"`
}
type VeeamConfig struct {
    Enabled bool           `yaml:"enabled"`
    Servers []ServerConfig `yaml:"servers"`   // one Enterprise Manager per entry
}
type ServerConfig struct {
    Instance           string `yaml:"instance"`
    Host               string `yaml:"host"`        // https://em-host:9398
    Username           string `yaml:"username"`
    Password           string `yaml:"password"`
    PasswordFile       string `yaml:"passwordFile"`
    InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
}
```

`config.yaml`: `collection` / `otlp` / `veeam` (with `${VEEAM_*}` env refs or `passwordFile`).

## Metrics (raw-facts, via core constructors)

- `license_seats_total{vendor="veeam",product,unit="instances",instance}` — licensed instances;
  **omitted** when unlimited / non-positive.
- `license_seats_used{vendor="veeam",product,unit="instances",instance}` — used instances.
- `license_expiration_timestamp_seconds{vendor="veeam",product,instance}` — from `ExpirationDate`;
  **absent** for perpetual.
- `license_up` / `license_build_info` and collector health metrics — from core.

`vendor="veeam"`; `unit="instances"` (Veeam Universal License / instance model); `product`=license
edition (fallback to a stable constant if the API omits it). No exporter-computed
`days_to_expiration` / `compliance_status`.

## Auth & reliability

- Secrets are `${ENV}` refs or `passwordFile` only, via `core.ResolveSecret`; never hardcoded or
  logged.
- A source failure (EM unreachable, auth 4xx, parse error) degrades to
  `license_up{vendor="veeam",instance}=0` — never crashes the cycle (core guarantees this).
- Retry excludes 4xx (never retry an auth failure) — matches the family policy; the resty client
  configures a bounded retry on 5xx/transport errors only.
- Default metrics port **9107** (m365=9105, vmware=9106, veeam=9107) so all three co-locate on
  one host.

## Testing / correctness oracle

New code (nothing to port) → real TDD. The oracle is an **`httptest` server** the collector's
client talks to, returning canned responses for `POST /api/sessionMngr` (sets the
`X-RestSvcSessionId` header) and `GET /api/licensing` (a representative license JSON). Tests:

- `parseLicense` unit tests: limited license → total+used+expiration; unlimited → omit
  seats_total; perpetual → omit expiration; missing counters → absent (not zero).
- A client integration test against the `httptest` server: session login → GET licensing →
  logout, asserting the produced `[]core.Sample` and that a 401 degrades cleanly (source error →
  `license_up=0` at the engine level).
- Never logs the session token / credentials (assert redaction on the `--trace` path).

Green gate: family `make ci` + `make release-snapshot` + `mkdocs build --strict` + semgrep 0.

## Repo bootstrap & release

- New repo `~/Projects/veeam_licenses_exporter`; module `github.com/fjacquet/veeam_licenses_exporter`;
  Go 1.26.x; `require licenses-exporter-core v1.0.0` (no `replace`).
- Scaffolding copied from the `vmware_licenses_exporter` template (already a clean core-consumer,
  fresh ADR-0001 numbering), renamed `vmware`→`veeam`, `9106`→`9107`, govmomi→resty in deps/docs.
- New public repo `github.com/fjacquet/veeam_licenses_exporter`, first tag **v0.1.0** (field
  mapping unverified against live EM; promote to v1.0.0 after verification). Visibility confirmed
  at publish.
- Execution: subagent-driven development, Sonnet 5 implementers/reviewers, Opus final review.

## Out of scope

- VBR-direct (`:9419`) or PowerShell license access — EM REST is the chosen path.
- Non-license Veeam metrics (job status, repository capacity) — a different metric family, not
  `license_`.
- Multi-tenant / VSPC aggregation — a single EM per configured server entry.
