// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	growthbook "github.com/jz-wilson/growthbook-go"
)

var _ datasource.DataSource = &sdkConnectionDataSource{}

func newSDKConnectionDataSource() datasource.DataSource {
	return &sdkConnectionDataSource{}
}

type sdkConnectionDataSource struct {
	client *growthbook.Client
}

func (d *sdkConnectionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sdk_connection"
}

func (d *sdkConnectionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]dschema.Attribute{
		"id": dschema.StringAttribute{
			MarkdownDescription: "SDK connection id.",
			Required:            true,
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
		attrs[k] = v
	}

	resp.Schema = dschema.Schema{
		MarkdownDescription: "Looks up a single GrowthBook SDK connection by id.",
		Attributes:          attrs,
	}
}

func (d *sdkConnectionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *sdkConnectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config sdkConnectionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	conn, err := d.client.GetSDKConnection(ctx, config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read SDK connection", err.Error())
		return
	}

	state, diags := sdkConnectionFromAPI(ctx, conn, "")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
