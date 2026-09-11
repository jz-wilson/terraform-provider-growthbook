// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	growthbook "github.com/jz-wilson/growthbook-go"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &ProjectResource{}
	_ resource.ResourceWithImportState = &ProjectResource{}
)

// NewProjectResource is the constructor registered with the provider.
func NewProjectResource() resource.Resource {
	return &ProjectResource{}
}

// ProjectResource manages a GrowthBook project.
type ProjectResource struct {
	client *growthbook.Client
}

// ProjectSettingsModel is the "settings" nested attribute.
type ProjectSettingsModel struct {
	StatsEngine     types.String  `tfsdk:"stats_engine"`
	ConfidenceLevel types.Float64 `tfsdk:"confidence_level"`
	PValueThreshold types.Float64 `tfsdk:"p_value_threshold"`
}

// ProjectResourceModel maps the resource schema to Go values.
type ProjectResourceModel struct {
	ID             types.String          `tfsdk:"id"`
	Name           types.String          `tfsdk:"name"`
	Description    types.String          `tfsdk:"description"`
	PublicID       types.String          `tfsdk:"public_id"`
	RestrictAccess types.Bool            `tfsdk:"restrict_access"`
	Settings       *ProjectSettingsModel `tfsdk:"settings"`
	DateCreated    types.String          `tfsdk:"date_created"`
	DateUpdated    types.String          `tfsdk:"date_updated"`
}

func (r *ProjectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *ProjectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a GrowthBook project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "GrowthBook project id (`prj_...`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name of the project.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Description of the project. Omitting this from config leaves whatever GrowthBook currently holds untouched (useful for imported projects); set it to `\"\"` explicitly to clear it.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"public_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Stable identifier used in feature flag payloads. GrowthBook derives one from `name` when left unset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"restrict_access": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Restrict this project to only members explicitly granted access. Omitting this from config leaves whatever GrowthBook currently holds untouched.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"settings": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Per-project statistics settings overriding the organization defaults. Left unset, GrowthBook reports the organization defaults here once the project is created.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"stats_engine": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Statistics engine, e.g. `bayesian` or `frequentist`.",
					},
					"confidence_level": schema.Float64Attribute{
						Optional:            true,
						MarkdownDescription: "Confidence level required to call a winner.",
					},
					"p_value_threshold": schema.Float64Attribute{
						Optional:            true,
						MarkdownDescription: "P-value threshold for statistical significance.",
					},
				},
			},
			"date_created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp the project was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"date_updated": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp the project was last updated. Recomputed by GrowthBook on every update, so plans against this attribute are never guaranteed empty.",
			},
		},
	}
}

func (r *ProjectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *ProjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProjectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := r.client.CreateProject(ctx, projectRequestFromModel(data))
	if err != nil {
		resp.Diagnostics.AddError("Unable to create GrowthBook project", err.Error())
		return
	}

	modelFromProject(&data, project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := r.client.GetProject(ctx, data.ID.ValueString())
	if err != nil {
		if growthbook.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read GrowthBook project", err.Error())
		return
	}

	modelFromProject(&data, project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProjectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state ProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := r.client.UpdateProject(ctx, state.ID.ValueString(), projectRequestFromModel(data))
	if err != nil {
		resp.Diagnostics.AddError("Unable to update GrowthBook project", err.Error())
		return
	}

	data.ID = state.ID
	modelFromProject(&data, project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteProject(ctx, data.ID.ValueString()); err != nil && !growthbook.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete GrowthBook project", err.Error())
	}
}

func (r *ProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// projectRequestFromModel builds a ProjectRequest sending only the fields
// configured on the model, so Update never clobbers unconfigured fields.
func projectRequestFromModel(data ProjectResourceModel) growthbook.ProjectRequest {
	req := growthbook.ProjectRequest{Name: data.Name.ValueString()}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		req.Description = &v
	}
	if !data.PublicID.IsNull() && !data.PublicID.IsUnknown() {
		v := data.PublicID.ValueString()
		req.PublicID = &v
	}
	if !data.RestrictAccess.IsNull() && !data.RestrictAccess.IsUnknown() {
		v := data.RestrictAccess.ValueBool()
		req.RestrictAccess = &v
	}
	if data.Settings != nil {
		s := &growthbook.ProjectSettings{}
		if !data.Settings.StatsEngine.IsNull() && !data.Settings.StatsEngine.IsUnknown() {
			v := data.Settings.StatsEngine.ValueString()
			s.StatsEngine = &v
		}
		if !data.Settings.ConfidenceLevel.IsNull() && !data.Settings.ConfidenceLevel.IsUnknown() {
			v := data.Settings.ConfidenceLevel.ValueFloat64()
			s.ConfidenceLevel = &v
		}
		if !data.Settings.PValueThreshold.IsNull() && !data.Settings.PValueThreshold.IsUnknown() {
			v := data.Settings.PValueThreshold.ValueFloat64()
			s.PValueThreshold = &v
		}
		req.Settings = s
	}
	return req
}

// modelFromProject copies the API representation into the Terraform model.
func modelFromProject(data *ProjectResourceModel, project *growthbook.Project) {
	data.ID = types.StringValue(project.ID)
	data.Name = types.StringValue(project.Name)
	// Always a known, non-null value (never types.StringNull()): description
	// is Optional+Computed with UseStateForUnknown, so an explicit "" in
	// config must round-trip to exactly "" here, not null, or the framework
	// reports "provider produced inconsistent result after apply".
	data.Description = types.StringValue(project.Description)
	data.PublicID = types.StringValue(project.PublicID)
	data.RestrictAccess = types.BoolPointerValue(project.RestrictAccess)
	data.DateCreated = stringOrNull(project.DateCreated)
	data.DateUpdated = stringOrNull(project.DateUpdated)

	if project.Settings == nil {
		data.Settings = nil
		return
	}
	data.Settings = &ProjectSettingsModel{
		StatsEngine:     stringPtrOrNull(project.Settings.StatsEngine),
		ConfidenceLevel: types.Float64PointerValue(project.Settings.ConfidenceLevel),
		PValueThreshold: types.Float64PointerValue(project.Settings.PValueThreshold),
	}
}

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func stringPtrOrNull(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}
