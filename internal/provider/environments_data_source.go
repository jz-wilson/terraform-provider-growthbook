// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	growthbook "github.com/jz-wilson/growthbook-go"
)

var (
	_ datasource.DataSource              = &environmentsDataSource{}
	_ datasource.DataSourceWithConfigure = &environmentsDataSource{}
)

// NewEnvironmentsDataSource is the constructor registered with the provider.
func NewEnvironmentsDataSource() datasource.DataSource {
	return &environmentsDataSource{}
}

type environmentsDataSource struct {
	client *growthbook.Client
}

// EnvironmentsDataSourceModel is the Terraform data model for the
// growthbook_environments data source.
type EnvironmentsDataSourceModel struct {
	Environments []EnvironmentDataSourceModel `tfsdk:"environments"`
}

func (d *environmentsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environments"
}

func (d *environmentsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists every GrowthBook environment in the organization.",
		Attributes: map[string]schema.Attribute{
			"environments": schema.ListNestedAttribute{
				MarkdownDescription: "All environments in the organization.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Environment id.",
							Computed:            true,
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
				},
			},
		},
	}
}

func (d *environmentsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *environmentsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	envs, err := d.client.ListEnvironments(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list GrowthBook environments", err.Error())
		return
	}

	model := EnvironmentsDataSourceModel{Environments: make([]EnvironmentDataSourceModel, 0, len(envs))}
	for _, env := range envs {
		envModel, diags := environmentDataSourceModelFromAPI(ctx, env)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		model.Environments = append(model.Environments, envModel)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
