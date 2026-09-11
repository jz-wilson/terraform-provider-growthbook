// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	growthbook "github.com/jz-wilson/growthbook-go"
)

var (
	_ resource.Resource                = &sdkConnectionResource{}
	_ resource.ResourceWithImportState = &sdkConnectionResource{}
)

func newSDKConnectionResource() resource.Resource {
	return &sdkConnectionResource{}
}

type sdkConnectionResource struct {
	client *growthbook.Client
}

func (r *sdkConnectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sdk_connection"
}

func (r *sdkConnectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	boolAttr := func(desc string) rschema.Attribute {
		return rschema.BoolAttribute{MarkdownDescription: desc, Optional: true, Computed: true}
	}

	resp.Schema = rschema.Schema{
		MarkdownDescription: "Manages a GrowthBook SDK connection, which issues the client key an SDK uses to fetch feature flag and experiment payloads for one environment.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				MarkdownDescription: "SDK connection id.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": rschema.StringAttribute{
				MarkdownDescription: "Display name for the SDK connection.",
				Required:            true,
			},
			"language": rschema.StringAttribute{
				MarkdownDescription: "SDK language, for example `javascript`, `node`, `go`, or `react`. The API reports the configured language back as a `languages` list; this attribute keeps the value you configured.",
				Required:            true,
			},
			"environment": rschema.StringAttribute{
				MarkdownDescription: "Environment this connection serves, for example `production` or `staging`.",
				Required:            true,
			},
			"projects": rschema.SetAttribute{
				MarkdownDescription: "Project ids this connection is scoped to. Omit or leave empty for all projects.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
			},
			"sdk_version": rschema.StringAttribute{
				MarkdownDescription: "SDK version reported to GrowthBook for payload compatibility.",
				Optional:            true,
				Computed:            true,
			},
			"encrypt_payload":                         boolAttr("Whether the SDK payload is encrypted."),
			"include_visual_experiments":              boolAttr("Whether visual editor experiments are included in the payload."),
			"include_draft_experiments":               boolAttr("Whether draft experiments are included in the payload."),
			"include_draft_experiment_refs":           boolAttr("Whether draft experiment references are included in the payload."),
			"include_experiment_names":                boolAttr("Whether experiment names are included in the payload."),
			"include_redirect_experiments":            boolAttr("Whether URL redirect experiments are included in the payload."),
			"include_rule_ids":                        boolAttr("Whether rule ids are included in the payload."),
			"include_project_id_in_metadata":          boolAttr("Whether project ids are included in feature metadata."),
			"include_custom_fields_in_metadata":       boolAttr("Whether custom fields are included in feature metadata."),
			"include_tags_in_metadata":                boolAttr("Whether tags are included in feature metadata."),
			"include_experiment_schedule_in_metadata": boolAttr("Whether experiment schedules are included in feature metadata."),
			"proxy_enabled":                           boolAttr("Whether GrowthBook Proxy is enabled for this connection."),
			"proxy_host": rschema.StringAttribute{
				MarkdownDescription: "GrowthBook Proxy host URL.",
				Optional:            true,
				Computed:            true,
			},
			"hash_secure_attributes":           boolAttr("Whether secure attributes are hashed before being sent to the SDK."),
			"remote_eval_enabled":              boolAttr("Whether remote evaluation is enabled for this connection."),
			"saved_group_references_enabled":   boolAttr("Whether saved group references are enabled in the payload."),
			"include_referenced_prerequisites": boolAttr("Whether referenced prerequisite features are included in the payload."),
			"allowed_custom_fields_in_metadata": rschema.SetAttribute{
				MarkdownDescription: "Custom field keys allowed in feature metadata.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
			},
			"key": rschema.StringAttribute{
				MarkdownDescription: "Client key used by SDKs to fetch this connection's payload.",
				Computed:            true,
				Sensitive:           true,
			},
			"encryption_key": rschema.StringAttribute{
				MarkdownDescription: "Key used to decrypt an encrypted payload.",
				Computed:            true,
				Sensitive:           true,
			},
			"proxy_signing_key": rschema.StringAttribute{
				MarkdownDescription: "Signing key used by GrowthBook Proxy.",
				Computed:            true,
				Sensitive:           true,
			},
			"connected": rschema.BoolAttribute{
				MarkdownDescription: "Whether this SDK connection has received at least one request.",
				Computed:            true,
			},
			"date_created": rschema.StringAttribute{
				MarkdownDescription: "Timestamp the connection was created.",
				Computed:            true,
			},
			"date_updated": rschema.StringAttribute{
				MarkdownDescription: "Timestamp the connection was last updated.",
				Computed:            true,
			},
		},
	}
}

func (r *sdkConnectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *sdkConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sdkConnectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := sdkConnectionToRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	conn, err := r.client.CreateSDKConnection(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create SDK connection", err.Error())
		return
	}

	state, diags := sdkConnectionFromAPI(ctx, conn, plan.Language.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sdkConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sdkConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	conn, err := r.client.GetSDKConnection(ctx, state.ID.ValueString())
	if err != nil {
		if growthbook.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read SDK connection", err.Error())
		return
	}

	newState, diags := sdkConnectionFromAPI(ctx, conn, state.Language.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *sdkConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sdkConnectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state sdkConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := sdkConnectionToRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	conn, err := r.client.UpdateSDKConnection(ctx, state.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update SDK connection", err.Error())
		return
	}

	newState, diags := sdkConnectionFromAPI(ctx, conn, plan.Language.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *sdkConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sdkConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteSDKConnection(ctx, state.ID.ValueString()); err != nil && !growthbook.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete SDK connection", err.Error())
	}
}

func (r *sdkConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resp.Diagnostics.AddError("Missing import id", fmt.Sprintf("%s import requires the SDK connection id.", req.ID))
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
