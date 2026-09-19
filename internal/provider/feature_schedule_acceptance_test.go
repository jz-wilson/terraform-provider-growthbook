// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccFeatureResource_scheduleRules drives a fake GrowthBook server
// through adding a rule schedule, changing it, and then clearing it,
// asserting each step converges to an empty plan and that clearing it
// actually removes the server-side value.
func TestAccFeatureResource_scheduleRules(t *testing.T) {
	server, _ := newFakeFeatureServer()
	defer server.Close()

	t.Setenv("TF_ACC", "1")
	t.Setenv("GROWTHBOOK_API_KEY", "secret_test")
	t.Setenv("GROWTHBOOK_API_URL", server.URL)

	const featureID = "ft_schedule"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fakeAPIProviderConfig() + fmt.Sprintf(`
resource "growthbook_feature" "test" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"

  rules = [
    {
      type             = "force"
      all_environments = true
      value            = "true"
      schedule_type    = "schedule"
      schedule_rules = [
        { enabled = true, timestamp = "2026-01-01T00:00:00Z" },
        { enabled = false, timestamp = null },
      ]
    },
  ]
}
`, featureID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.0.schedule_type", "schedule"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.0.schedule_rules.#", "2"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.0.schedule_rules.0.enabled", "true"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.0.schedule_rules.0.timestamp", "2026-01-01T00:00:00Z"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.0.schedule_rules.1.enabled", "false"),
					resource.TestCheckNoResourceAttr("growthbook_feature.test", "rules.0.schedule_rules.1.timestamp"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// Clear the schedule: the empty list must clear the
				// server-side value, not leave it behind.
				Config: fakeAPIProviderConfig() + fmt.Sprintf(`
resource "growthbook_feature" "test" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"

  rules = [
    {
      type             = "force"
      all_environments = true
      value            = "true"
      schedule_rules   = []
    },
  ]
}
`, featureID),
				Check: resource.TestCheckResourceAttr("growthbook_feature.test", "rules.0.schedule_rules.#", "0"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "growthbook_feature.test",
				ImportState:       true,
				ImportStateVerify: true,
				Config: fakeAPIProviderConfig() + fmt.Sprintf(`
resource "growthbook_feature" "test" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"

  rules = [
    {
      type             = "force"
      all_environments = true
      value            = "true"
      schedule_rules   = []
    },
  ]
}
`, featureID),
			},
		},
	})
}

// TestAccFeatureResource_scheduleRulesProErrorTracksState mirrors
// TestAccFeatureResource_rulePrerequisitesEnterpriseErrorTracksState: on a
// sub-Pro plan GrowthBook accepts the write but silently drops
// schedule_rules, and the provider must report a clear error while still
// tracking the created resource (so a later plan without scheduling applies
// as an Update, not "feature already exists").
func TestAccFeatureResource_scheduleRulesProErrorTracksState(t *testing.T) {
	server, fake := newFakeFeatureServer()
	defer server.Close()

	fake.mu.Lock()
	fake.denyScheduleRules = true
	fake.mu.Unlock()

	t.Setenv("TF_ACC", "1")
	t.Setenv("GROWTHBOOK_API_KEY", "secret_test")
	t.Setenv("GROWTHBOOK_API_URL", server.URL)

	const deniedID = "ft_schedule_denied"
	ruleWithSchedule := `
  rules = [
    {
      type             = "force"
      all_environments = true
      value            = "true"
      schedule_rules = [
        { enabled = true, timestamp = "2026-01-01T00:00:00Z" },
      ]
    },
  ]
`
	ruleWithoutSchedule := `
  rules = [
    {
      type             = "force"
      all_environments = true
      value            = "true"
    },
  ]
`
	featureConfig := func(rules string) string {
		return fakeAPIProviderConfig() + fmt.Sprintf(`
resource "growthbook_feature" "denied" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"

  %s
}
`, deniedID, rules)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			fake.mu.Lock()
			defer fake.mu.Unlock()
			if _, ok := fake.features[deniedID]; ok {
				return fmt.Errorf("feature %q still exists on the fake server after destroy", deniedID)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config:      featureConfig(ruleWithSchedule),
				ExpectError: regexp.MustCompile(`Pro plan`),
			},
			{
				// If Create hadn't tracked state, this would fail with
				// "feature already exists" instead of applying as an Update.
				Config: featureConfig(ruleWithoutSchedule),
				Check:  resource.TestCheckResourceAttr("growthbook_feature.denied", "rules.0.type", "force"),
			},
		},
	})
}
