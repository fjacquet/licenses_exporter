# vmware_licenses_exporter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `vmware_licenses_exporter` — a standalone VMware vSphere license exporter that consumes `licenses-exporter-core`, keeps the govmomi `LicenseManager` collector, and (as its final task) promotes core to v1.0.0 as the second independent consumer.

**Architecture:** A new repo structurally identical to `m365_licenses_exporter`: a thin `main.go` (cobra) delegates the whole lifecycle to `core.Main`; a `Config` embeds `core.Base` plus a `vmware:` block; the repo owns only `internal/vmware` (govmomi `LicenseManager`) and the consumer `Config`. Every sample is built through core constructors, so the `license_` wire schema is identical to the other family exporters.

**Tech Stack:** Go 1.26.x, `github.com/fjacquet/licenses-exporter-core` v0.1.0, `github.com/vmware/govmomi` (incl. `simulator`/vcsim for tests), `github.com/spf13/cobra`, `github.com/sirupsen/logrus`.

## Global Constraints

- New repo at `~/Projects/vmware_licenses_exporter`; module `github.com/fjacquet/vmware_licenses_exporter`; Go 1.26.x.
- Depend on `github.com/fjacquet/licenses-exporter-core v0.1.0` — published, **no `replace` directive**. (Bumped to v1.0.0 in Task 4.)
- Keep **govmomi** (mature official vSphere SDK — the family's standing choice; no hand-rolled client, no "SDK-not-useful" ADR).
- Schema identity: build every `Sample` only through core constructors (`core.SeatSample`, `core.ExpirationSample`, `core.MetricSeatsTotal`, `core.MetricSeatsUsed`) — never a raw `core.Sample{}` literal.
- Raw-facts / absent-never-zero, preserved byte-for-byte from the source collector: `seats_total` **omitted** when `Total <= 0` (unlimited); `seats_used` always; `license_expiration_timestamp_seconds` from the `expirationDate` property, **absent** for perpetual (no `9999` sentinel). `vendor = "vmware"`, `unit = info.CostUnit` (fallback `"unit"`), `product = info.Name`.
- Secrets are `${ENV}` refs or `passwordFile` only, via `core.ResolveSecret` — never hardcoded or logged.
- Auth: govmomi stateless per cycle (login → `LicenseManager.List` → bounded-context logout + close); logout failure is a `logrus` warn, never a cycle failure. `insecureSkipVerify` per vCenter.
- `--trace` never enables govmomi SDK debug (would leak the session cookie / credentials); it only logs repo-owned bodies.
- Default metrics port **9106** (m365's 9105 + 1, so the two co-locate on one host).
- No inline `//nolint` / `# nosemgrep` except the ratified `# nosemgrep` on CI caller `uses: fjacquet/ci/...@vN` lines.
- **Correctness oracle:** the ported `internal/vmware/parse_test.go` + `source_test.go` (vcsim) pass **unchanged** (they reference no `internal/license`/`internal/config` symbols) — the byte-for-byte proof VMware sample output matches the unified exporter.
- Family Makefile target contract (`make cli`, `make ci`, `make release-snapshot`), CI, GoReleaser, docs (`docs/metrics.md`) + `CHANGELOG.md`, and one ADR updated in the same work.
- Commit trailer on every commit:
  ```
  Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_01XksHzPMuvUvvkbPgLWXTN9
  ```

**Source locations (verbatim ports):**
- Collector code + tests: `git -C /Users/fjacquet/Projects/licenses_exporter show main:internal/vmware/<file>`.
- Scaffolding template (already single-vendor, core-consuming): `git -C /Users/fjacquet/Projects/licenses_exporter show feat/m365-conversion:<file>` — rename `m365`→`vmware`, `9105`→`9106`.

## File Structure

**Created (new repo `~/Projects/vmware_licenses_exporter`):**
- `go.mod` / `go.sum`
- `internal/vmware/config.go` — `VMwareConfig` + `VCenterConfig`
- `internal/vmware/{parse,source,vmware}.go` — rewired collector; `{parse,source}_test.go` — copied unchanged
- `config.go` (package `main`) — consumer `Config` + `loadConfig`
- `main.go` (package `main`) — thin cobra wrapper; `main_test.go`
- `config.yaml`
- Scaffolding: `Makefile`, `.goreleaser.yaml`, `Dockerfile`, `Dockerfile.goreleaser`, `docker-compose.yml`, `docker-compose.ghcr.yml`, `.golangci.yml`, `.gitignore`, `LICENSE`, `.github/**` (CI), `mkdocs.yml`
- Docs: `README.md`, `docs/metrics.md`, `docs/dashboards.md`, `docs/deployment/docker.md`, `docs/adr/index.md`, `docs/adr/0001-consume-core-retain-govmomi.md`, `CHANGELOG.md`
- `grafana/**`, `prometheus.yml`

---

## Task 1: Bootstrap repo + VMware collector on core

Stand up the new module and port the collector so it compiles as a core consumer (library, no binary yet). The oracle tests are the gate.

**Files:**
- Create: `~/Projects/vmware_licenses_exporter/go.mod`
- Create: `internal/vmware/config.go`, `internal/vmware/parse.go`, `internal/vmware/source.go`, `internal/vmware/vmware.go`
- Copy unchanged: `internal/vmware/parse_test.go`, `internal/vmware/source_test.go`

**Interfaces:**
- Consumes from core v0.1.0: `core.Source` (`Vendor()/Instance()/Collect(ctx)([]core.Sample,error)`); `core.Sample`; `core.SeatSample(name, vendor, product, unit, instance string, v float64) core.Sample`; `core.ExpirationSample(vendor, product, instance string, tsUnix float64) core.Sample`; consts `core.MetricSeatsTotal`, `core.MetricSeatsUsed`; `core.ResolveSecret(inline, file string) (string, error)`. Import alias `core "github.com/fjacquet/licenses-exporter-core"`.
- Produces for Task 2: `vmware.NewSources(cfg vmware.VMwareConfig) ([]core.Source, error)`; `vmware.VMwareConfig{Enabled bool; VCenters []vmware.VCenterConfig}`; `vmware.VCenterConfig{Instance, Host, Username, Password, PasswordFile string; InsecureSkipVerify bool}`.

- [ ] **Step 1: Init the repo and module**

```bash
mkdir -p ~/Projects/vmware_licenses_exporter && cd ~/Projects/vmware_licenses_exporter
git init
go mod init github.com/fjacquet/vmware_licenses_exporter
go get github.com/fjacquet/licenses-exporter-core@v0.1.0
```
Add a `.gitignore` (copy from the m365 template in Step order is fine, but at minimum ignore `bin/`, `dist/`, `*.out`, `coverage.html`, `.env`, `.superpowers/`).

- [ ] **Step 2: Create `internal/vmware/config.go`**

```go
package vmware

// VMwareConfig is the vSphere block of the exporter config. Enabled=false (or an
// empty VCenters list) yields zero sources — the exporter then serves only
// license_build_info.
type VMwareConfig struct {
	Enabled  bool            `yaml:"enabled"`
	VCenters []VCenterConfig `yaml:"vcenters"`
}

// VCenterConfig is one vCenter target. Password is an inline ${ENV} ref;
// PasswordFile is a path read at load (ResolveSecret governs precedence).
type VCenterConfig struct {
	Instance           string `yaml:"instance"`
	Host               string `yaml:"host"`
	Username           string `yaml:"username"`
	Password           string `yaml:"password"`
	PasswordFile       string `yaml:"passwordFile"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
}
```

- [ ] **Step 3: Create `internal/vmware/parse.go`** (rewired from `main:internal/vmware/parse.go` — import path only)

```go
package vmware

import (
	"time"

	core "github.com/fjacquet/licenses-exporter-core"
	"github.com/vmware/govmomi/vim25/types"
)

const vendor = "vmware"

// licensesToSamples maps vSphere LicenseManager entries to license samples.
// Unlimited licenses (Total <= 0) omit seats_total (absent-not-zero).
func licensesToSamples(instance string, infos []types.LicenseManagerLicenseInfo) []core.Sample {
	var out []core.Sample
	for _, info := range infos {
		unit := info.CostUnit
		if unit == "" {
			unit = "unit"
		}
		product := info.Name
		if info.Total > 0 {
			out = append(out, core.SeatSample(core.MetricSeatsTotal, vendor, product, unit, instance, float64(info.Total)))
		}
		out = append(out, core.SeatSample(core.MetricSeatsUsed, vendor, product, unit, instance, float64(info.Used)))
		if exp, ok := expiration(info.Properties); ok {
			out = append(out, core.ExpirationSample(vendor, product, instance, float64(exp.Unix())))
		}
	}
	return out
}

// expiration extracts the expirationDate property; absent for perpetual licenses.
func expiration(props []types.KeyAnyValue) (time.Time, bool) {
	for _, p := range props {
		if p.Key != "expirationDate" {
			continue
		}
		if t, ok := p.Value.(time.Time); ok {
			return t, true
		}
	}
	return time.Time{}, false
}
```

- [ ] **Step 4: Create `internal/vmware/source.go`** (rewired from `main:internal/vmware/source.go` — `license`→`core`)

```go
package vmware

import (
	"context"
	"fmt"
	"net/url"
	"time"

	core "github.com/fjacquet/licenses-exporter-core"
	"github.com/sirupsen/logrus"
	"github.com/vmware/govmomi"
	vlicense "github.com/vmware/govmomi/license"
	"github.com/vmware/govmomi/vim25/soap"
)

type source struct {
	instance string
	host     string
	username string
	password string
	insecure bool
}

func (s *source) Vendor() string   { return vendor }
func (s *source) Instance() string { return s.instance }

// Collect logs in fresh, lists licenses, and logs out — stateless per cycle.
// Logout uses a fresh background context (so it runs even if ctx was canceled
// mid-cycle) BOUNDED by a timeout so a stalled TCP can never block the deferred
// call indefinitely; a logout failure is logged so operators have visibility
// into potential vCenter session leaks.
func (s *source) Collect(ctx context.Context) ([]core.Sample, error) {
	u, err := soap.ParseURL(s.host)
	if err != nil {
		return nil, fmt.Errorf("parse vcenter url: %w", err)
	}
	u.User = url.UserPassword(s.username, s.password)

	c, err := govmomi.NewClient(ctx, u, s.insecure)
	if err != nil {
		return nil, fmt.Errorf("vcenter login: %w", err)
	}
	defer func() {
		logoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := c.Logout(logoutCtx); err != nil {
			logrus.WithFields(logrus.Fields{"vendor": vendor, "instance": s.instance}).WithError(err).Warn("vcenter logout failed")
		}
	}()

	infos, err := vlicense.NewManager(c.Client).List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list licenses: %w", err)
	}
	return licensesToSamples(s.instance, infos), nil
}
```

- [ ] **Step 5: Create `internal/vmware/vmware.go`** (rewired from `main:internal/vmware/vmware.go` — `config`/`license`→`core` + local `VMwareConfig`)

```go
package vmware

