// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	growthbook "github.com/jz-wilson/growthbook-go"
)

// SavedGroupModel maps the growthbook_saved_group resource and data source
// schemas to Go values.
type SavedGroupModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Type         types.String `tfsdk:"type"`
	Condition    types.String `tfsdk:"condition"`
	AttributeKey types.String `tfsdk:"attribute_key"`
	Values       types.Set    `tfsdk:"values"`
	Owner        types.String `tfsdk:"owner"`
	OwnerEmail   types.String `tfsdk:"owner_email"`
	Description  types.String `tfsdk:"description"`
	Projects     types.Set    `tfsdk:"projects"`
	Archived     types.Bool   `tfsdk:"archived"`
	UseEmptyList types.Bool   `tfsdk:"use_empty_list_group"`
	DateCreated  types.String `tfsdk:"date_created"`
	DateUpdated  types.String `tfsdk:"date_updated"`
}

// savedGroupModelFromAPI converts an API SavedGroup into the Terraform model.
func savedGroupModelFromAPI(ctx context.Context, g *growthbook.SavedGroup) (SavedGroupModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	values, d := types.SetValueFrom(ctx, types.StringType, g.Values)
	diags.Append(d...)
	projects, d := types.SetValueFrom(ctx, types.StringType, g.Projects)
	diags.Append(d...)

	model := SavedGroupModel{
		ID:           types.StringValue(g.ID),
		Name:         types.StringValue(g.Name),
		Type:         types.StringValue(g.Type),
		Condition:    stringOrNull(g.Condition),
		AttributeKey: stringOrNull(g.AttributeKey),
		Values:       values,
		Owner:        stringOrNull(g.Owner),
		OwnerEmail:   stringOrNull(g.OwnerEmail),
		Description:  types.StringValue(g.Description),
		Projects:     projects,
		Archived:     boolPtrToType(g.Archived),
		UseEmptyList: boolPtrToType(g.UseEmptyList),
		DateCreated:  types.StringValue(g.DateCreated),
		DateUpdated:  types.StringValue(g.DateUpdated),
	}
	return model, diags
}

// savedGroupCreateRequest builds the request for POST /v1/saved-groups. Type
// and attributeKey are create-only: the update request schema rejects them
// outright (additionalProperties: false), since the API infers type from the
// group's existing fields and attributeKey never changes after creation.
func savedGroupCreateRequest(ctx context.Context, m SavedGroupModel) (growthbook.SavedGroupRequest, diag.Diagnostics) {
	req, diags := savedGroupUpdateRequest(ctx, m)
	req.Type = m.Type.ValueString()
	req.AttributeKey = m.AttributeKey.ValueString()
	return req, diags
}

// savedGroupUpdateRequest builds the request for POST /v1/saved-groups/{id},
// sending only fields configured on the model so an update never clobbers an
// unconfigured field. values/projects use *[]string so a configured empty
// set is distinguishable from an unconfigured (omitted) one: only the latter
// leaves the field out of the request entirely.
func savedGroupUpdateRequest(ctx context.Context, m SavedGroupModel) (growthbook.SavedGroupRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := growthbook.SavedGroupRequest{
		Name:      m.Name.ValueString(),
		Condition: stringPtrOrNil(m.Condition),
		Owner:     stringPtrOrNil(m.Owner),
	}

	if !m.Values.IsNull() && !m.Values.IsUnknown() {
		var values []string
		diags.Append(m.Values.ElementsAs(ctx, &values, false)...)
		req.Values = &values
	}
	if !m.Projects.IsNull() && !m.Projects.IsUnknown() {
		var projects []string
		diags.Append(m.Projects.ElementsAs(ctx, &projects, false)...)
		req.Projects = &projects
	}

	return req, diags
}
