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

// TestAccAttributeResource_live exercises full CRUD against a real
// GrowthBook organization. It only runs when GROWTHBOOK_LIVE=1. Attributes
// are creatable on the GrowthBook free plan, so this does not require a
// paid organization.
func TestAccAttributeResource_live(t *testing.T) {
	testAccLivePreCheck(t)

	property := acctest.RandomWithPrefix("tf-acc-attr") // e.g. tf-acc-attr-123456789

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "growthbook_attribute" "test" {
  property    = %q
  datatype    = "string"
  description = "created by acceptance test"
}
`, property),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_attribute.test", "property", property),
					resource.TestCheckResourceAttr("growthbook_attribute.test", "datatype", "string"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: fmt.Sprintf(`
resource "growthbook_attribute" "test" {
  property    = %q
  datatype    = "enum"
  enum        = "a,b,c"
  description = "updated by acceptance test"
}
`, property),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_attribute.test", "datatype", "enum"),
					resource.TestCheckResourceAttr("growthbook_attribute.test", "enum", "a,b,c"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// Configure tags, then remove them in the next step to
				// prove GrowthBook's real clear semantics: a null plan
				// value (tags is Optional-only, not Computed) must send an
				// explicit "[]", not omit the field, or the attribute would
				// keep its old tags and drift forever. projects follows the
				// identical code path in attributeRequestFromModel, so this
				// covers both without needing a second real project id to
				// exist on this organization.
				Config: fmt.Sprintf(`
resource "growthbook_attribute" "test" {
  property    = %q
  datatype    = "enum"
  enum        = "a,b,c"
  description = "updated by acceptance test"
  tags        = ["tf-acc"]
}
`, property),
				Check: resource.TestCheckResourceAttr("growthbook_attribute.test", "tags.#", "1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: fmt.Sprintf(`
resource "growthbook_attribute" "test" {
  property    = %q
  datatype    = "enum"
  enum        = "a,b,c"
  description = "updated by acceptance test"
}
`, property),
				Check: resource.TestCheckNoResourceAttr("growthbook_attribute.test", "tags"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:                         "growthbook_attribute.test",
				ImportState:                          true,
				ImportStateId:                        property,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "property",
			},
		},
	})
}