import (
	"fmt"

	core "github.com/fjacquet/licenses-exporter-core"
)

// NewSources builds one stateless Source per configured vCenter.
func NewSources(cfg VMwareConfig) ([]core.Source, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	var out []core.Source
	for _, v := range cfg.VCenters {
		pw, err := core.ResolveSecret(v.Password, v.PasswordFile)
		if err != nil {
			return nil, fmt.Errorf("vcenter %q: %w", v.Instance, err)
		}
		out = append(out, &source{
			instance: v.Instance,
			host:     v.Host,
			username: v.Username,
			password: pw,
			insecure: v.InsecureSkipVerify,
		})
	}
	return out, nil
}
```

- [ ] **Step 6: Copy the oracle tests UNCHANGED**

```bash
cd ~/Projects/vmware_licenses_exporter
git -C /Users/fjacquet/Projects/licenses_exporter show main:internal/vmware/parse_test.go  > internal/vmware/parse_test.go
git -C /Users/fjacquet/Projects/licenses_exporter show main:internal/vmware/source_test.go > internal/vmware/source_test.go
```
Do NOT edit these files. They import only `testing` + govmomi packages (`vim25/types`, `vim25/xml`, `simulator`) and assert on `s.Name`/`s.Value`/`s.Labels` (fields shared by the old `license.Sample` and new `core.Sample`), so they compile and pass against the rewired package unchanged.

- [ ] **Step 7: Tidy, build, and run the oracle**

```bash
GOFLAGS=-mod=mod go mod tidy
go build ./...
go test ./internal/vmware/... -race -v
```
Expected: `go build ./...` succeeds (no binary yet — no `package main`). `go test ./internal/vmware/...` PASS — `TestLimitedLicenseEmitsTotalUsedExpiration`, `TestExpirationDateXMLDecodeEndToEnd`, `TestUnlimitedLicenseOmitsTotal`, and `TestCollectAgainstVcsim` all green. `go.mod` has `licenses-exporter-core v0.1.0` + `govmomi` + `logrus`.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat: VMware LicenseManager collector on licenses-exporter-core v0.1.0" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XksHzPMuvUvvkbPgLWXTN9"
```

