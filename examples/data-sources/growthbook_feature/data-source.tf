data "growthbook_feature" "checkout_redesign" {
  id = "checkout-redesign"
}

output "checkout_redesign_default_value" {
  value = data.growthbook_feature.checkout_redesign.default_value
}
