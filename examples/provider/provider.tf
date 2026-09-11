terraform {
  required_providers {
    growthbook = {
      source = "jz-wilson/growthbook"
    }
  }
}

provider "growthbook" {
  # Both can also come from GROWTHBOOK_API_KEY / GROWTHBOOK_API_URL.
  api_key = var.growthbook_api_key
  api_url = "https://growthbook.example.com/api"
}