---

## Task 2: Thin main.go + consumer Config + config.yaml

Add the binary: a cobra wrapper delegating to `core.Main`, the consumer `Config`, and a vmware-only `config.yaml`. Default port **9106**.

**Files:**
- Create: `config.go`, `main.go`, `main_test.go`, `config.yaml`

**Interfaces:**
- Consumes from core v0.1.0: `core.Base` (`Validate() error`; embeds `Collection.Interval time.Duration` + `OTLP.{Endpoint,Insecure}`, yaml-inline); `core.LoadYAML(path string, into any) error`; `core.Main(core.App) error`; `core.App{Version, Addr string; Once, Debug, Trace bool; ConfigPath string; Load func() (core.Base, []core.Source, error)}`; `core.Source`.
- Consumes from Task 1: `vmware.NewSources(vmware.VMwareConfig) ([]core.Source, error)`, `vmware.VMwareConfig`.
- Produces for Task 3: binary `bin/vmware_licenses_exporter`; flags `--config` (`config.yaml`), `--web.listen-address` (`:9106`), `--debug`, `--once`, `--trace`.

- [ ] **Step 1: Write the failing test** `main_test.go`

```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	core "github.com/fjacquet/licenses-exporter-core"
)

// TestLoadConfigParsesBaseAndVMware proves the consumer Config wires core.Base
// (collection/otlp) AND the vendor vmware block from one YAML file.
func TestLoadConfigParsesBaseAndVMware(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `
collection:
  interval: 3h
