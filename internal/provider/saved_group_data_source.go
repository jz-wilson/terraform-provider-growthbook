// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	growthbook "github.com/jz-wilson/growthbook-go"
)

var _ datasource.DataSource = &SavedGroupDataSource{}

// NewSavedGroupDataSource is the constructor registered with the provider.
func NewSavedGroupDataSource() datasource.DataSource {
	return &SavedGroupDataSource{}
}

// SavedGroupDataSource looks up an existing GrowthBook saved group by id.
type SavedGroupDataSource struct {
	client *growthbook.Client
}

func (d *SavedGroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_saved_group"
}

func (d *SavedGroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a GrowthBook saved group by id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "GrowthBook saved group id.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Display name of the saved group.",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Saved group type: `condition` or `list`.",
			},
			"condition": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "JSON-encoded condition for the group, when `type = \"condition\"`.",
			},
			"attribute_key": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Attribute key the group's list of values is based on, when `type = \"list\"`.",
			},
			"values": schema.SetAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "List of attribute values, when `type = \"list\"`.",
			},
			"owner": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The userId of the owner.",
			},
			"owner_email": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Email address of the owner, when it can be resolved to a known user.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of the saved group.",
			},
			"projects": schema.SetAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Project ids this saved group is scoped to.",
			},
			"archived": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the saved group is archived.",
			},
			"use_empty_list_group": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "GrowthBook-managed flag for empty list-group handling.",
			},
			"date_created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp the saved group was created.",
			},
			"date_updated": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp the saved group was last updated.",
			},
		},
	}
}

func (d *SavedGroupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *SavedGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SavedGroupModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := d.client.GetSavedGroup(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read GrowthBook saved group", fmt.Sprintf("id %q: %s", data.ID.ValueString(), err))
		return
	}

	model, diags := savedGroupModelFromAPI(ctx, group)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
