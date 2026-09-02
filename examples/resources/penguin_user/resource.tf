# SPDX-License-Identifier: AGPL-3.0-or-later
resource "penguin_user" "alice" {
  email    = "alice@example.com"
  username = "alice"
  password = var.alice_password # injected from a secret store
  language = "en"
}

variable "alice_password" {
  type      = string
  sensitive = true
}
