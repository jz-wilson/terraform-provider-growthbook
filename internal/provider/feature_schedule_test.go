// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	growthbook "github.com/jz-wilson/growthbook-go"
)

const scheduleTestTimestamp = "2026-01-01T00:00:00Z"

func TestScheduleRulesToAPI_NilVsEmpty(t *testing.T) {
	if got := scheduleRulesToAPI(nil); got != nil {
		t.Errorf("scheduleRulesToAPI(nil) = %#v, want nil", got)
	}
	got := scheduleRulesToAPI([]scheduleRuleModel{})
	if got == nil || len(got) != 0 {
		t.Errorf("scheduleRulesToAPI([]) = %#v, want non-nil empty slice", got)
	}
}

func TestScheduleRulesToAPI_Populated(t *testing.T) {
	rules := []scheduleRuleModel{
		{Enabled: types.BoolValue(true), Timestamp: types.StringValue(scheduleTestTimestamp)},
		{Enabled: types.BoolValue(false), Timestamp: types.StringNull()},
	}
	got := scheduleRulesToAPI(rules)
	if len(got) != 2 {
		t.Fatalf("scheduleRulesToAPI() = %#v, want 2 entries", got)
	}
	if !got[0].Enabled || got[0].Timestamp == nil || *got[0].Timestamp != scheduleTestTimestamp {
		t.Errorf("scheduleRulesToAPI()[0] = %#v", got[0])
	}
	if got[1].Enabled || got[1].Timestamp != nil {
		t.Errorf("scheduleRulesToAPI()[1] = %#v, want disabled with nil timestamp", got[1])
	}
}

func TestScheduleRulesFromAPI_NilVsEmpty(t *testing.T) {
	// Same normalization as prerequisitesFromAPI: the live API always sends
	// scheduleRules (as [] when empty), unlike the fake test server, which
	// omits it, so decoding must not distinguish between them.
	if got := scheduleRulesFromAPI(nil); got != nil {
		t.Errorf("scheduleRulesFromAPI(nil) = %#v, want nil", got)
	}
	if got := scheduleRulesFromAPI([]growthbook.ScheduleRule{}); got != nil {
		t.Errorf("scheduleRulesFromAPI([]) = %#v, want nil", got)
	}
}

func TestRuleToAPIFromAPI_ScheduleRules(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	ts := scheduleTestTimestamp

	rule := ruleModel{
		Type:            types.StringValue("force"),
		Description:     types.StringNull(),
		Enabled:         types.BoolValue(true),
		Condition:       types.StringNull(),
		AllEnvironments: types.BoolValue(true),
		Environments:    types.SetNull(types.StringType),
		Value:           types.StringValue("true"),
		Coverage:        types.Float64Null(),
		HashAttribute:   types.StringNull(),
		ExperimentID:    types.StringNull(),
		ScheduleType:    types.StringValue("schedule"),
		ScheduleRules: []scheduleRuleModel{
			{Enabled: types.BoolValue(true), Timestamp: types.StringValue(ts)},
		},
	}

	api := ruleToAPI(ctx, rule, &diags)
	if diags.HasError() {
		t.Fatalf("ruleToAPI() diags = %v", diags)
	}
	if api.ScheduleType != "schedule" || len(api.ScheduleRules) != 1 || api.ScheduleRules[0].Timestamp == nil || *api.ScheduleRules[0].Timestamp != ts {
		t.Fatalf("ruleToAPI() schedule = %#v", api)
	}

	back := ruleFromAPI(ctx, api, &diags)
	if diags.HasError() {
		t.Fatalf("ruleFromAPI() diags = %v", diags)
	}
	if back.ScheduleType.ValueString() != "schedule" || len(back.ScheduleRules) != 1 {
		t.Fatalf("ruleFromAPI() schedule = %#v", back)
	}
	if back.ScheduleRules[0].Timestamp.ValueString() != ts {
		t.Errorf("ruleFromAPI() ScheduleRules[0].Timestamp = %#v", back.ScheduleRules[0].Timestamp)
	}
}

func TestRequireScheduleRulesPersisted(t *testing.T) {
	scheduleRule := scheduleRuleModel{Enabled: types.BoolValue(true), Timestamp: types.StringValue(scheduleTestTimestamp)}
	planWithSchedule := featureModel{Rules: []ruleModel{{ScheduleRules: []scheduleRuleModel{scheduleRule}}}}
	planWithoutSchedule := featureModel{Rules: []ruleModel{{}}}

	cases := []struct {
		name      string
		plan      featureModel
		feature   *growthbook.Feature
		wantError bool
	}{
		{
			name:    "plan has none",
			plan:    planWithoutSchedule,
			feature: &growthbook.Feature{Rules: []growthbook.FeatureRule{{}}},
		},
		{
			name: "persisted",
			plan: planWithSchedule,
			feature: &growthbook.Feature{Rules: []growthbook.FeatureRule{{
				ScheduleRules: []growthbook.ScheduleRule{{Enabled: true, Timestamp: strPtr(scheduleTestTimestamp)}},
			}}},
		},
		{
			name:      "silently dropped",
			plan:      planWithSchedule,
			feature:   &growthbook.Feature{Rules: []growthbook.FeatureRule{{}}},
			wantError: true,
		},
		{
			name:      "rule missing from response entirely",
			plan:      planWithSchedule,
			feature:   &growthbook.Feature{},
			wantError: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := requireScheduleRulesPersisted(tc.plan, tc.feature)
			if diags.HasError() != tc.wantError {
				t.Errorf("requireScheduleRulesPersisted() diags = %v, wantError %v", diags, tc.wantError)
			}
			if tc.wantError {
				msg := diags[0].Detail()
				if !strings.Contains(msg, "Pro plan") {
					t.Errorf("error detail = %q, want mention of Pro plan", msg)
				}
			}
		})
	}
}

func strPtr(s string) *string { return &s }
