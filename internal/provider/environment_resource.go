// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	growthbook "github.com/jz-wilson/growthbook-go"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &environmentResource{}
	_ resource.ResourceWithConfigure   = &environmentResource{}
	_ resource.ResourceWithImportState = &environmentResource{}
)

// NewEnvironmentResource is the constructor registered with the provider.
func NewEnvironmentResource() resource.Resource {
	return &environmentResource{}
}

type environmentResource struct {
	client *growthbook.Client
}

// EnvironmentResourceModel is the Terraform data model for
// growthbook_environment.
type EnvironmentResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Description  types.String `tfsdk:"description"`
	ToggleOnList types.Bool   `tfsdk:"toggle_on_list"`
	DefaultState types.Bool   `tfsdk:"default_state"`
	Projects     types.Set    `tfsdk:"projects"`
	Parent       types.String `tfsdk:"parent"`
}

func (r *environmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (r *environmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a GrowthBook environment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Caller-chosen environment id, for example `production`. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Human-readable description of the environment.",
				Optional:            true,
				Computed:            true,
			},
			"toggle_on_list": schema.BoolAttribute{
				MarkdownDescription: "Whether this environment appears in the feature list toggle.",
				Optional:            true,
				Computed:            true,
			},
			"default_state": schema.BoolAttribute{
				MarkdownDescription: "Default enabled state for new features in this environment.",
				Optional:            true,
				Computed:            true,
			},
			"projects": schema.SetAttribute{
				MarkdownDescription: "Project ids this environment is scoped to. Omit or leave empty for all projects.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"parent": schema.StringAttribute{
				MarkdownDescription: "Id of the parent environment to clone rules from at creation time. Create-only: changing this forces a new resource.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *environmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *environmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := environmentRequestFromModel(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := r.client.CreateEnvironment(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create GrowthBook environment", err.Error())
		return
	}

	model, diags := environmentModelFromAPI(ctx, env)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *environmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := r.client.GetEnvironment(ctx, state.ID.ValueString())
	if err != nil {
		if growthbook.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read GrowthBook environment", err.Error())
		return
	}

	model, diags := environmentModelFromAPI(ctx, env)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *environmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan EnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := environmentRequestFromModel(ctx, plan, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := r.client.UpdateEnvironment(ctx, plan.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update GrowthBook environment", err.Error())
		return
	}

	model, diags := environmentModelFromAPI(ctx, env)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *environmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteEnvironment(ctx, state.ID.ValueString()); err != nil && !growthbook.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete GrowthBook environment", err.Error())
	}
}

func (r *environmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// environmentRequestFromModel builds the API request body from plan data.
// includeID controls whether the caller-chosen id travels in the body,
// which only Create needs; Update always drops it via the client.
func environmentRequestFromModel(ctx context.Context, m EnvironmentResourceModel, includeID bool) (growthbook.EnvironmentRequest, diag.Diagnostics) {
	var apiReq growthbook.EnvironmentRequest
	if includeID {
		apiReq.ID = m.ID.ValueString()
	}

	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		v := m.Description.ValueString()
		apiReq.Description = &v
	}
	if !m.ToggleOnList.IsNull() && !m.ToggleOnList.IsUnknown() {
		v := m.ToggleOnList.ValueBool()
		apiReq.ToggleOnList = &v
	}
	if !m.DefaultState.IsNull() && !m.DefaultState.IsUnknown() {
		v := m.DefaultState.ValueBool()
		apiReq.DefaultState = &v
	}
	if includeID && !m.Parent.IsNull() && !m.Parent.IsUnknown() {
		v := m.Parent.ValueString()
		apiReq.Parent = &v
	}

	// projects is Optional-only (not Computed): null means "no projects",
	// not "leave unconfigured". On Create that's the same thing as omitting
	// the field. On Update it is not: GrowthBook has no other signal for
	// "clear the list", so a null plan must still send an explicit empty
	// list, or removing the last configured project would never reach the
	// API and the resource would drift forever (or fail apply with an
	// inconsistent result once Read reports the old list back).
	var diags diag.Diagnostics
	isUpdate := !includeID
	if !m.Projects.IsUnknown() {
		if m.Projects.IsNull() {
			if isUpdate {
				empty := []string{}
				apiReq.Projects = &empty
			}
		} else {
			var projects []string
			diags.Append(m.Projects.ElementsAs(ctx, &projects, false)...)
			apiReq.Projects = &projects
		}
	}

	return apiReq, diags
}
