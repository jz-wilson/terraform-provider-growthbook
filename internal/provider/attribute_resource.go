// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                     = &attributeResource{}
	_ resource.ResourceWithConfigure        = &attributeResource{}
	_ resource.ResourceWithConfigValidators = &attributeResource{}
	_ resource.ResourceWithImportState      = &attributeResource{}
)

// attributeDatatypes are the datatype enum values the API accepts.
var attributeDatatypes = []string{"boolean", "string", "number", "secureString", "enum", "string[]", "number[]", "secureString[]"}

// attributeFormats are the format enum values the API accepts. "" (unset) is
// valid, so it is not listed here; OneOf only constrains non-null values.
var attributeFormats = []string{"version", "date", "isoCountryCode"}

// NewAttributeResource is the constructor registered with the provider.
func NewAttributeResource() resource.Resource {
	return &attributeResource{}
}

type attributeResource struct {
	client *growthbook.Client
}

// AttributeResourceModel is the Terraform data model for growthbook_attribute.
type AttributeResourceModel struct {
	Property      types.String `tfsdk:"property"`
	Datatype      types.String `tfsdk:"datatype"`
	Description   types.String `tfsdk:"description"`
	HashAttribute types.Bool   `tfsdk:"hash_attribute"`
	Archived      types.Bool   `tfsdk:"archived"`
	Enum          types.String `tfsdk:"enum"`
	Format        types.String `tfsdk:"format"`
	Projects      types.Set    `tfsdk:"projects"`
	Tags          types.Set    `tfsdk:"tags"`
}

func (r *attributeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_attribute"
}

func (r *attributeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a GrowthBook SDK targeting attribute.",
		Attributes: map[string]schema.Attribute{
			"property": schema.StringAttribute{
				MarkdownDescription: "Attribute property name, and its identifier. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"datatype": schema.StringAttribute{
				MarkdownDescription: "Attribute datatype: one of `boolean`, `string`, `number`, `secureString`, `enum`, `string[]`, `number[]`, `secureString[]`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(attributeDatatypes...),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the attribute. Omitting this from config leaves whatever GrowthBook currently holds untouched; set it to `\"\"` explicitly to clear it.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"hash_attribute": schema.BoolAttribute{
				MarkdownDescription: "Whether this attribute is hashed before being sent to the SDK.",
				Optional:            true,
				Computed:            true,
			},
			"archived": schema.BoolAttribute{
				MarkdownDescription: "Whether this attribute is archived.",
				Optional:            true,
				Computed:            true,
			},
			"enum": schema.StringAttribute{
				MarkdownDescription: "Comma-separated list of allowed values. Required when `datatype` is `enum`; optionally restricts `string[]`/`number[]`/`secureString[]`; ignored for every other datatype.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"format": schema.StringAttribute{
				MarkdownDescription: "Attribute format: one of `version`, `date`, `isoCountryCode`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(attributeFormats...),
				},
			},
			"projects": schema.SetAttribute{
				MarkdownDescription: "Project ids this attribute is scoped to. Omit or leave empty for all projects.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"tags": schema.SetAttribute{
				MarkdownDescription: "Tags applied to this attribute.",
				ElementType:         types.StringType,
				Optional:            true,
			},
		},
	}
}

// ConfigValidators enforces the API's own rule (see the "enum" field
// description in the OpenAPI spec) that "enum" is required when datatype is
// "enum", so a bad config fails fast in `terraform plan` rather than a 400
// from the API during apply.
func (r *attributeResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		&attributeEnumRequiredValidator{},
	}
}

func (r *attributeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *attributeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AttributeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := attributeRequestFromModel(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	attr, err := r.client.CreateAttribute(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create GrowthBook attribute", err.Error())
		return
	}

	model, diags := attributeModelFromAPI(ctx, attr)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *attributeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AttributeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attr, err := r.client.GetAttribute(ctx, state.Property.ValueString())
	if err != nil {
		if growthbook.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
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

func (r *attributeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AttributeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := attributeRequestFromModel(ctx, plan, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	attr, err := r.client.UpdateAttribute(ctx, plan.Property.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update GrowthBook attribute", err.Error())
		return
	}

	model, diags := attributeModelFromAPI(ctx, attr)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *attributeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AttributeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteAttribute(ctx, state.Property.ValueString()); err != nil && !growthbook.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete GrowthBook attribute", err.Error())
	}
}

func (r *attributeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resp.Diagnostics.AddError("Missing import id", fmt.Sprintf("%s import requires the attribute property.", req.ID))
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("property"), req, resp)
}

// attributeRequestFromModel builds the API request body from plan data.
// includeProperty controls whether the caller-chosen property travels in the
// body, which only Create needs: the PUT schema rejects it outright.
func attributeRequestFromModel(ctx context.Context, m AttributeResourceModel, includeProperty bool) (growthbook.AttributeRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	apiReq := growthbook.AttributeRequest{Datatype: m.Datatype.ValueString()}
	if includeProperty {
		apiReq.Property = m.Property.ValueString()
	}

	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		v := m.Description.ValueString()
		apiReq.Description = &v
	}
	if !m.HashAttribute.IsNull() && !m.HashAttribute.IsUnknown() {
		v := m.HashAttribute.ValueBool()
		apiReq.HashAttribute = &v
	}
	if !m.Archived.IsNull() && !m.Archived.IsUnknown() {
		v := m.Archived.ValueBool()
		apiReq.Archived = &v
	}
	if !m.Enum.IsNull() && !m.Enum.IsUnknown() {
		v := m.Enum.ValueString()
		apiReq.Enum = &v
	}
	if !m.Format.IsNull() && !m.Format.IsUnknown() {
		v := m.Format.ValueString()
		apiReq.Format = &v
	}

	// projects/tags are Optional-only (not Computed): null means "no
	// projects/tags", not "leave unconfigured". On Create that's the same
	// thing as omitting the field. On Update it is not: GrowthBook has no
	// other signal for "clear the list", so a null plan must still send an
	// explicit empty list, or removing the last configured value from a
	// list would never reach the API and the resource would drift forever.
	isUpdate := !includeProperty
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
	if !m.Tags.IsUnknown() {
		if m.Tags.IsNull() {
			if isUpdate {
				empty := []string{}
				apiReq.Tags = &empty
			}
		} else {
			var tags []string
			diags.Append(m.Tags.ElementsAs(ctx, &tags, false)...)
			apiReq.Tags = &tags
		}
	}

	return apiReq, diags
}
