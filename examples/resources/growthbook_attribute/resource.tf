resource "growthbook_attribute" "plan_tier" {
  property    = "plan_tier"
  datatype    = "enum"
  enum        = "free,pro,enterprise"
  description = "The customer's subscription plan tier"
  projects    = ["prj_marketing"]
  tags        = ["billing"]
}
