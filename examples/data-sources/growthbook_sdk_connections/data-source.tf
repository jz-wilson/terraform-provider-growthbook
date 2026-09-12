data "growthbook_sdk_connections" "all" {}

output "sdk_connection_names" {
  value = [for c in data.growthbook_sdk_connections.all.sdk_connections : c.name]
}
