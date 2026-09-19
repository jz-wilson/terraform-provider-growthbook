// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var elementTypeString = types.StringType

var featureValueTypes = []string{"boolean", "string", "number", "json"}

var featureRuleTypes = []string{"force", "rollout", "experiment-ref"}

var featureRuleScheduleTypes = []string{"none", "schedule"}

func stringOneOf(values []string) validator.String {
	return stringvalidator.OneOf(values...)
}

// featurePrerequisitesSchema is the feature-level `prerequisites` attribute:
// a set of other features' IDs, each of which must evaluate to true. Unlike
// rule-level prerequisites, there is no per-entry condition here.
func featurePrerequisitesSchema() schema.SetAttribute {
	return schema.SetAttribute{
		Optional:            true,
		ElementType:         elementTypeString,
		MarkdownDescription: "Feature IDs; each must evaluate to `true`. Omit to leave unmanaged; set to `[]` to clear.",
	}
}

// rulePrerequisiteSchema is the `prerequisites` list attribute on a rule:
// each entry gates the rule on another feature's value via a JSON
// condition.
func rulePrerequisiteSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Optional: true,
		MarkdownDescription: "Gates the rule on another feature's value. Omit to leave unmanaged; set to `[]` to clear. " +
			"Requires GrowthBook Enterprise (the \"prerequisite-targeting\" commercial feature); on other plans " +
			"GrowthBook silently drops it and the provider reports an error rather than let state drift.",
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"id": schema.StringAttribute{
					Required:            true,
					MarkdownDescription: "The parent feature's key.",
				},
				"condition": schema.StringAttribute{
					Required:            true,
					MarkdownDescription: "JSON condition evaluated against the parent feature's value, e.g. `{\"value\": true}`. Whitespace/property-order differences are ignored.",
					Validators:          []validator.String{jsonStringValidator{}},
					PlanModifiers:       []planmodifier.String{jsonNormalizePlanModifier{}},
				},
			},
		},
	}
}

func featureRuleSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Optional:            true,
		MarkdownDescription: "Ordered list of targeting rules. Omit this attribute to leave rules unmanaged; set it to `[]` to clear all rules.",
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"type": schema.StringAttribute{
					Required:            true,
					MarkdownDescription: "One of `force`, `rollout`, `experiment-ref`.",
					Validators:          []validator.String{stringOneOf(featureRuleTypes)},
				},
				"description": schema.StringAttribute{Optional: true},
				"enabled": schema.BoolAttribute{
					Optional: true,
					Computed: true,
					Default:  booldefault.StaticBool(true),
				},
				"condition": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "JSON targeting condition. Whitespace/property-order differences are ignored.",
					Validators:          []validator.String{jsonStringValidator{}},
					PlanModifiers:       []planmodifier.String{jsonNormalizePlanModifier{}},
				},
				"saved_groups": schema.ListNestedAttribute{
					Optional: true,
					NestedObject: schema.NestedAttributeObject{
						Attributes: map[string]schema.Attribute{
							"match": schema.StringAttribute{Required: true},
							"ids": schema.SetAttribute{
								Required:    true,
								ElementType: elementTypeString,
							},
						},
					},
				},
				"prerequisites": rulePrerequisiteSchema(),
				"all_environments": schema.BoolAttribute{
					Optional: true,
					Computed: true,
					Default:  booldefault.StaticBool(false),
				},
				"environments": schema.SetAttribute{
					Optional:    true,
					ElementType: elementTypeString,
				},
				"value":          schema.StringAttribute{Optional: true},
				"coverage":       schema.Float64Attribute{Optional: true},
				"hash_attribute": schema.StringAttribute{Optional: true},
				"experiment_id":  schema.StringAttribute{Optional: true},
				"variations": schema.ListNestedAttribute{
					Optional: true,
					NestedObject: schema.NestedAttributeObject{
						Attributes: map[string]schema.Attribute{
							"variation_id": schema.StringAttribute{Optional: true},
							"value":        schema.StringAttribute{Required: true},
						},
					},
				},
				"rule_id": schema.StringAttribute{
					Computed:            true,
					MarkdownDescription: "Server-assigned rule identifier.",
				},
				"schedule_type": schema.StringAttribute{
					Optional: true,
					MarkdownDescription: "Simple on/off scheduling mode: `none` or `schedule`. Set to `schedule` when `schedule_rules` " +
						"is configured. Requires GrowthBook Pro (the \"schedule-feature-flag\" commercial feature); on other plans " +
						"GrowthBook silently drops `schedule_rules` and the provider reports an error rather than let state drift.",
					Validators: []validator.String{stringOneOf(featureRuleScheduleTypes)},
				},
				"schedule_rules": schema.ListNestedAttribute{
					Optional: true,
					MarkdownDescription: "Time-based on/off schedule for this rule. Omit to leave unmanaged; set to `[]` to clear. " +
						"Requires GrowthBook Pro, same as `schedule_type`.",
					NestedObject: schema.NestedAttributeObject{
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required:            true,
								MarkdownDescription: "Whether the rule is enabled or disabled once this transition activates.",
							},
							"timestamp": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "RFC3339 timestamp when this transition activates. Omit for an open-ended transition.",
							},
						},
					},
				},
			},
		},
	}
}

func featureEnvironmentSchema() schema.MapNestedAttribute {
	return schema.MapNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"enabled": schema.BoolAttribute{Required: true},
			},
		},
	}
}

func (r *featureResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a GrowthBook feature flag, its per-environment enablement, and its targeting rules.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The feature's key.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"value_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "One of `boolean`, `string`, `number`, `json`.",
				Validators:          []validator.String{stringOneOf(featureValueTypes)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"default_value": schema.StringAttribute{Required: true},
			"description": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"project": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"owner": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"tags": schema.SetAttribute{
				Optional:    true,
				ElementType: elementTypeString,
			},
			"archived": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"environments":     featureEnvironmentSchema(),
			"rules":            featureRuleSchema(),
			"prerequisites":    featurePrerequisitesSchema(),
			"revision_version": schema.Int64Attribute{Computed: true},
			"date_created":     schema.StringAttribute{Computed: true},
			"date_updated":     schema.StringAttribute{Computed: true},
		},
	}
}
