# SPDX-License-Identifier: AGPL-3.0-or-later
# The generic data source reads ANY Application API path and returns the raw
# response as canonical JSON. Combine with jsondecode() for full read coverage.
data "penguin_api" "eggs" {
  path = "eggs"
  query = {
    per_page = "100"
  }
}

# Find a specific egg id by name, e.g. to feed penguin_server.egg.
output "paper_egg_id" {
  value = one([
    for e in jsondecode(data.penguin_api.eggs.json).data :
    e.attributes.id if e.attributes.name == "Paper"
  ])
}
