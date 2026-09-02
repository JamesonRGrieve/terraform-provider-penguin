---
page_title: "penguin Provider"
description: |-
  The Penguin provider manages a Penguin / Pelican game-server control panel
  through its admin Application API.
---

# penguin Provider

The Penguin provider manages a **[Penguin](https://github.com/JamesonRGrieve/penguin-panel)**
game-server control panel — and any Pelican-lineage panel — through its admin
**Application API**. Penguin runs each server as an isolated Proxmox LXC container;
it is a soft-fork of [Pelican](https://pelican.dev), which builds on the
Pterodactyl ecosystem.

The provider offers two layers:

- **Generic resources** (`penguin_api_resource`, `penguin_api`) that address any
  API path, giving complete coverage of the Application API by construction.
- **Typed resources** (`penguin_node`, `penguin_user`, `penguin_server`, …) that
  add ergonomic, fully-schematised sugar over the same endpoints.

## Example Usage

```terraform
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

## Authentication

The Application API is **admin-only** and authenticated with an **application API
token** (prefix `papp_`), sent as an `Authorization: Bearer` header. Create one in
the panel under *Admin → API Keys* (application scope) and grant it the ACL
resources you intend to manage. Supply it with the `api_token` argument or the
`PENGUIN_API_TOKEN` environment variable, injected from a secret store at apply
time — never commit it.

## Schema

### Optional

- `endpoint` (String) — Base URL of the panel, e.g. `https://panel.example.com`.
  The `/api/application` prefix is appended automatically. May be set via
  `PENGUIN_ENDPOINT`.
- `api_token` (String, Sensitive) — Application API token (`papp_…`). May be set
  via `PENGUIN_API_TOKEN`.
- `insecure` (Boolean) — Skip TLS certificate verification. May be set via
  `PENGUIN_INSECURE`. Defaults to `false`.

## Resources and data sources

| Resource | Data source | Description |
|---|---|---|
| `penguin_api_resource` | `penguin_api` | Generic — manage/read any API path |
| `penguin_node` | `penguin_node` | A Wings daemon target |
| `penguin_user` | `penguin_user` | A panel user account |
| `penguin_role` | `penguin_role` | An admin role |
| `penguin_database_host` | `penguin_database_host` | A MySQL/MariaDB host |
| `penguin_allocation` | `penguin_allocation` | A node IP:port allocation |
| `penguin_server` | `penguin_server` | A game/application server |
| `penguin_server_database` | `penguin_server_database` | A per-server database |

Objects without a typed resource yet (webhooks, mounts, plugins, egg import,
server power actions) are fully manageable through `penguin_api_resource`.
