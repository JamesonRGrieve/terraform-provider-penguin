# terraform-provider-penguin

The official OpenTofu / Terraform provider for **Penguin** — the Proxmox-LXC
game-server control panel (a soft-fork of Pelican). It manages a panel through
its admin **Application API**. Standalone and public: no dependency on any private
infra. AGPL-3.0-or-later.

> Read the workspace `/home/jameson/Source/CLAUDE.md`, the user tenets, and the
> stack rules (SOP *Stack Standards → Go* “OpenTofu provider conventions”, SOP *Stack Standards → OpenTofu*) first.
> This file wins on repo specifics.

## Status

**v0.0.1 — pre-release. Do NOT publish/release yet.** No registry publish, no
release tag, no GoReleaser run until the operator says so. `main.go` `version`
defaults to `dev` and is overwritten by ldflags at (a future) release. The
`docs/index.md` landing page is authored here but is surfaced on the public site,
which lives in the **penguin-web** repo — do not edit penguin-web from here.

## Architecture — generic first, typed sugar on top

Per go.md §8, coverage is guaranteed by construction and typed resources are
sugar, never the only path:

- **`internal/client`** — framework-free transport. Bearer auth, the Fractal
  envelope (`{object, attributes}` / `{object:"list", data, meta.pagination}`),
  auto-pagination, typed `APIError` (+ `IsNotFound` for self-healing reads), and
  429/5xx/transport retries with backoff. No `terraform-plugin-framework` import,
  so it is unit-tested in isolation.
- **`internal/provider`** — framework glue.
  - `api_resource.go` / `api_data_source.go` — the **generic** `penguin_api_resource`
    and `penguin_api`. These address *any* `/api/application` path → 100% API
    coverage, including objects with no typed resource (webhooks, mounts, plugins,
    egg import, server actions).
  - Typed resources: `node`, `user`, `role`, `database_host`, `allocation`,
    `server`, `server_database`. Each is a paired `*_resource.go` / `*_data_source.go`
    that **shares one model struct + one `applyX` mapping** (the data source reuses
    the resource's mapping). CRUD plumbing is factored into `object.go`
    (`createObject`/`readObject`/`updateObject`/`deleteObject`, self-healing on 404);
    attribute conversions into `attr_helpers.go`; provider/id/env helpers into
    `helpers.go`.
- **`main.go`** — provider server entry. Address `registry.terraform.io/zephyrex/penguin`.

### Invariants (go.md §8 / tofu.md §14)

- Every stateful resource implements `ImportState` and self-heals: Read drops the
  resource from state (`RemoveResource`) on a 404 rather than erroring.
- Apply-twice → no-changes. Computed identity fields (`id`, `uuid`, `created_at`)
  use `UseStateForUnknown`; server-defaulted fields are `Optional+Computed+Default`
  so an omitted value never churns.
- **Write-only-not-readable fields are not read back**, so their config value is
  preserved and never shows drift: user `password`/`timezone`, database-host
  `password`/`node_ids`, server `environment` (avoids churn vs. egg-injected
  defaults — the data source exposes the full effective env via `readEnv=true`),
  server `skip_scripts`/`start_on_completion`.
- Immutable objects (`allocation`, `server_database` fields) use `RequiresReplace`.
- Secrets come only from provider config / env; nothing hard-coded, no plaintext
  in committed state.

### What is intentionally NOT typed yet

- `mount` — upstream `StoreMountRequest` currently declares no `rules()`, so a
  typed create would silently persist nothing. Deferred until fixed upstream; use
  `penguin_api_resource`.
- `webhook` — usable via `penguin_api_resource` today; typed sugar is a follow-up.
- `egg`, `plugin` — read/import/delete only upstream (no field CRUD); the generic
  data source / resource cover them.

## The fork seam — penguin → pelican → pterodactyl

This repo is the first of three sibling providers (`zephyrex/penguin`,
`zephyrex/pelican`, `zephyrex/pterodactyl`), one per panel flavour. Keep the
flavour-specific surface isolated so a fork is a small diff:

- **Identical across all three:** `internal/client` (same `/api/application` base
  path and Fractal envelope) and the generic `penguin_api_resource` / `penguin_api`.
- **Penguin ≈ Pelican today:** the Penguin Application API is currently
  byte-for-byte stock Pelican (the Docker→LXC changes live in Penguin Wings; the
  planned nullable `node.backend` enum is design-only, not yet in the API). So the
  `pelican` fork is near-identical — expect only branding/module-path changes.
- **Pterodactyl differs:** older API shape — it has `locations` and `nests`
  resources and an `api-keys` surface that Pelican folded away, and some field
  names differ. Those deltas live entirely in the typed `internal/provider/*`
  resources, not the client.

When forking: change the module path, provider `Address`/`TypeName`, `NOTICE`, and
only the typed resources that diverge. Do not re-fork the client or the generic
resources.

## Quality gate

`make check` = `go mod tidy` (no diff) + `gofmt` + `go vet` + `go build` +
`go test`. Mirrored in CI (`.github/workflows/ci.yml`) and the pre-commit hook
(`.githooks/pre-commit`; enable with `make install-hooks`). `golangci-lint`
(`make lint`) and `tfplugindocs` (`make docs`) skip cleanly when not installed.
Never `--no-verify`. Every Go/`.tf`/script file carries the AGPL SPDX header.
