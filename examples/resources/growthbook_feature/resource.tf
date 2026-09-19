resource "growthbook_feature" "checkout_redesign" {
  id            = "checkout-redesign"
  value_type    = "boolean"
  default_value = "false"
  description   = "Enables the redesigned checkout flow."
  project       = growthbook_project.storefront.id

  tags = ["checkout", "growth"]

  environments = {
    production = {
      enabled = false
    }
    staging = {
      enabled = true
    }
  }

  # prerequisites is a set of feature IDs, each of which must evaluate to
  # true; it is unmanaged unless set here, and set to [] to explicitly
  # clear it through Terraform.
  prerequisites = [growthbook_feature.holiday_mode.id]

  # rules is unmanaged (left to the GrowthBook UI/other tooling) unless set
  # here; set it to [] to explicitly clear all rules through Terraform.
  rules = [
    {
      type             = "force"
      description      = "Always on for US traffic"
      condition        = jsonencode({ country = "US" })
      all_environments = true
      value            = "true"
      prerequisites = [
        {
          id        = growthbook_feature.holiday_mode.id
          condition = jsonencode({ value = false })
        },
      ]
    },
    {
      type           = "rollout"
      description    = "Gradual rollout to everyone else"
      coverage       = 0.25
      hash_attribute = "id"
      value          = "true"
      environments   = ["staging"]

      # schedule_rules is a simple time-based on/off schedule; it requires
      # GrowthBook Pro. Like rules[].prerequisites, it is unmanaged unless
      # set here, and set to [] to explicitly clear it.
      schedule_type = "schedule"
      schedule_rules = [
        { enabled = true, timestamp = "2026-06-01T00:00:00Z" },
        { enabled = false, timestamp = null },
      ]
    },
    {
      type          = "experiment-ref"
      description   = "Tie the flag to a running experiment"
      experiment_id = "exp_checkout_redesign"
      variations = [
        { variation_id = "0", value = "false" },
        { variation_id = "1", value = "true" },
      ]
    },
  ]
}
