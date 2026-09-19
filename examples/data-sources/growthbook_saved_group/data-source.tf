data "growthbook_saved_group" "beta_users" {
  id = "sg_abc123"
}

output "beta_users_values" {
  value = data.growthbook_saved_group.beta_users.values
}
