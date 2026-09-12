resource "growthbook_sdk_connection" "production" {
  name        = "Production Web"
  language    = "javascript"
  environment = "production"

  encrypt_payload              = false
  include_visual_experiments   = true
  include_draft_experiments    = false
  include_experiment_names     = true
  include_redirect_experiments = true
  include_rule_ids             = false
  hash_secure_attributes       = true
  remote_eval_enabled          = false
}
