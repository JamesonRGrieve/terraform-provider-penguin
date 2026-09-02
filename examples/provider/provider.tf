# SPDX-License-Identifier: AGPL-3.0-or-later
terraform {
  required_providers {
    penguin = {
      source = "zephyrex/penguin"
      # Pre-release (v0.0.1), not yet published to the registry — consume via a
      # dev_overrides block in ~/.terraformrc until the first release.
      version = "~> 0.0.1"
    }
  }
}

provider "penguin" {
  # The panel base URL. May also be set via PENGUIN_ENDPOINT.
  endpoint = "https://panel.example.com"

  # An application API token (papp_...). May also be set via PENGUIN_API_TOKEN.
  # Inject it from your secret store at apply time; never commit it.
  api_token = var.penguin_api_token
}

variable "penguin_api_token" {
  type      = string
  sensitive = true
}
