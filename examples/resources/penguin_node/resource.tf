# SPDX-License-Identifier: AGPL-3.0-or-later
resource "penguin_node" "primary" {
  name   = "node-eu-1"
  fqdn   = "node-eu-1.example.com"
  scheme = "https"

  memory = 16384  # MiB
  disk   = 200000 # MiB
  cpu    = 800    # percent (8 cores)

  tags = ["eu", "prod"]
}
