data "growthbook_sdk_connection" "production" {
  id = "sdk_abc123"
}

output "sdk_client_key" {
  value     = data.growthbook_sdk_connection.production.key
  sensitive = true
}
