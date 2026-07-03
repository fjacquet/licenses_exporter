# CLAUDE.md

Guidance for working in `m365_licenses_exporter`. Design:
`docs/superpowers/specs/2026-07-02-licenses-exporter-core-design.md` +
[ADR-0010](docs/adr/0010-consume-licenses-exporter-core.md).

## Commands

- `make cli` — build `bin/m365_licenses_exporter`.
- `make test` / `make test-race` — tests.
- `make tools` — install pinned dev/CI tooling (golangci-lint, cyclonedx-gomod, govulncheck).
- `make ci` — gofmt check + vet + lint + race tests + govulncheck + build (the CI gate).
- `make release-snapshot` — local GoReleaser dry-run (binaries + archives + SBOM + checksums).
- Run: `./bin/m365_licenses_exporter --config config.yaml [--once] [--debug] [--trace]`. Secrets
  are `${ENV}` refs in `config.yaml` (or `clientSecretFile`). `--once --debug` dumps every
  collected sample (sorted, exposition style); `--trace` logs every **repo-owned** API
  response body for live payload validation (never SDK debug modes — they leak the token).
- Demo stack: `docker compose up` (exporter + Prometheus + Grafana, auto-provisioned; :9105).
- Docs: `uvx --with mkdocs-material --with pymdown-extensions mkdocs build --strict`.

## Gotchas

- **`make cli` builds the binary, not `make`.** Bare `make` = `make all`
  (`clean lint test build`): it deletes `bin/` and `build` is `go build ./...`
  (compile check, no artifact). Use `make cli` to produce `bin/m365_licenses_exporter`.
- **Never put a literal `${VAR}` in `config.yaml` — even in a comment.** `core.Expand`
  does strict `${UPPER}` substitution over the raw file *before* YAML parsing and fails on
  any unset var, so a `# … ${ENV} …` comment aborts startup with
  `unset environment variable "ENV"`. Describe env-ref syntax in prose, not literal `${}`.
- **`--trace` is a no-op for the M365 collector** (msgraph-sdk-go is non-injectable — no
  repo-owned transport). Use `--once --debug` to inspect collected samples.

## Architecture

