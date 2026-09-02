# SPDX-License-Identifier: AGPL-3.0-or-later
resource "penguin_allocation" "game" {
  node_id = penguin_node.primary.id
  ip      = "10.0.0.10"
  port    = 25565
  alias   = "mc.example.com"
}