otlp:
  endpoint: "otel:4317"
  insecure: true
vmware:
  enabled: true
  vcenters:
    - instance: dc-a
      host: https://vcenter-a.example.com
      username: svc-ro
      password: shhh
      insecureSkipVerify: true
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	var cfg Config
	if err := core.LoadYAML(path, &cfg); err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if cfg.Collection.Interval.Hours() != 3 {
		t.Errorf("interval = %v, want 3h", cfg.Collection.Interval)
	}
	if cfg.OTLP.Endpoint != "otel:4317" {
		t.Errorf("otlp endpoint = %q, want otel:4317", cfg.OTLP.Endpoint)
	}
	if !cfg.VMware.Enabled || len(cfg.VMware.VCenters) != 1 || cfg.VMware.VCenters[0].Instance != "dc-a" {
		t.Errorf("vmware block not parsed: %+v", cfg.VMware)
	}
}

// TestLoadReturnsSourcesForEnabledVCenter proves the App.Load closure builds a
// core.Source per enabled vCenter (the wiring core.Main will drive).
func TestLoadReturnsSourcesForEnabledVCenter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `
collection:
  interval: 2h
vmware:
  enabled: true
  vcenters:
    - instance: dc-a
      host: https://vcenter-a.example.com
      username: svc-ro
      password: shhh
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	base, sources, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if base.Collection.Interval.Hours() != 2 {
		t.Errorf("interval = %v, want 2h", base.Collection.Interval)
	}
	if len(sources) != 1 {
		t.Fatalf("got %d sources, want 1", len(sources))
	}
	if sources[0].Vendor() != "vmware" || sources[0].Instance() != "dc-a" {
		t.Errorf("source identity = %s/%s, want vmware/dc-a", sources[0].Vendor(), sources[0].Instance())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test . -run TestLoad -v`
