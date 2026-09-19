# attribute_key must reference an attribute that already exists in the
# organization (see growthbook_attribute); a fresh organization with no
# attributes defined would need one created first.
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
