# SPDX-License-Identifier: AGPL-3.0-or-later
resource "penguin_server_database" "world" {
  server_id = penguin_server.survival.id
  host_id   = penguin_database_host.mariadb.id
  database  = "world"
  remote    = "%"
}

# The generated credentials are available as computed attributes:
#   penguin_server_database.world.username
#   penguin_server_database.world.password  (sensitive)
