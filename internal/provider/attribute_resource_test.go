// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccAttributeResource exercises full CRUD for growthbook_attribute
// against a fake in-process GrowthBook server: create, update in place,
// import, and a datatype change (datatype is not RequiresReplace in this
// provider's schema, matching the API's PUT accepting it).
func TestAccAttributeResource(t *testing.T) {
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
  property    = "plan_tier"
  datatype    = "string"
  description = "The customer's plan tier"
  projects    = ["proj_1"]
  tags        = ["billing"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_attribute.plan_tier", "property", "plan_tier"),
					resource.TestCheckResourceAttr("growthbook_attribute.plan_tier", "datatype", "string"),
					resource.TestCheckResourceAttr("growthbook_attribute.plan_tier", "description", "The customer's plan tier"),
					resource.TestCheckResourceAttr("growthbook_attribute.plan_tier", "projects.#", "1"),
					resource.TestCheckResourceAttr("growthbook_attribute.plan_tier", "tags.#", "1"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName: "growthbook_attribute.plan_tier",
				// growthbook_attribute has no separate "id" attribute
				// (property is the identifier), so the default
				// ImportState behavior of importing by the state's "id"
				// meta field has nothing to read; import by property
				// explicitly instead.
				ImportStateId:                        "plan_tier",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "property",
			},
			{
				// Update in place: description changes, and datatype changes
				// from "string" to "enum" (with the now-required "enum"
				// field set), neither of which forces replacement.
				Config: `
resource "growthbook_attribute" "plan_tier" {
  property    = "plan_tier"
  datatype    = "enum"
  enum        = "free,pro,enterprise"
  description = "Updated description"
  projects    = ["proj_1"]
  tags        = ["billing"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_attribute.plan_tier", "datatype", "enum"),
					resource.TestCheckResourceAttr("growthbook_attribute.plan_tier", "enum", "free,pro,enterprise"),
					resource.TestCheckResourceAttr("growthbook_attribute.plan_tier", "description", "Updated description"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccAttributeResource_enumRequiredValidation confirms the config
// validator catches datatype = "enum" with no enum set at plan time, before
// any request reaches the API.
func TestAccAttributeResource_enumRequiredValidation(t *testing.T) {
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
resource "growthbook_attribute" "bad" {
  property = "plan_tier"
  datatype = "enum"
}
`,
				ExpectError: regexp.MustCompile(`(?i)enum`),
			},
		},
	})
}

// TestAccAttributeResource_updateOmitsUnconfigured records the PUT body
// GrowthBook receives and asserts an update that only configures
// "description" sends no other option keys, so Update never clobbers fields
// the config left unconfigured.
func TestAccAttributeResource_updateOmitsUnconfigured(t *testing.T) {
	t.Setenv("TF_ACC", "1")

	var lastPutBody map[string]any
	server := newFakeAttributeServer(func(body []byte) {
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
resource "growthbook_attribute" "plan_tier" {
  property = "plan_tier"
  datatype = "string"
}
`,
			},
			{
				Config: `
resource "growthbook_attribute" "plan_tier" {
  property    = "plan_tier"
  datatype    = "string"
  description = "only this changed"
}
`,
			},
		},
	})

	if lastPutBody == nil {
		t.Fatal("no PUT request observed")
	}
	// archived/hashAttribute/projects/tags have no UseStateForUnknown plan
	// modifier, so an unconfigured value plans as unknown and is omitted
	// outright. description/enum/format do have the modifier (so an
	// explicit "" set through config round-trips instead of drifting to
	// null forever, matching project.go's identical description field) and
	// are resent with their last known value instead - harmless, since
	// that value is exactly what the server already has.
	for _, key := range []string{"archived", "hashAttribute", "projects", "tags"} {
		if _, ok := lastPutBody[key]; ok {
			t.Errorf("PUT body = %v, want no %q key when unconfigured", lastPutBody, key)
		}
	}
	if lastPutBody["description"] != "only this changed" {
		t.Errorf("PUT body description = %v, want %q", lastPutBody["description"], "only this changed")
	}
}
