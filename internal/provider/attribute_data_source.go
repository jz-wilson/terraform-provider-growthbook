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
	_ datasource.DataSource              = &attributeDataSource{}
	_ datasource.DataSourceWithConfigure = &attributeDataSource{}
)

// NewAttributeDataSource is the constructor registered with the provider.
func NewAttributeDataSource() datasource.DataSource {
	return &attributeDataSource{}
}

type attributeDataSource struct {
	client *growthbook.Client
}

func (d *attributeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_attribute"
}

func (d *attributeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single GrowthBook SDK targeting attribute by property.",
		Attributes: map[string]schema.Attribute{
			"property": schema.StringAttribute{
				MarkdownDescription: "Attribute property name, and its identifier.",
				Required:            true,
			},
			"datatype": schema.StringAttribute{
				MarkdownDescription: "Attribute datatype.",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the attribute.",
				Computed:            true,
			},
			"hash_attribute": schema.BoolAttribute{
				MarkdownDescription: "Whether this attribute is hashed before being sent to the SDK.",
				Computed:            true,
			},
			"archived": schema.BoolAttribute{
				MarkdownDescription: "Whether this attribute is archived.",
				Computed:            true,
			},
			"enum": schema.StringAttribute{
				MarkdownDescription: "Comma-separated list of allowed values.",
				Computed:            true,
			},
			"format": schema.StringAttribute{
				MarkdownDescription: "Attribute format.",
				Computed:            true,
			},
			"projects": schema.SetAttribute{
				MarkdownDescription: "Project ids this attribute is scoped to.",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"tags": schema.SetAttribute{
				MarkdownDescription: "Tags applied to this attribute.",
				ElementType:         types.StringType,
				Computed:            true,
			},
		},
	}
}

func (d *attributeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *attributeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config AttributeResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	property := config.Property.ValueString()
	attr, err := d.client.GetAttribute(ctx, property)
	if err != nil {
		if growthbook.IsNotFound(err) {
			resp.Diagnostics.AddError("Attribute not found", fmt.Sprintf("No GrowthBook attribute with property %q was found.", property))
			return
		}
		resp.Diagnostics.AddError("Unable to read GrowthBook attribute", err.Error())
		return
	}

	model, diags := attributeModelFromAPI(ctx, attr)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