Expected: FAIL — `undefined: Config`, `undefined: loadConfig`.

- [ ] **Step 3: Create `config.go`**

```go
package main

import (
	core "github.com/fjacquet/licenses-exporter-core"
	"github.com/fjacquet/vmware_licenses_exporter/internal/vmware"
)

// Config is the exporter's full config: the shared core.Base (collection + otlp)
// inline, plus the vendor-specific vmware block.
type Config struct {
	core.Base `yaml:",inline"`
	VMware    vmware.VMwareConfig `yaml:"vmware"`
}

// loadConfig parses the file and builds the sources — the single closure body
// core.Main calls at startup and on every reload.
func loadConfig(path string) (core.Base, []core.Source, error) {
	var cfg Config
	if err := core.LoadYAML(path, &cfg); err != nil {
		return core.Base{}, nil, err
	}
	if err := cfg.Validate(); err != nil {
		return core.Base{}, nil, err
	}
	sources, err := vmware.NewSources(cfg.VMware)
	if err != nil {
		return core.Base{}, nil, err
	}
	return cfg.Base, sources, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test . -run TestLoad -v`
Expected: PASS (both).

- [ ] **Step 5: Create `main.go`** (default addr `:9106`)

```go
package main

import (
	core "github.com/fjacquet/licenses-exporter-core"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// version is injected via -ldflags at build time (see Makefile `make cli`).
var version = "dev"

func main() {
	var (
		cfgPath string
		addr    string
		debug   bool
		once    bool
		trace   bool
	)
	root := &cobra.Command{
		Use:   "vmware_licenses_exporter",
		Short: "VMware vSphere license Prometheus + OTLP exporter",
		RunE: func(_ *cobra.Command, _ []string) error {
			return core.Main(core.App{
				Version:    version,
				Addr:       addr,
				Once:       once,
				Debug:      debug,
				Trace:      trace,
				ConfigPath: cfgPath,
				Load:       func() (core.Base, []core.Source, error) { return loadConfig(cfgPath) },
			})
		},
	}
	root.Flags().StringVar(&cfgPath, "config", "config.yaml", "path to config.yaml")
	root.Flags().StringVar(&addr, "web.listen-address", ":9106", "metrics listen address")
	root.Flags().BoolVar(&debug, "debug", false, "debug logging")
	root.Flags().BoolVar(&once, "once", false, "run one collection cycle and exit")
	root.Flags().BoolVar(&trace, "trace", false, "log repo-owned API responses (SDK tracing intentionally disabled)")
	if err := root.Execute(); err != nil {
		logrus.WithError(err).Fatal("exporter failed")
	}
}
```

- [ ] **Step 6: Create `config.yaml`**

```yaml
# vmware_licenses_exporter — VMware vSphere license exporter.
# Secrets are ${ENV} refs (expanded at load) or passwordFile paths.

collection:
  interval: 2h            # how often to poll vCenter LicenseManager

otlp:
  endpoint: ""            # empty disables OTLP; e.g. "otel-collector:4317"
  insecure: false

vmware:
  enabled: true
  vcenters:
    - instance: primary
      host: ${VC_HOST}              # e.g. https://vcenter.example.com
      username: ${VC_USERNAME}
      password: ${VC_PASSWORD}
      # passwordFile: /run/secrets/vc_password  # alternative to password
      insecureSkipVerify: false
```

