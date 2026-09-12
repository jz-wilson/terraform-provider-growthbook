resource "growthbook_project" "example" {
  name            = "Example Project"
  description     = "Managed by Terraform"
  restrict_access = false

  settings = {
    stats_engine      = "bayesian"
    confidence_level  = 0.95
    p_value_threshold = 0.05
  }
}
