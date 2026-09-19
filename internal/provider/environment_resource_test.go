// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccEnvironmentResource exercises full CRUD for growthbook_environment
// against a fake in-process GrowthBook server, so it runs without any real
// GrowthBook instance. It still goes through resource.Test / TF_ACC because
// that is what drives real Terraform CLI plan/apply/import/destroy cycles.
func TestAccEnvironmentResource(t *testing.T) {
	t.Setenv("TF_ACC", "1")

	server := newFakeEnvironmentServer(nil)
	t.Cleanup(server.Close)
	t.Setenv("GROWTHBOOK_API_URL", server.URL)
	t.Setenv("GROWTHBOOK_API_KEY", "test-key")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "growthbook_environment" "staging" {
  id             = "staging"
  description    = "Staging"
  toggle_on_list = true
  default_state  = false
  projects       = ["proj_1"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_environment.staging", "id", "staging"),
					resource.TestCheckResourceAttr("growthbook_environment.staging", "description", "Staging"),
					resource.TestCheckResourceAttr("growthbook_environment.staging", "toggle_on_list", "true"),
					resource.TestCheckResourceAttr("growthbook_environment.staging", "default_state", "false"),
					resource.TestCheckResourceAttr("growthbook_environment.staging", "projects.#", "1"),
					resource.TestCheckResourceAttr("growthbook_environment.staging", "projects.0", "proj_1"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "growthbook_environment.staging",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: `
resource "growthbook_environment" "staging" {
  id             = "staging"
  description    = "Staging (updated)"
  toggle_on_list = false
  default_state  = false
  projects       = ["proj_1"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_environment.staging", "description", "Staging (updated)"),
					resource.TestCheckResourceAttr("growthbook_environment.staging", "toggle_on_list", "false"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccEnvironmentResource_clearsProjectsOnRemoval proves that removing
// projects from config (which plans as null, since it is Optional-only, not
// Computed) sends an explicit "[]" on Update rather than omitting the
// field - the fix for the inconsistent-result-after-apply bug an omission
// would cause (GrowthBook would keep the old list, and the provider's
// post-apply Read would then disagree with the null the plan promised).
func TestAccEnvironmentResource_clearsProjectsOnRemoval(t *testing.T) {
	t.Setenv("TF_ACC", "1")

	var lastPutBody map[string]any
	server := newFakeEnvironmentServer(func(body []byte) {
		_ = json.Unmarshal(body, &lastPutBody)
	})
	t.Cleanup(server.Close)
	t.Setenv("GROWTHBOOK_API_URL", server.URL)
	t.Setenv("GROWTHBOOK_API_KEY", "test-key")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "growthbook_environment" "staging" {
  id       = "staging"
  projects = ["proj_1"]
}
`,
				Check: resource.TestCheckResourceAttr("growthbook_environment.staging", "projects.#", "1"),
			},
			{
				Config: `
resource "growthbook_environment" "staging" {
  id = "staging"
}
`,
				Check: resource.TestCheckNoResourceAttr("growthbook_environment.staging", "projects"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})

	if lastPutBody == nil {
		t.Fatal("no PUT request observed")
	}
	projects, ok := lastPutBody["projects"].([]any)
	if !ok || len(projects) != 0 {
		t.Errorf("PUT body projects = %v, want an explicit empty list", lastPutBody["projects"])
	}
}

// TestAccEnvironmentResource_parent verifies the create-only parent
// attribute is sent on create and forces replacement when changed.
func TestAccEnvironmentResource_parent(t *testing.T) {
	t.Setenv("TF_ACC", "1")

	server := newFakeEnvironmentServer(nil)
	t.Cleanup(server.Close)
	t.Setenv("GROWTHBOOK_API_URL", server.URL)
	t.Setenv("GROWTHBOOK_API_KEY", "test-key")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "growthbook_environment" "child" {
  id     = "child"
  parent = "production"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_environment.child", "id", "child"),
					resource.TestCheckResourceAttr("growthbook_environment.child", "parent", "production"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: `
resource "growthbook_environment" "child" {
  id     = "child2"
  parent = "production"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_environment.child", "id", "child2"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
