# SPDX-License-Identifier: AGPL-3.0-or-later
data "penguin_server" "survival" {
  id = 12
}

output "server_status" {
  value = data.penguin_server.survival.status
}
