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

const (
	prereqTestParentID = "parent_feature"
	prereqTestCond     = `{"value":true}`
)

func TestFeaturePrerequisitesToAPI_NilVsEmpty(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics

	// A null/unknown set leaves prerequisites unmanaged: no pointer at all.
	if got := featurePrerequisitesToAPI(ctx, types.SetNull(types.StringType), &diags); got != nil {
		t.Errorf("featurePrerequisitesToAPI(null) = %#v, want nil", got)
	}

	// A known, empty set clears prerequisites: a pointer to [].
	got := featurePrerequisitesToAPI(ctx, emptyStringSet(), &diags)
	if got == nil {
		t.Fatal("featurePrerequisitesToAPI([]) = nil, want non-nil pointer to empty slice")
	}
	if len(*got) != 0 {
		t.Errorf("featurePrerequisitesToAPI([]) = %#v, want empty slice", *got)
	}
	if diags.HasError() {
		t.Fatalf("diags = %v", diags)
	}
}

func TestFeaturePrerequisitesToAPI_Populated(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics

	set, d := types.SetValueFrom(ctx, types.StringType, []string{prereqTestParentID})
	diags.Append(d...)
	if diags.HasError() {
		t.Fatalf("SetValueFrom() diags = %v", diags)
	}

	got := featurePrerequisitesToAPI(ctx, set, &diags)
	if got == nil || len(*got) != 1 {
		t.Fatalf("featurePrerequisitesToAPI() = %#v, want 1 entry", got)
	}
	if (*got)[0] != prereqTestParentID {
		t.Errorf("featurePrerequisitesToAPI()[0] = %#v", (*got)[0])
	}
}

func TestPrerequisitesFromAPI_NilVsEmpty(t *testing.T) {
	// Both a nil and a literal empty API response normalize to nil: the
	// live API always sends "prerequisites" (as [] when empty), unlike the
	// fake test server, which omits it, so decoding must not distinguish
	// between them at this layer.
	if got := prerequisitesFromAPI(nil); got != nil {
		t.Errorf("prerequisitesFromAPI(nil) = %#v, want nil", got)
	}
	if got := prerequisitesFromAPI([]growthbook.FeaturePrerequisite{}); got != nil {
		t.Errorf("prerequisitesFromAPI([]) = %#v, want nil", got)
	}
}

func TestRuleToAPIFromAPI_Prerequisites(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics

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
		Prerequisites: []prerequisiteModel{
			{ID: types.StringValue(prereqTestParentID), Condition: types.StringValue(prereqTestCond)},
		},
	}

	api := ruleToAPI(ctx, rule, &diags)
	if diags.HasError() {
		t.Fatalf("ruleToAPI() diags = %v", diags)
	}
	if len(api.Prerequisites) != 1 || api.Prerequisites[0].ID != prereqTestParentID || api.Prerequisites[0].Condition != prereqTestCond {
		t.Fatalf("ruleToAPI() Prerequisites = %#v", api.Prerequisites)
	}

	back := ruleFromAPI(ctx, api, &diags)
	if diags.HasError() {
		t.Fatalf("ruleFromAPI() diags = %v", diags)
	}
	if len(back.Prerequisites) != 1 || back.Prerequisites[0].ID.ValueString() != prereqTestParentID {
		t.Fatalf("ruleFromAPI() Prerequisites = %#v", back.Prerequisites)
	}
}

func TestRequireRulePrerequisitesPersisted(t *testing.T) {
	planWithPrereq := featureModel{
		Rules: []ruleModel{{
			Prerequisites: []prerequisiteModel{
				{ID: types.StringValue(prereqTestParentID), Condition: types.StringValue(prereqTestCond)},
			},
		}},
	}
	planWithoutPrereq := featureModel{Rules: []ruleModel{{}}}

	cases := []struct {
		name      string
		plan      featureModel
		feature   *growthbook.Feature
		wantError bool
	}{
		{
			name:    "plan has none",
			plan:    planWithoutPrereq,
			feature: &growthbook.Feature{Rules: []growthbook.FeatureRule{{}}},
		},
		{
			name: "persisted",
			plan: planWithPrereq,
			feature: &growthbook.Feature{Rules: []growthbook.FeatureRule{{
				Prerequisites: []growthbook.FeaturePrerequisite{{ID: prereqTestParentID, Condition: prereqTestCond}},
			}}},
		},
		{
			name:      "silently dropped",
			plan:      planWithPrereq,
			feature:   &growthbook.Feature{Rules: []growthbook.FeatureRule{{}}},
			wantError: true,
		},
		{
			name:      "rule missing from response entirely",
			plan:      planWithPrereq,
			feature:   &growthbook.Feature{},
			wantError: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := requireRulePrerequisitesPersisted(tc.plan, tc.feature)
			if diags.HasError() != tc.wantError {
				t.Errorf("requireRulePrerequisitesPersisted() diags = %v, wantError %v", diags, tc.wantError)
			}
			if tc.wantError {
				msg := diags[0].Detail()
				if !strings.Contains(msg, "Enterprise plan") {
					t.Errorf("error detail = %q, want mention of Enterprise plan", msg)
				}
			}
		})
	}
}

// TestJSONSemanticEqual_Prerequisites exercises the same JSON-normalization
// plan modifier condition uses, which prerequisite conditions reuse
// verbatim (jsonNormalizePlanModifier / jsonSemanticEqual).
func TestJSONSemanticEqual_Prerequisites(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"identical", `{"value":true}`, `{"value":true}`, true},
		{"whitespace differs", `{"value":true}`, `{ "value" : true }`, true},
		{"key order differs", `{"a":1,"b":2}`, `{"b":2,"a":1}`, true},
		{"value differs", `{"value":true}`, `{"value":false}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := jsonSemanticEqual(tc.a, tc.b); got != tc.want {
				t.Errorf("jsonSemanticEqual(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
