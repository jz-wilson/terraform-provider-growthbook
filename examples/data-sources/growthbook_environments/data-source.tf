data "growthbook_environments" "all" {}

output "environment_ids" {
  value = [for e in data.growthbook_environments.all.environments : e.id]
}
