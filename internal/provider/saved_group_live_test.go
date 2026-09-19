// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccSavedGroupResource_live exercises full CRUD against a real
// GrowthBook organization. It only runs when GROWTHBOOK_LIVE=1 (see
// testAccLivePreCheck). Saved groups are creatable on the GrowthBook free
// plan.
//
// A type="list" saved group's attribute_key must reference an attribute
// that already exists in the organization, or the API returns HTTP 400
// ("Unknown attributeKey"); a fresh CI GrowthBook instance has none, so this
// test creates a growthbook_attribute alongside the saved group and
// references its property.
func TestAccSavedGroupResource_live(t *testing.T) {
	testAccLivePreCheck(t)

	property := acctest.RandomWithPrefix("tf-acc-attr")
	name := acctest.RandomWithPrefix("tf-acc")
	updatedName := name + "-updated"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "growthbook_attribute" "test" {
  property = %q
  datatype = "string"
}

resource "growthbook_saved_group" "test" {
  name          = %q
  type          = "list"
  attribute_key = growthbook_attribute.test.property
  values        = ["user-1", "user-2"]
}
`, property, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "name", name),
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "type", "list"),
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "attribute_key", property),
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "values.#", "2"),
					resource.TestCheckResourceAttrSet("growthbook_saved_group.test", "id"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: fmt.Sprintf(`
resource "growthbook_attribute" "test" {
  property = %q
  datatype = "string"
}

resource "growthbook_saved_group" "test" {
  name          = %q
  type          = "list"
  attribute_key = growthbook_attribute.test.property
  values        = []
}
`, property, updatedName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "name", updatedName),
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "values.#", "0"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "growthbook_saved_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
