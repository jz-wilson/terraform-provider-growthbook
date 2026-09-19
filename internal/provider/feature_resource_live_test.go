// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"regexp"
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

// TestAccFeatureResource_prerequisitesLive exercises feature-level
// prerequisites (a set of feature IDs) against a real GrowthBook instance:
// add, then remove, asserting the removal actually clears the server-side
// value after a refresh rather than leaving it behind. Rule-level
// prerequisites are not exercised here: GrowthBook's "prerequisite-
// targeting" is an Enterprise-only commercial feature and the free/
// unlicensed CI instance silently drops it rather than erroring - see
// TestAccFeatureResource_rulePrerequisitesRequiresEnterpriseLive, which
// asserts that behavior instead.
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
}
`, childKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_feature.child", "prerequisites.#", "1"),
					resource.TestCheckTypeSetElemAttr("growthbook_feature.child", "prerequisites.*", parentKey),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// Remove it: assert it's actually gone after a refresh, not
				// merely absent from this apply's plan.
				Config: parentConfig + fmt.Sprintf(`
resource "growthbook_feature" "child" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"

  prerequisites = []
}
`, childKey),
				Check: resource.TestCheckResourceAttr("growthbook_feature.child", "prerequisites.#", "0"),
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
}
`, childKey),
			},
		},
	})
}

// TestAccFeatureResource_rulePrerequisitesRequiresEnterpriseLive proves the
// free-plan behavior for rule-level prerequisites against a real
// GrowthBook instance: the API accepts the create request but silently
// drops rules[0].prerequisites (GrowthBook's "prerequisite-targeting" is
// Enterprise-only), and the provider must turn that into a clear error
// instead of surfacing the Plugin Framework's confusing "element 0 has
// vanished" consistency failure. The feature is created via the real API
// before the error surfaces (Terraform never records it in state, since
// Create never returns one), so cleanup relies on the tf-acc- sweeper
// rather than a state-driven destroy.
func TestAccFeatureResource_rulePrerequisitesRequiresEnterpriseLive(t *testing.T) {
	if os.Getenv("GROWTHBOOK_LIVE") == "" {
		t.Skip("set GROWTHBOOK_LIVE=1 to run acceptance tests against a real GrowthBook instance")
	}
	testAccPreCheck(t)

	parentKey := acctest.RandomWithPrefix("tf-acc-")
	childKey := acctest.RandomWithPrefix("tf-acc-")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "growthbook_feature" "parent" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"
}

resource "growthbook_feature" "child" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"

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
`, parentKey, childKey),
				ExpectError: regexp.MustCompile(`Enterprise plan`),
			},
		},
	})
}
