// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccAttributeDataSource confirms growthbook_attribute can read an
// existing attribute by property, against a fake in-process GrowthBook
// server.
func TestAccAttributeDataSource(t *testing.T) {
	t.Setenv("TF_ACC", "1")

	server := newFakeAttributeServer(nil)
	t.Cleanup(server.Close)
	t.Setenv("GROWTHBOOK_API_URL", server.URL)
	t.Setenv("GROWTHBOOK_API_KEY", "test-key")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "growthbook_attribute" "plan_tier" {
  property = "plan_tier"
  datatype = "string"
}

data "growthbook_attribute" "plan_tier" {
  property = growthbook_attribute.plan_tier.property
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.growthbook_attribute.plan_tier", "property", "plan_tier"),
					resource.TestCheckResourceAttr("data.growthbook_attribute.plan_tier", "datatype", "string"),
				),
			},
		},
	})
}

// TestAccAttributeDataSource_notFound confirms a missing property surfaces a
// clean error instead of a panic or an empty result.
func TestAccAttributeDataSource_notFound(t *testing.T) {
	t.Setenv("TF_ACC", "1")

	server := newFakeAttributeServer(nil)
	t.Cleanup(server.Close)
	t.Setenv("GROWTHBOOK_API_URL", server.URL)
	t.Setenv("GROWTHBOOK_API_KEY", "test-key")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "growthbook_attribute" "missing" {
  property = "does_not_exist"
}
`,
				ExpectError: regexp.MustCompile(`(?i)not found`),
			},
		},
	})
}
