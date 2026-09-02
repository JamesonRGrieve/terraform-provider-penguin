# SPDX-License-Identifier: AGPL-3.0-or-later
resource "penguin_server" "survival" {
  name       = "survival"
  user       = penguin_user.alice.id
  egg        = 3 # a Minecraft (Paper) egg id, from `data.penguin_api`
  allocation = penguin_allocation.game.id

  limit_memory = 4096  # MiB
  limit_disk   = 10240 # MiB
  limit_cpu    = 200   # percent (2 cores)

  feature_databases = 2
  feature_backups   = 5

  environment = {
    SERVER_JARFILE    = "server.jar"
    MINECRAFT_VERSION = "latest"
    BUILD_NUMBER      = "latest"
  }

  start_on_completion = true
}