- [ ] **Step 7: Build the binary and smoke it**

```bash
go build -ldflags="-s -w -X main.version=dev" -o bin/vmware_licenses_exporter .
./bin/vmware_licenses_exporter --help
printf 'collection:\n  interval: 2h\nvmware:\n  enabled: false\n' > /tmp/vmw-smoke.yaml
./bin/vmware_licenses_exporter --once --config /tmp/vmw-smoke.yaml
```
Expected: `--help` lists the five flags with `Use: vmware_licenses_exporter` and `--web.listen-address` default `:9106`. The `--once` run with `enabled: false` exits 0 (zero sources), prints nothing, no panic, no secret in output.

- [ ] **Step 8: Full suite + commit**

```bash
go test ./... -race
git add -A
git commit -m "feat: thin main delegating to core.Main; vmware-only config.yaml (:9106)" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XksHzPMuvUvvkbPgLWXTN9"
```
Expected: `go test ./...` PASS (vmware oracle + the two main tests).

---

## Task 3: Family scaffolding + docs + ADR-0001 + CHANGELOG

Add the family build/release/deploy scaffolding and docs, adapted from the m365 template (`m365`→`vmware`, `9105`→`9106`), and record the architecture ADR. Family-conformance + docs gate.

**Files:**
- Create (from `feat/m365-conversion` template, renamed): `Makefile`, `.goreleaser.yaml`, `Dockerfile`, `Dockerfile.goreleaser`, `docker-compose.yml`, `docker-compose.ghcr.yml`, `.golangci.yml`, `.gitignore` (finalize), `LICENSE`, `.github/**`, `mkdocs.yml`, `prometheus.yml`, `grafana/**`
- Create (new/adapted): `README.md`, `docs/metrics.md`, `docs/dashboards.md`, `docs/deployment/docker.md`, `docs/adr/index.md`, `docs/adr/0001-consume-core-retain-govmomi.md`, `CHANGELOG.md`

- [ ] **Step 1: Copy the build/release scaffolding, renamed**

For each file below, read the m365 version and write it into the vmware repo with `m365_licenses_exporter`→`vmware_licenses_exporter` (and any `m365`→`vmware`, `9105`→`9106`) substituted:
```
Makefile  .goreleaser.yaml  Dockerfile  Dockerfile.goreleaser
docker-compose.yml  docker-compose.ghcr.yml  .golangci.yml  LICENSE
mkdocs.yml  prometheus.yml
grafana/provisioning/datasources/datasource.yml
grafana/provisioning/dashboards/dashboards.yml
grafana/dashboards/licenses-overview.json
.github/workflows/*.yml
```
Source: `git -C /Users/fjacquet/Projects/licenses_exporter show feat/m365-conversion:<path>`. In `Makefile` set `BIN = vmware_licenses_exporter` and keep `release-snapshot` capped at `--parallelism 1` (the m365 template already has it). In `.golangci.yml`, keep the `main.go`/`_test.go` exclusions; **no G304 block** (secret/config file reads live in core). The four ratified `# nosemgrep` CI-caller lines stay verbatim.

