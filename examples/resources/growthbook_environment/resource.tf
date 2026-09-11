resource "growthbook_environment" "staging" {
  id             = "staging"
  description    = "Staging"
  toggle_on_list = true
  default_state  = false
  projects       = ["prj_marketing"]
}

# An environment cloned from an existing one at creation time. "parent" is
# create-only: changing it forces a new resource.
resource "growthbook_environment" "staging_eu" {
  id     = "staging-eu"
  parent = growthbook_environment.staging.id
}
