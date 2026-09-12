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

var (
	_ datasource.DataSource              = &environmentDataSource{}
	_ datasource.DataSourceWithConfigure = &environmentDataSource{}
)

// NewEnvironmentDataSource is the constructor registered with the provider.
func NewEnvironmentDataSource() datasource.DataSource {
	return &environmentDataSource{}
}

type environmentDataSource struct {
	client *growthbook.Client
}

// EnvironmentDataSourceModel is the Terraform data model for the
// growthbook_environment data source.
type EnvironmentDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Description  types.String `tfsdk:"description"`
	ToggleOnList types.Bool   `tfsdk:"toggle_on_list"`
	DefaultState types.Bool   `tfsdk:"default_state"`
	Projects     types.Set    `tfsdk:"projects"`
	Parent       types.String `tfsdk:"parent"`
}

func (d *environmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (d *environmentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single GrowthBook environment by id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Environment id, for example `production`.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Human-readable description of the environment.",
				Computed:            true,
			},
			"toggle_on_list": schema.BoolAttribute{
				MarkdownDescription: "Whether this environment appears in the feature list toggle.",
				Computed:            true,
			},
			"default_state": schema.BoolAttribute{
				MarkdownDescription: "Default enabled state for new features in this environment.",
				Computed:            true,
			},
			"projects": schema.SetAttribute{
				MarkdownDescription: "Project ids this environment is scoped to.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"parent": schema.StringAttribute{
				MarkdownDescription: "Id of the parent environment, if any.",
				Computed:            true,
			},
		},
	}
}

func (d *environmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *environmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config EnvironmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := config.ID.ValueString()
	env, err := d.client.GetEnvironment(ctx, id)
	if err != nil {
		if growthbook.IsNotFound(err) {
			resp.Diagnostics.AddError("Environment not found", fmt.Sprintf("No GrowthBook environment with id %q was found.", id))
			return
		}
		resp.Diagnostics.AddError("Unable to read GrowthBook environment", err.Error())
		return
	}

	model, diags := environmentDataSourceModelFromAPI(ctx, *env)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
