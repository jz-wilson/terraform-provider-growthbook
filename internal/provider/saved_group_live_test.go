// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccSavedGroupResource_live exercises full CRUD against a real
// GrowthBook organization. It only runs when GROWTHBOOK_LIVE=1, in addition
// to the usual TF_ACC=1 and GROWTHBOOK_API_KEY / GROWTHBOOK_API_URL that
// testAccPreCheck requires. Saved groups are creatable on the GrowthBook free
// plan.
func TestAccSavedGroupResource_live(t *testing.T) {
	if os.Getenv("GROWTHBOOK_LIVE") != "1" {
		t.Skip("set GROWTHBOOK_LIVE=1 to run live GrowthBook acceptance tests")
	}
	testAccPreCheck(t)

	name := acctest.RandomWithPrefix("tf-acc")
	updatedName := name + "-updated"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "growthbook_saved_group" "test" {
  name          = %q
  type          = "list"
  attribute_key = "userId"
  values        = ["user-1", "user-2"]
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "name", name),
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "type", "list"),
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "values.#", "2"),
					resource.TestCheckResourceAttrSet("growthbook_saved_group.test", "id"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: fmt.Sprintf(`
resource "growthbook_saved_group" "test" {
  name          = %q
  type          = "list"
  attribute_key = "userId"
  values        = []
}
`, updatedName),
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
