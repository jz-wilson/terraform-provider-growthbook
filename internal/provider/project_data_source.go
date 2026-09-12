// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	growthbook "github.com/jz-wilson/growthbook-go"
)

var _ datasource.DataSource = &ProjectDataSource{}

// NewProjectDataSource is the constructor registered with the provider.
func NewProjectDataSource() datasource.DataSource {
	return &ProjectDataSource{}
}

// ProjectDataSource looks up an existing GrowthBook project by id.
type ProjectDataSource struct {
	client *growthbook.Client
}

func (d *ProjectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *ProjectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a GrowthBook project by id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "GrowthBook project id (`prj_...`).",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Display name of the project.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of the project.",
			},
			"public_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable identifier used in feature flag payloads.",
			},
			"restrict_access": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this project is restricted to explicitly granted members.",
			},
			"settings": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Per-project statistics settings overriding the organization defaults.",
				Attributes: map[string]schema.Attribute{
					"stats_engine": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Statistics engine, e.g. `bayesian` or `frequentist`.",
					},
					"confidence_level": schema.Float64Attribute{
						Computed:            true,
						MarkdownDescription: "Confidence level required to call a winner.",
					},
					"p_value_threshold": schema.Float64Attribute{
						Computed:            true,
						MarkdownDescription: "P-value threshold for statistical significance.",
					},
				},
			},
			"date_created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp the project was created.",
			},
			"date_updated": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp the project was last updated.",
			},
		},
	}
}

func (d *ProjectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *ProjectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := d.client.GetProject(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read GrowthBook project", fmt.Sprintf("id %q: %s", data.ID.ValueString(), err))
		return
	}

	modelFromProject(&data, project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
