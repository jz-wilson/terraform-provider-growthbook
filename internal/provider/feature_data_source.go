// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	growthbook "github.com/jz-wilson/growthbook-go"
)

var (
	_ datasource.DataSource              = &featureDataSource{}
	_ datasource.DataSourceWithConfigure = &featureDataSource{}
)

func NewFeatureDataSource() datasource.DataSource {
	return &featureDataSource{}
}

type featureDataSource struct {
	client *growthbook.Client
}

func (d *featureDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_feature"
}

func (d *featureDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *featureDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a GrowthBook feature flag by key.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Required: true, MarkdownDescription: "The feature's key."},
			"value_type":    schema.StringAttribute{Computed: true},
			"default_value": schema.StringAttribute{Computed: true},
			"description":   schema.StringAttribute{Computed: true},
			"project":       schema.StringAttribute{Computed: true},
			"owner":         schema.StringAttribute{Computed: true},
			"tags": schema.SetAttribute{
				Computed:    true,
				ElementType: elementTypeString,
			},
			"archived": schema.BoolAttribute{Computed: true},
			"environments": schema.MapNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"enabled": schema.BoolAttribute{Computed: true},
					},
				},
			},
			"rules": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type":        schema.StringAttribute{Computed: true},
						"description": schema.StringAttribute{Computed: true},
						"enabled":     schema.BoolAttribute{Computed: true},
						"condition":   schema.StringAttribute{Computed: true},
						"saved_groups": schema.ListNestedAttribute{
							Computed: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"match": schema.StringAttribute{Computed: true},
									"ids": schema.SetAttribute{
										Computed:    true,
										ElementType: elementTypeString,
									},
								},
							},
						},
						"all_environments": schema.BoolAttribute{Computed: true},
						"environments": schema.SetAttribute{
							Computed:    true,
							ElementType: elementTypeString,
						},
						"value":          schema.StringAttribute{Computed: true},
						"coverage":       schema.Float64Attribute{Computed: true},
						"hash_attribute": schema.StringAttribute{Computed: true},
						"experiment_id":  schema.StringAttribute{Computed: true},
						"variations": schema.ListNestedAttribute{
							Computed: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"variation_id": schema.StringAttribute{Computed: true},
									"value":        schema.StringAttribute{Computed: true},
								},
							},
						},
						"rule_id": schema.StringAttribute{Computed: true},
					},
				},
			},
			"revision_version": schema.Int64Attribute{Computed: true},
			"date_created":     schema.StringAttribute{Computed: true},
			"date_updated":     schema.StringAttribute{Computed: true},
		},
	}
}

func (d *featureDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config featureModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	feature, err := d.client.GetFeature(ctx, config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read GrowthBook feature", err.Error())
		return
	}

	state := featureModelFromAPI(ctx, feature, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
