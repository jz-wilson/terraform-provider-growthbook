// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// testAccLivePreCheck additionally requires GROWTHBOOK_LIVE=1, opting these
// tests in to running against a real GrowthBook (started from
// e2e/docker-compose.yml in CI; see e2e/README.md). The instance backing
// these tests is on GrowthBook's free/unlicensed plan, which refuses to
// create custom environments (402 Payment Required). These tests therefore
// never create or delete an environment: they read the built-in
// "production" environment through the data sources, and exercise the
// resource's import/update path against "production" itself, reverting
// before the test ends so "production" is never left owned (and is never
// deleted) by Terraform state.
func testAccLivePreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("GROWTHBOOK_LIVE") != "1" {
		t.Skip("set GROWTHBOOK_LIVE=1 to run tests against a live GrowthBook instance")
	}
	testAccPreCheck(t)
}

// TestAccEnvironmentsDataSource_live confirms growthbook_environments sees
// the built-in "production" environment that every GrowthBook organization
// has.
func TestAccEnvironmentsDataSource_live(t *testing.T) {
	testAccLivePreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "growthbook_environments" "all" {}`,
				Check: resource.TestCheckTypeSetElemNestedAttrs(
					"data.growthbook_environments.all", "environments.*", map[string]string{
						"id": "production",
					},
				),
			},
		},
	})
}

// TestAccEnvironmentDataSource_live confirms growthbook_environment can read
// "production" by id.
func TestAccEnvironmentDataSource_live(t *testing.T) {
	testAccLivePreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "growthbook_environment" "production" {
  id = "production"
}
`,
				Check: resource.TestCheckResourceAttr("data.growthbook_environment.production", "id", "production"),
			},
		},
	})
}

// TestAccEnvironmentResource_live_production imports the real "production"
// environment, edits its description, reverts the description, then drops
// it from Terraform state with a config-driven `removed` block rather than
// deleting the resource block outright. A plain resource-block removal would
// plan a Destroy and call DeleteEnvironment against production; the
// `removed` block (Terraform 1.7+, this repo pins 1.16.2) with
// `lifecycle { destroy = false }` forgets the resource from state without
// touching the real environment, so the automatic end-of-test destroy that
// resource.Test would otherwise run has nothing left to act on.
func TestAccEnvironmentResource_live_production(t *testing.T) {
	testAccLivePreCheck(t)

	const importCfg = `
resource "growthbook_environment" "production" {
  id = "production"
}

import {
  to = growthbook_environment.production
  id = "production"
}
`

	const updateCfg = `
resource "growthbook_environment" "production" {
  id          = "production"
  description = "Managed by TestAccEnvironmentResource_live_production"
}
`

	const revertCfg = `
resource "growthbook_environment" "production" {
  id          = "production"
  description = "Production"
}
`

	const removedCfg = `
removed {
  from = growthbook_environment.production

  lifecycle {
    destroy = false
  }
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Import the pre-existing environment without creating or
				// changing anything.
				Config: importCfg,
				Check:  resource.TestCheckResourceAttr("growthbook_environment.production", "id", "production"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// Persist the imported state, then change the description.
				Config: updateCfg,
				Check:  resource.TestCheckResourceAttr("growthbook_environment.production", "description", "Managed by TestAccEnvironmentResource_live_production"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// Revert the description back to what "production" starts
				// with on a fresh GrowthBook instance.
				Config: revertCfg,
				Check:  resource.TestCheckResourceAttr("growthbook_environment.production", "description", "Production"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// Drop it from state without deleting it. See the doc
				// comment above for why this uses `removed` instead of
				// simply omitting the resource block.
				Config: removedCfg,
			},
		},
	})
}
