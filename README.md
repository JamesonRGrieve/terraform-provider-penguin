<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
# terraform-provider-penguin

The official OpenTofu / Terraform provider for **[Penguin](https://github.com/JamesonRGrieve/penguin-panel)** —
a game-server control panel that runs each server as an isolated **Proxmox LXC**
container. Penguin is a soft-fork of [Pelican](https://pelican.dev) (itself the
successor to Pterodactyl); this provider manages a panel through its admin
**Application API** and works against any Pelican-lineage panel.

Registry address: **`zephyrex/penguin`**. Licensed **AGPL-3.0-or-later**.

> **Status: pre-release (v0.0.1).** Not yet published to the Terraform Registry.
> Build it locally and consume it through a `dev_overrides` block (see
> *Development* below) until the first release.

## Usage

```hcl
terraform {
  required_providers {
    penguin = {
      source  = "zephyrex/penguin"
      version = "~> 0.0.1"
    }
  }
}

provider "penguin" {
  endpoint  = "https://panel.example.com" # or PENGUIN_ENDPOINT
  api_token = var.penguin_api_token       # papp_… ; or PENGUIN_API_TOKEN
}

resource "penguin_node" "primary" {
  name   = "node-eu-1"
  fqdn   = "node-eu-1.example.com"
  memory = 16384
  disk   = 200000
  cpu    = 800
}
```

### Authentication

The Application API is **admin-only** and authenticated with an **application API
token** (prefix `papp_`), sent as `Authorization: Bearer <token>`. Create one in
the panel (Admin → API Keys, application scope) and grant it the ACL resources you
intend to manage. Provide it via the `api_token` argument or the
`PENGUIN_API_TOKEN` environment variable — inject it from your secret store at
apply time; never commit it.

| Provider argument | Env var | Purpose |
|---|---|---|
| `endpoint` | `PENGUIN_ENDPOINT` | Panel base URL (the `/api/application` prefix is added automatically) |
| `api_token` | `PENGUIN_API_TOKEN` | Application API token (`papp_…`) |
| `insecure` | `PENGUIN_INSECURE` | Skip TLS verification (default `false`) |

## Resources & data sources

Two layers, by design (see the Go standard's generic-first mandate):

**Generic (complete coverage by construction).** These address *any* Application
API path, so the provider covers the whole API — including endpoints without a
typed resource yet (webhooks, mounts, plugins, egg import, server actions):

| Type | Purpose |
|---|---|
| `penguin_api_resource` (resource) | Manage any object by collection path + JSON body |
| `penguin_api` (data source) | Read any path (object or list) as canonical JSON |

**Typed (ergonomic sugar).** Full schemas, import to zero-diff, self-healing reads:

| Resource | Data source | Notes |
|---|---|---|
| `penguin_node` | `penguin_node` | Wings daemon target |
| `penguin_user` | `penguin_user` | Panel user account |
| `penguin_role` | `penguin_role` | Admin role (name only via API) |
| `penguin_database_host` | `penguin_database_host` | MySQL/MariaDB host |
| `penguin_allocation` | `penguin_allocation` | Node IP:port (immutable → replace) |
| `penguin_server` | `penguin_server` | Game/app server |
| `penguin_server_database` | `penguin_server_database` | Per-server database (immutable → replace) |

`mount` and `webhook` do not have typed resources yet — use `penguin_api_resource`
for them (webhook example: `examples/resources/penguin_api_resource`). Note that
upstream `StoreMountRequest` currently accepts no fields, so a typed mount resource
is intentionally deferred until that is fixed upstream.

See `examples/` for runnable configuration and per-resource `import.sh`.

## Development

Requires Go 1.26+. The single gate is `make check`.

```sh
make check          # go mod tidy (no diff) + gofmt + go vet + go build + go test
make lint           # golangci-lint (skipped cleanly if not installed)
make docs           # regenerate docs/ via tfplugindocs (skipped if not installed)
make install        # build into the local plugin mirror for dev overrides
make install-hooks  # git config core.hooksPath .githooks
```

The client transport (`internal/client`) is framework-free and unit-tested in
isolation; the framework glue lives in `internal/provider`.

### Local testing with dev overrides

```hcl
# ~/.terraformrc
provider_installation {
  dev_overrides { "zephyrex/penguin" = "/home/you/.terraform.d/plugins/registry.terraform.io/zephyrex/penguin/0.1.0/linux_amd64" }
  direct {}
}
```

Run `make install`, then `tofu plan` in an example directory.

## Lineage

Penguin is a soft-fork of Pelican, which builds on the Pterodactyl ecosystem.
This provider targets the Pelican-lineage Application API; sibling providers for
the other panels (`zephyrex/pelican`, `zephyrex/pterodactyl`) are planned. Provider
design was informed by prior open-source Pterodactyl providers (see `NOTICE`); no
third-party source is incorporated.

## License

AGPL-3.0-or-later. See `LICENSE` and `NOTICE`.
