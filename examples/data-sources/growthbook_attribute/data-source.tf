data "growthbook_attribute" "plan_tier" {
  property = "plan_tier"
}

output "plan_tier_datatype" {
  value = data.growthbook_attribute.plan_tier.datatype
}