> Contingency (do not pre-suppress): if `make ci` in Step 6 flags gosec (e.g. G402 on the govmomi insecure path), add a path-scoped exclusion in `.golangci.yml` for `internal/vmware/source.go` with a one-line comment — NOT an inline `//nolint`. Expect it NOT to fire (the unified repo's `make ci` passed with this exact code).

- [ ] **Step 2: `docs/metrics.md`** (VMware-flavoured, same 7 metric names)

Write `docs/metrics.md` documenting the generic `license_` schema with VMware examples, e.g.:
```
license_seats_total{vendor="vmware",product="vSphere 8 Enterprise Plus",unit="CPUs",instance="dc-a"} 32
license_seats_used{vendor="vmware",product="vSphere 8 Enterprise Plus",unit="CPUs",instance="dc-a"} 24
license_expiration_timestamp_seconds{vendor="vmware",product="vSphere 8 Enterprise Plus",instance="dc-a"} 1.8039456e+09
```
Include the exact seven metric names: `license_seats_total`, `license_seats_used`, `license_expiration_timestamp_seconds`, `license_up`, `license_collector_last_success_timestamp_seconds`, `license_scrape_duration_seconds`, `license_build_info`. Note the unlimited-key rule (`Total<=0` → seats_total omitted) and perpetual-license rule (expiration absent).

- [ ] **Step 3: `README.md`, `docs/dashboards.md`, `docs/deployment/docker.md`**

`README.md`: title `# vmware_licenses_exporter`; a VMware vSphere license exporter built on `github.com/fjacquet/licenses-exporter-core`; polls vCenter `LicenseManager` per configured vCenter; run/compose instructions (binary `vmware_licenses_exporter`, port `9106`); the vCenter read-only role note (a read-only account with the `Global > Licenses` / `Sessions` privileges needed to `List` licenses and log in/out); the family note + ADR-0001 link. `docs/dashboards.md` + `docs/deployment/docker.md`: adapt from the m365 template — vmware examples, port `9106`, vCenter env vars (`VC_HOST`/`VC_USERNAME`/`VC_PASSWORD`); no m365/tenant prose.

- [ ] **Step 4: ADR-0001 + index**

Create `docs/adr/0001-consume-core-retain-govmomi.md`:
```markdown
# 1. Consume licenses-exporter-core and retain govmomi

Date: 2026-07-02

## Status
Accepted

## Context
This exporter is the VMware sibling in the licenses_exporter family. The
vendor-neutral engine (schema, snapshot store, collection loop, dual export,
hot-reload server) lives in the shared library
`github.com/fjacquet/licenses-exporter-core`. For vSphere access, govmomi is the
official, mature Go SDK: it models the `LicenseManager` view and the license
capacity/usage/expiration fields we export, with session-based login.

## Decision
Depend on `licenses-exporter-core`; build every sample through its constructors.
`main.go` delegates the whole lifecycle to `core.Main`. Retain govmomi as the
vSphere client — it is both available and useful per the family client rule, so no
hand-rolled resty client and no "SDK-not-useful" ADR is warranted. Authenticate
stateless per collection cycle (login → LicenseManager.List → logout), with no
persisted session.

## Consequences
- Schema identity is guaranteed by construction — no local `license_` metric code.
- Engine bugfixes/features arrive via a core version bump, not a local edit.
- govmomi's dependency weight is accepted for its correctness and coverage.
- Startup is fatal on an unbuildable-but-valid config (core behaviour); see the core CHANGELOG.
```
Create/adapt `docs/adr/index.md` listing ADR-0001.

- [ ] **Step 5: `CHANGELOG.md`**

```markdown
# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release: a VMware vSphere license exporter (vCenter `LicenseManager` via govmomi)
  built on `github.com/fjacquet/licenses-exporter-core`. Emits the shared `license_` schema
  (`vendor="vmware"`), so it shares one Prometheus / Grafana view with the other family
  exporters. Default metrics port `9106`. See ADR-0001.
```

- [ ] **Step 6: Run the full gate**

```bash
cd ~/Projects/vmware_licenses_exporter
make ci
make release-snapshot
uvx --with mkdocs-material --with pymdown-extensions mkdocs build --strict
uvx semgrep scan --config auto --skip-unknown-extensions .
```
Expected: `make ci` green (gofmt, vet, golangci-lint 0 issues, `-race` tests pass, govulncheck clean, binary builds as `bin/vmware_licenses_exporter`). `make release-snapshot` produces archives/SBOM/cask named `vmware_licenses_exporter`. mkdocs `--strict` no warnings. semgrep 0 findings.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "chore: family scaffolding + docs + ADR-0001 (vmware_licenses_exporter, :9106)" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XksHzPMuvUvvkbPgLWXTN9"
```

---

## Task 4: Promote core to v1.0.0 + first release (ops — after the whole-branch review)

Not a TDD task. Cross-repo. Run after the vmware branch is green and reviewed. Confirm all version numbers with the user before pushing anything outward-facing.

- [ ] **Step 1: Promote licenses-exporter-core to v1.0.0**

In `~/Projects/licenses-exporter-core` (on `main`, at the published v0.1.0 HEAD): update the README "Versioning" section to state v1.0.0 (second consumer — vmware — compiled against it unchanged) and add a `## [1.0.0]` CHANGELOG entry. Commit, then:
```bash
cd ~/Projects/licenses-exporter-core
git tag -a v1.0.0 -m "v1.0.0 — API validated by a second independent consumer (vmware_licenses_exporter)"
git push origin main && git push origin v1.0.0
```

- [ ] **Step 2: Bump vmware to core v1.0.0**

```bash
cd ~/Projects/vmware_licenses_exporter
go get github.com/fjacquet/licenses-exporter-core@v1.0.0
GOFLAGS=-mod=mod go mod tidy
make ci
git commit -am "build: require licenses-exporter-core v1.0.0" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XksHzPMuvUvvkbPgLWXTN9"
```
Expected: `make ci` green against core v1.0.0.

- [ ] **Step 3: Publish the vmware repo + release** (confirm visibility + version with the user)

Set `mkdocs.yml` `extra.version` to the chosen tag. Then:
```bash
cd ~/Projects/vmware_licenses_exporter
gh repo create fjacquet/vmware_licenses_exporter --public --source=. --remote=origin \
  --description "VMware vSphere license exporter (Prometheus + OTLP) — licenses_exporter family, built on licenses-exporter-core"
git push -u origin main
git tag -a <version> -m "vmware_licenses_exporter <version>"
git push origin <version>
```
Verify `gh release view` shows the GoReleaser assets named `vmware_licenses_exporter`.

---

## Self-Review

**1. Spec coverage:**
- Consume core v0.1.0, keep govmomi, VMware-only → Task 1. ✅
- Thin main via `core.Main` + `Config` embedding `core.Base`, port 9106 → Task 2. ✅
- Metrics parity (seats_total omit on Total<=0, expiration absent for perpetual, unit=CostUnit) → Task 1 parse.go + oracle tests. ✅
- Auth (govmomi stateless per cycle, bounded logout, logout-warn, secrets via ResolveSecret) → Task 1 source.go/vmware.go. ✅
- Correctness oracle (parse_test + source_test/vcsim pass unchanged) → Task 1 Step 6-7. ✅
- New public repo bootstrap from m365 template → Task 1 Step 1 + Task 3. ✅
- ADR (consume core + retain govmomi) + docs/metrics.md + CHANGELOG → Task 3. ✅
- Core v1.0.0 promotion (build on v0.1.0, then promote + bump) → Task 4. ✅
- Family CI/GoReleaser conformance → Task 3 Steps 1, 6. ✅

**2. Placeholder scan:** No "TBD"/"handle errors"/"similar to". Every code step shows complete code. The gosec contingency in Task 3 Step 1 is a concrete grep-and-scope instruction, and `<version>` in Task 4 is an explicit user-confirmed value, not a code placeholder.

**3. Type consistency:** `VMwareConfig`/`VCenterConfig` field names (`Enabled`, `VCenters`, `Instance`, `Host`, `Username`, `Password`, `PasswordFile`, `InsecureSkipVerify`) match between Task 1 (definition) and Task 2 (`loadConfig`, test YAML). `core.SeatSample(name, vendor, product, unit, instance, v)`, `core.ExpirationSample(vendor, product, instance, tsUnix)`, `core.MetricSeatsTotal/Used`, `core.ResolveSecret`, `core.Base`, `core.LoadYAML`, `core.Main`, `core.App`, `core.Source`, `core.Sample` match the published v0.1.0 surface. `loadConfig` signature `(path string) (core.Base, []core.Source, error)` matches `App.Load` and the test. `vendor = "vmware"` unchanged.
