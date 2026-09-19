resource "growthbook_saved_group" "beta_users" {
  name          = "Beta Users"
  type          = "list"
  attribute_key = "userId"
  values        = ["user-123", "user-456"]
}

resource "growthbook_saved_group" "us_visitors" {
  name      = "US Visitors"
  type      = "condition"
  condition = jsonencode({ country = "US" })
}
