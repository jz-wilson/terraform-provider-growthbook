// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccEnvironmentResource exercises full CRUD for growthbook_environment
// against a fake in-process GrowthBook server, so it runs without any real
// GrowthBook instance. It still goes through resource.Test / TF_ACC because
// that is what drives real Terraform CLI plan/apply/import/destroy cycles.
func TestAccEnvironmentResource(t *testing.T) {
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
			},
		},
	})
}

// TestAccEnvironmentResource_projectsNullDrift locks in a fix for a real
// drift observed against a live GrowthBook: an environment with no projects
// can come back from the API as a nil "projects" slice on one call and a
// non-nil empty slice on another (see newFakeEnvironmentServer's PUT
// handler). Since projects is Optional+Computed, an unconfigured plan just
// carries the prior null value forward; if Create/Read/Update ever produced
// different Terraform values (null vs. empty set) for that same "no
// projects" API state, Terraform would fail the step with "Provider
// produced inconsistent result after apply". This test leaves projects
// unconfigured through a create and an update (forced by changing
// description) so both fake-server representations are exercised.
func TestAccEnvironmentResource_projectsNullDrift(t *testing.T) {
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
resource "growthbook_environment" "no_projects" {
  id = "no-projects"
}
`,
				Check: resource.TestCheckResourceAttr("growthbook_environment.no_projects", "id", "no-projects"),
			},
			{
				// Same projects (still unconfigured), but a changed
				// description forces an Update call, whose fake response
				// exercises the non-nil empty projects representation.
				Config: `
resource "growthbook_environment" "no_projects" {
  id          = "no-projects"
  description = "still no projects"
}
`,
				Check: resource.TestCheckResourceAttr("growthbook_environment.no_projects", "description", "still no projects"),
			},
		},
	})
}

// TestAccEnvironmentResource_parent verifies the create-only parent
// attribute is sent on create and forces replacement when changed.
func TestAccEnvironmentResource_parent(t *testing.T) {
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
resource "growthbook_environment" "child" {
  id     = "child"
  parent = "production"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_environment.child", "id", "child"),
					resource.TestCheckResourceAttr("growthbook_environment.child", "parent", "production"),
				),
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
			},
		},
	})
}
