# SPDX-License-Identifier: AGPL-3.0-or-later
# The generic resource manages ANY Application API object by path — full API
# coverage without a typed resource. Here it manages a webhook (no typed
# resource exists yet). Create POSTs /api/application/webhooks; read/update/
# delete address webhooks/<id>.
resource "penguin_api_resource" "deploy_webhook" {
  path = "webhooks"
  body = jsonencode({
    name        = "deploys"
    description = "Notify on server lifecycle events"
    endpoint    = "https://hooks.example.com/penguin"
    scope       = "global"
    events      = ["server:created", "server:deleted"]
  })
}

output "webhook_id" {
  value = penguin_api_resource.deploy_webhook.id
}
