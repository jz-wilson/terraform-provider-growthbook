// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

// Package provider implements the GrowthBook Terraform provider on top of
// the shared github.com/jz-wilson/growthbook-go client.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	growthbook "github.com/jz-wilson/growthbook-go"
)

// Ensure GrowthBookProvider satisfies the provider interface.
var _ provider.Provider = &GrowthBookProvider{}

// GrowthBookProvider defines the provider implementation.
type GrowthBookProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// GrowthBookProviderModel describes the provider data model.
type GrowthBookProviderModel struct {
	APIKey types.String `tfsdk:"api_key"`
	APIURL types.String `tfsdk:"api_url"`
}

func (p *GrowthBookProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "growthbook"
	resp.Version = p.version
}

func (p *GrowthBookProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage GrowthBook projects, environments, features and SDK connections through the GrowthBook REST API.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "GrowthBook secret API key (`secret_...`). Can also be set with the `GROWTHBOOK_API_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"api_url": schema.StringAttribute{
				MarkdownDescription: "API root including the `/api` suffix, for example `https://growthbook.example.com/api`. Defaults to GrowthBook Cloud (`" + growthbook.DefaultAPIURL + "`). Can also be set with the `GROWTHBOOK_API_URL` environment variable.",
				Optional:            true,
			},
		},
	}
}

func (p *GrowthBookProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data GrowthBookProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.APIKey.IsUnknown() || data.APIURL.IsUnknown() {
		resp.Diagnostics.AddError(
			"Unknown provider configuration",
			"api_key and api_url must be known at plan time; do not derive them from resources created in the same configuration.",
		)
		return
	}

	apiKey := os.Getenv("GROWTHBOOK_API_KEY")
	if !data.APIKey.IsNull() {
		apiKey = data.APIKey.ValueString()
	}
	apiURL := os.Getenv("GROWTHBOOK_API_URL")
	if !data.APIURL.IsNull() {
		apiURL = data.APIURL.ValueString()
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing GrowthBook API key",
			"Set the api_key provider attribute or the GROWTHBOOK_API_KEY environment variable.",
		)
		return
	}

	client, err := growthbook.New(growthbook.Credentials{APIKey: apiKey, APIURL: apiURL})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create GrowthBook client", err.Error())
		return
	}

	tflog.Info(ctx, "Configured GrowthBook client", map[string]any{"api_url": apiURL})

	resp.DataSourceData = client
	resp.ResourceData = client
}

// Resources lists every managed resource this provider offers.
func (p *GrowthBookProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewEnvironmentResource,
		NewProjectResource,
		NewFeatureResource,
		newSDKConnectionResource,
		NewAttributeResource,
		NewSavedGroupResource,
	}
}

// DataSources lists every data source this provider offers.
func (p *GrowthBookProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewEnvironmentDataSource,
		NewEnvironmentsDataSource,
		NewProjectDataSource,
		NewFeatureDataSource,
		newSDKConnectionDataSource,
		newSDKConnectionsDataSource,
		NewAttributeDataSource,
		NewSavedGroupDataSource,
	}
}

// New returns a provider constructor for the given version string.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &GrowthBookProvider{version: version}
	}
}
