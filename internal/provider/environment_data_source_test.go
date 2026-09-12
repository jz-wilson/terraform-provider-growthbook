// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccEnvironmentDataSource reads the pre-seeded "production" environment
// off the fake server through the singular data source.
func TestAccEnvironmentDataSource(t *testing.T) {
	t.Setenv("TF_ACC", "1")

	server := newFakeEnvironmentServer()
	t.Cleanup(server.Close)
	t.Setenv("GROWTHBOOK_API_URL", server.URL)
	t.Setenv("GROWTHBOOK_API_KEY", "test-key")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "growthbook_environment" "prod" {
  id = "production"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.growthbook_environment.prod", "id", "production"),
					resource.TestCheckResourceAttr("data.growthbook_environment.prod", "description", "Production"),
					resource.TestCheckResourceAttr("data.growthbook_environment.prod", "toggle_on_list", "true"),
				),
			},
		},
	})
}

// TestAccEnvironmentsDataSource lists every environment off the fake server
// through the plural data source.
func TestAccEnvironmentsDataSource(t *testing.T) {
	t.Setenv("TF_ACC", "1")

	server := newFakeEnvironmentServer()
	t.Cleanup(server.Close)
	t.Setenv("GROWTHBOOK_API_URL", server.URL)
	t.Setenv("GROWTHBOOK_API_KEY", "test-key")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "growthbook_environments" "all" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.growthbook_environments.all", "environments.#", "1"),
					resource.TestCheckResourceAttr("data.growthbook_environments.all", "environments.0.id", "production"),
				),
			},
		},
	})
}
