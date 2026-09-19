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

// TestAccFeatureResource_live exercises the resource and data source against
// a real GrowthBook instance (Cloud or self-hosted). Features are creatable
// on the free plan, so this only needs GROWTHBOOK_LIVE=1 plus the usual
// GROWTHBOOK_API_KEY/GROWTHBOOK_API_URL that testAccPreCheck verifies.
func TestAccFeatureResource_live(t *testing.T) {
	if os.Getenv("GROWTHBOOK_LIVE") == "" {
		t.Skip("set GROWTHBOOK_LIVE=1 to run acceptance tests against a real GrowthBook instance")
	}
	testAccPreCheck(t)

	key := acctest.RandomWithPrefix("tf-acc")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "growthbook_feature" "test" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"
  description   = "created by terraform-provider-growthbook acceptance tests"

  environments = {
    production = { enabled = true }
  }

  rules = [
    {
      type             = "force"
      description      = "US force"
      condition        = jsonencode({ country = "US" })
      all_environments = true
      value            = "true"
    },
    {
      type           = "rollout"
      coverage       = 0.5
      hash_attribute = "id"
      value          = "true"
    },
  ]
}

data "growthbook_feature" "test" {
  id = growthbook_feature.test.id

  depends_on = [growthbook_feature.test]
}
`, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_feature.test", "id", key),
					resource.TestCheckResourceAttr("growthbook_feature.test", "value_type", "boolean"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.#", "2"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.0.type", "force"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.1.type", "rollout"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "environments.production.enabled", "true"),
					resource.TestCheckResourceAttrSet("growthbook_feature.test", "rules.0.rule_id"),
					resource.TestCheckResourceAttrSet("growthbook_feature.test", "rules.1.rule_id"),
					resource.TestCheckResourceAttr("data.growthbook_feature.test", "value_type", "boolean"),
					resource.TestCheckResourceAttr("data.growthbook_feature.test", "rules.#", "2"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// Update the description and reorder/modify the rules.
				Config: fmt.Sprintf(`
resource "growthbook_feature" "test" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"
  description   = "updated by terraform-provider-growthbook acceptance tests"

  environments = {
    production = { enabled = true }
  }

  rules = [
    {
      type           = "rollout"
      coverage       = 0.75
      hash_attribute = "id"
      value          = "true"
    },
    {
      type             = "force"
      description      = "US force, updated"
      condition        = jsonencode({ country = "US" })
      all_environments = true
      value            = "true"
    },
  ]
}
`, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_feature.test", "description", "updated by terraform-provider-growthbook acceptance tests"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.0.type", "rollout"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.0.coverage", "0.75"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.1.type", "force"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.1.description", "US force, updated"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "growthbook_feature.test",
				ImportState:       true,
				ImportStateVerify: true,
				Config: fmt.Sprintf(`
resource "growthbook_feature" "test" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"
  description   = "updated by terraform-provider-growthbook acceptance tests"

  environments = {
    production = { enabled = true }
  }

  rules = [
    {
      type           = "rollout"
      coverage       = 0.75
      hash_attribute = "id"
      value          = "true"
    },
    {
      type             = "force"
      description      = "US force, updated"
      condition        = jsonencode({ country = "US" })
      all_environments = true
      value            = "true"
    },
  ]
}
`, key),
			},
		},
	})
}

// TestAccFeatureResource_prerequisitesLive exercises feature-level (a set
// of feature IDs) and rule-level ({id, condition}) prerequisites against a
// real GrowthBook instance: add, change the rule-level condition, then
// remove both, asserting the removal actually clears the server-side
// value after a refresh rather than leaving it behind.
func TestAccFeatureResource_prerequisitesLive(t *testing.T) {
	if os.Getenv("GROWTHBOOK_LIVE") == "" {
		t.Skip("set GROWTHBOOK_LIVE=1 to run acceptance tests against a real GrowthBook instance")
	}
	testAccPreCheck(t)

	parentKey := acctest.RandomWithPrefix("tf-acc-")
	childKey := acctest.RandomWithPrefix("tf-acc-")

	parentConfig := fmt.Sprintf(`
resource "growthbook_feature" "parent" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"
}
`, parentKey)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: parentConfig + fmt.Sprintf(`
resource "growthbook_feature" "child" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"

  prerequisites = [growthbook_feature.parent.id]

  rules = [
    {
      type             = "force"
      all_environments = true
      value            = "true"
      prerequisites = [
        {
          id        = growthbook_feature.parent.id
          condition = jsonencode({ value = true })
        },
      ]
    },
  ]
}
`, childKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_feature.child", "prerequisites.#", "1"),
					resource.TestCheckTypeSetElemAttr("growthbook_feature.child", "prerequisites.*", parentKey),
					resource.TestCheckResourceAttr("growthbook_feature.child", "rules.0.prerequisites.#", "1"),
					resource.TestCheckResourceAttr("growthbook_feature.child", "rules.0.prerequisites.0.condition", `{"value":true}`),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// Change the rule-level condition.
				Config: parentConfig + fmt.Sprintf(`
resource "growthbook_feature" "child" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"

  prerequisites = [growthbook_feature.parent.id]

  rules = [
    {
      type             = "force"
      all_environments = true
      value            = "true"
      prerequisites = [
        {
          id        = growthbook_feature.parent.id
          condition = jsonencode({ value = false })
        },
      ]
    },
  ]
}
`, childKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_feature.child", "rules.0.prerequisites.0.condition", `{"value":false}`),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// Remove all prerequisites: assert they are actually gone
				// after a refresh, not merely absent from this apply's plan.
				Config: parentConfig + fmt.Sprintf(`
resource "growthbook_feature" "child" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"

  prerequisites = []

  rules = [
    {
      type             = "force"
      all_environments = true
      value            = "true"
      prerequisites    = []
    },
  ]
}
`, childKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_feature.child", "prerequisites.#", "0"),
					resource.TestCheckResourceAttr("growthbook_feature.child", "rules.0.prerequisites.#", "0"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "growthbook_feature.child",
				ImportState:       true,
				ImportStateVerify: true,
				Config: parentConfig + fmt.Sprintf(`
resource "growthbook_feature" "child" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"

  prerequisites = []

  rules = [
    {
      type             = "force"
      all_environments = true
      value            = "true"
      prerequisites    = []
    },
  ]
}
`, childKey),
			},
		},
	})
}
