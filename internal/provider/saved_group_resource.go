// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	growthbook "github.com/jz-wilson/growthbook-go"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &SavedGroupResource{}
	_ resource.ResourceWithImportState = &SavedGroupResource{}
)

// savedGroupTypes are the saved group types the GrowthBook API accepts.
var savedGroupTypes = []string{"condition", "list"}

// NewSavedGroupResource is the constructor registered with the provider.
func NewSavedGroupResource() resource.Resource {
	return &SavedGroupResource{}
}

// SavedGroupResource manages a GrowthBook saved group.
type SavedGroupResource struct {
	client *growthbook.Client
}

func (r *SavedGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_saved_group"
}

func (r *SavedGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a GrowthBook saved group, a reusable list of attribute values or a condition that feature rules can target. Destroy archives the group before deleting it (GrowthBook requires this) and fails with the API's HTTP 422 if the group is still referenced by a feature, experiment, or another saved group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "GrowthBook saved group id.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name of the saved group.",
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Saved group type: `condition` (targets by a JSON condition) or `list` (targets by a list of values for one attribute). Immutable after creation.",
				Validators:          []validator.String{stringvalidator.OneOf(savedGroupTypes...)},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"condition": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "JSON-encoded condition for the group. Required when `type = \"condition\"`, and rejected when `type = \"list\"`.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("values"), path.MatchRoot("attribute_key")),
				},
			},
			"attribute_key": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Attribute key the group's list of values is based on. Required when `type = \"list\"`, and rejected when `type = \"condition\"`. Immutable after creation. Must reference an attribute that already exists in the organization (see `growthbook_attribute`); the API returns HTTP 400 (\"Unknown attributeKey\") otherwise.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("condition")),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"values": schema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "List of attribute values for `type = \"list\"`. Omit to leave unchanged; set to `[]` to clear.",
			},
			"owner": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The userId or email address of the owner. Defaults to the user associated with the request's Personal Access Token when omitted.",
			},
			"owner_email": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Email address of the owner, when it can be resolved to a known user.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of the saved group. Not settable through this API; GrowthBook does not accept it on create or update.",
			},
			"projects": schema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Project ids this saved group is scoped to. Omit to leave unchanged; set to `[]` to clear (all projects).",
			},
			"archived": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the saved group is archived. Not settable through this resource.",
			},
			"use_empty_list_group": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "GrowthBook-managed flag for empty list-group handling.",
			},
			"date_created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp the saved group was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"date_updated": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp the saved group was last updated.",
			},
		},
	}
}

func (r *SavedGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *SavedGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SavedGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := savedGroupCreateRequest(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := r.client.CreateSavedGroup(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create GrowthBook saved group", err.Error())
		return
	}

	model, diags := savedGroupModelFromAPI(ctx, group)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *SavedGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SavedGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := r.client.GetSavedGroup(ctx, data.ID.ValueString())
	if err != nil {
		if growthbook.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read GrowthBook saved group", err.Error())
		return
	}

	model, diags := savedGroupModelFromAPI(ctx, group)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *SavedGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SavedGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state SavedGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := savedGroupUpdateRequest(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := r.client.UpdateSavedGroup(ctx, state.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update GrowthBook saved group", err.Error())
		return
	}

	model, diags := savedGroupModelFromAPI(ctx, group)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// Delete archives the saved group before deleting it: GrowthBook refuses
// DELETE /v1/saved-groups/{id} on a group that isn't archived first (HTTP
// 400). If the group is already gone, the archive call itself reports
// IsNotFound and delete is skipped. Any other archive error (for example,
// already archived) is not fatal here; if it actually blocked archiving
// (HTTP 422, the group is still referenced by a feature, experiment, or
// another saved group), the delete call below surfaces that as its own
// error.
func (r *SavedGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SavedGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := data.ID.ValueString()
	if _, err := r.client.ArchiveSavedGroup(ctx, id); err != nil && growthbook.IsNotFound(err) {
		return
	}

	if err := r.client.DeleteSavedGroup(ctx, id); err != nil && !growthbook.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete GrowthBook saved group", err.Error())
	}
}

func (r *SavedGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