This repo is a thin **consumer** of `github.com/fjacquet/licenses-exporter-core`, which owns
the vendor-neutral engine: schema, snapshot store, collection loop, dual export
(`/metrics` + OTLP push), and the hot-reload HTTP server. `main.go` delegates the whole
lifecycle to `core.Main`; the repo owns only `internal/m365` (the Microsoft Graph collector,
implementing core's `Source` interface — `Vendor()`, `Instance()`, `Collect(ctx) → []Sample`)
and the consumer `Config`. See
[ADR-0010](docs/adr/0010-consume-licenses-exporter-core.md). A source failure degrades to
`license_up{vendor,instance}=0`, never crashing the cycle (core behaviour). The VMware
collector and the in-repo engine that predate the core split are gone; see `CHANGELOG.md`.

## Conventions (load-bearing)

- **Generic schema, vendor labels.** One `license_` prefix (novel vs. the family's per-vendor
  prefix — see ADR); vendors are distinguished by `vendor,product,unit,instance` labels, built
  from the shared constructors in `licenses-exporter-core` — never re-implemented locally.
- **Raw facts, absent-never-zero.** Expose `license_seats_total`, `license_seats_used`, and
  `license_expiration_timestamp_seconds` (**omitted** when perpetual — no `9999` sentinel).
  No exporter-computed `days_to_expiration` or `compliance_status`; derive those in PromQL /
  alert rules. An unparseable value yields an **absent** sample, never a fake `0`. At
  **cold start** only `license_build_info` is emitted — no `license_up` or target series
  until each source's first collect resolves.
- **Label-key consistency.** A metric name carries one label-key set across all series (all
  vendors); guaranteed by construction via the shared core constructors.
- **Auth.** M365 = `azidentity` client-credentials (via the SDK), Graph app permission
  `Organization.Read.All`. Retry **excludes 4xx** (never retry auth failures, core policy).
  The Graph SDK is non-injectable, so `--trace` wraps only repo-owned transports — never
  enable SDK debug (leaks bearer token).
- **Reload is cancelable.** Each collection cycle runs under a cancelable context (core
  behaviour); SIGHUP cancels in-flight requests, re-validates config, respawns the loop, and
  keeps serving the last-good snapshot until the new one is ready (never blanks `/metrics`).
- **config.yaml is the way.** Collector toggling is `m365.enabled:`, not an env var;
  `${ENV}` refs expand in host/user/secret; `.env` is a convenience, never the source of truth.
- **Always update docs (`docs/metrics.md`) + `CHANGELOG.md`** in the same change as a feature.

## Adding a vendor collector

This repo is **M365-only by design** — it is one member of the `licenses_exporter` family.
Additional vendors get their own sibling repo (the published `vmware_licenses_exporter`
and `veeam_licenses_exporter`) consuming `github.com/fjacquet/licenses-exporter-core`,
not a package added here. Within
`internal/m365`, a new source still follows core's `Source` interface — `Vendor()`,
`Instance()`, `Collect(ctx) → []Sample` — with a tolerant `parse` stamping
`vendor,product,unit,instance`; document metrics in `docs/metrics.md` and add a
`CHANGELOG.md` entry for any collector change.

---

<!-- rtk-instructions v2 -->
# RTK (Rust Token Killer) - Token-Optimized Commands

## Golden Rule

**Always prefix commands with `rtk`**. If RTK has a dedicated filter, it uses it. If not, it passes through unchanged. This means RTK is always safe to use.

**Important**: Even in command chains with `&&`, use `rtk`:

```bash
# ❌ Wrong
git add . && git commit -m "msg" && git push

# ✅ Correct
rtk git add . && rtk git commit -m "msg" && rtk git push
```

## RTK Commands by Workflow

### Build & Compile (80-90% savings)

```bash
rtk cargo build         # Cargo build output
rtk cargo check         # Cargo check output
rtk cargo clippy        # Clippy warnings grouped by file (80%)
rtk tsc                 # TypeScript errors grouped by file/code (83%)
rtk lint                # ESLint/Biome violations grouped (84%)
rtk prettier --check    # Files needing format only (70%)
rtk next build          # Next.js build with route metrics (87%)
```

### Test (60-99% savings)

```bash
rtk cargo test          # Cargo test failures only (90%)
rtk go test             # Go test failures only (90%)
rtk jest                # Jest failures only (99.5%)
rtk vitest              # Vitest failures only (99.5%)
rtk playwright test     # Playwright failures only (94%)
rtk pytest              # Python test failures only (90%)
rtk rake test           # Ruby test failures only (90%)
rtk rspec               # RSpec test failures only (60%)
rtk test <cmd>          # Generic test wrapper - failures only
```

### Git (59-80% savings)

```bash
rtk git status          # Compact status
rtk git log             # Compact log (works with all git flags)
rtk git diff            # Compact diff (80%)
rtk git show            # Compact show (80%)
rtk git add             # Ultra-compact confirmations (59%)
rtk git commit          # Ultra-compact confirmations (59%)
rtk git push            # Ultra-compact confirmations
rtk git pull            # Ultra-compact confirmations
rtk git branch          # Compact branch list
rtk git fetch           # Compact fetch
rtk git stash           # Compact stash
rtk git worktree        # Compact worktree
```

Note: Git passthrough works for ALL subcommands, even those not explicitly listed.

### GitHub (26-87% savings)

```bash
rtk gh pr view <num>    # Compact PR view (87%)
rtk gh pr checks        # Compact PR checks (79%)
rtk gh run list         # Compact workflow runs (82%)
rtk gh issue list       # Compact issue list (80%)
rtk gh api              # Compact API responses (26%)
```

### JavaScript/TypeScript Tooling (70-90% savings)

```bash
rtk pnpm list           # Compact dependency tree (70%)
rtk pnpm outdated       # Compact outdated packages (80%)
rtk pnpm install        # Compact install output (90%)
rtk npm run <script>    # Compact npm script output
rtk npx <cmd>           # Compact npx command output
rtk prisma              # Prisma without ASCII art (88%)
```

### Files & Search (60-75% savings)

```bash
rtk ls <path>           # Tree format, compact (65%)
rtk read <file>         # Code reading with filtering (60%)
rtk grep <pattern>      # Search grouped by file (75%). Format flags (-c, -l, -L, -o, -Z) run raw.
rtk find <pattern>      # Find grouped by directory (70%)
```

### Analysis & Debug (70-90% savings)

```bash
rtk err <cmd>           # Filter errors only from any command
rtk log <file>          # Deduplicated logs with counts
rtk json <file>         # JSON structure without values
rtk deps                # Dependency overview
rtk env                 # Environment variables compact
rtk summary <cmd>       # Smart summary of command output
rtk diff                # Ultra-compact diffs
```

### Infrastructure (85% savings)

```bash
rtk docker ps           # Compact container list
rtk docker images       # Compact image list
rtk docker logs <c>     # Deduplicated logs
rtk kubectl get         # Compact resource list
rtk kubectl logs        # Deduplicated pod logs
```

### Network (65-70% savings)

```bash
rtk curl <url>          # Compact HTTP responses (70%)
rtk wget <url>          # Compact download output (65%)
```

### Meta Commands

```bash
rtk gain                # View token savings statistics
rtk gain --history      # View command history with savings
rtk discover            # Analyze Claude Code sessions for missed RTK usage
rtk proxy <cmd>         # Run command without filtering (for debugging)
rtk init                # Add RTK instructions to CLAUDE.md
rtk init --global       # Add RTK to ~/.claude/CLAUDE.md
```

## Token Savings Overview

| Category | Commands | Typical Savings |
|----------|----------|-----------------|
| Tests | vitest, playwright, cargo test | 90-99% |
| Build | next, tsc, lint, prettier | 70-87% |
| Git | status, log, diff, add, commit | 59-80% |
| GitHub | gh pr, gh run, gh issue | 26-87% |
| Package Managers | pnpm, npm, npx | 70-90% |
| Files | ls, read, grep, find | 60-75% |
| Infrastructure | docker, kubectl | 85% |
| Network | curl, wget | 65-70% |

Overall average: **60-90% token reduction** on common development operations.
<!-- /rtk-instructions -->
