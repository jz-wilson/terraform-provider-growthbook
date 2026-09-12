data "growthbook_environment" "production" {
  id = "production"
}

output "production_toggle_on_list" {
  value = data.growthbook_environment.production.toggle_on_list
}
