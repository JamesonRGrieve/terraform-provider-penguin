# SPDX-License-Identifier: AGPL-3.0-or-later
resource "penguin_database_host" "mariadb" {
  name     = "mariadb-eu"
  host     = "10.0.0.20"
  port     = 3306
  username = "pelican"
  password = var.db_host_password

  node_ids = [penguin_node.primary.id]
}

variable "db_host_password" {
  type      = string
  sensitive = true
}
