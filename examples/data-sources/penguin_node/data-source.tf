# SPDX-License-Identifier: AGPL-3.0-or-later
data "penguin_node" "primary" {
  id = 5
}

output "node_fqdn" {
  value = data.penguin_node.primary.fqdn
}
