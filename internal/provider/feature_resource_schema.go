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

func stringOneOf(values []string) validator.String {
	return stringvalidator.OneOf(values...)
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
				"enabled":     schema.BoolAttribute{Optional: true},
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
			"revision_version": schema.Int64Attribute{Computed: true},
			"date_created":     schema.StringAttribute{Computed: true},
			"date_updated":     schema.StringAttribute{Computed: true},
		},
	}
}
