// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	growthbook "github.com/jz-wilson/growthbook-go"
)

var _ datasource.DataSource = &sdkConnectionsDataSource{}

func newSDKConnectionsDataSource() datasource.DataSource {
	return &sdkConnectionsDataSource{}
}

type sdkConnectionsDataSource struct {
	client *growthbook.Client
}

type sdkConnectionsDataSourceModel struct {
	SDKConnections []sdkConnectionModel `tfsdk:"sdk_connections"`
}

func (d *sdkConnectionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sdk_connections"
}

func (d *sdkConnectionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	itemAttrs := map[string]dschema.Attribute{
		"id": dschema.StringAttribute{
			MarkdownDescription: "SDK connection id.",
			Computed:            true,
		},
		"name": dschema.StringAttribute{
			MarkdownDescription: "Display name for the SDK connection.",
			Computed:            true,
		},
		"language": dschema.StringAttribute{
			MarkdownDescription: "First SDK language reported for this connection.",
			Computed:            true,
		},
		"environment": dschema.StringAttribute{
			MarkdownDescription: "Environment this connection serves.",
			Computed:            true,
		},
	}
	for k, v := range sdkConnectionDataSourceOptionAttributes() {
		itemAttrs[k] = v
	}

	resp.Schema = dschema.Schema{
		MarkdownDescription: "Lists every GrowthBook SDK connection in the organization.",
		Attributes: map[string]dschema.Attribute{
			"sdk_connections": dschema.ListNestedAttribute{
				MarkdownDescription: "All SDK connections in the organization.",
				Computed:            true,
				NestedObject: dschema.NestedAttributeObject{
					Attributes: itemAttrs,
				},
			},
		},
	}
}

func (d *sdkConnectionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *sdkConnectionsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	conns, err := d.client.ListSDKConnections(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list SDK connections", err.Error())
		return
	}

	state := sdkConnectionsDataSourceModel{SDKConnections: make([]sdkConnectionModel, 0, len(conns))}
	for i := range conns {
		item, diags := sdkConnectionFromAPI(ctx, &conns[i], "")
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.SDKConnections = append(state.SDKConnections, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
