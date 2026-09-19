// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	growthbook "github.com/jz-wilson/growthbook-go"
)

// attributeModelFromAPI converts an API Attribute into the resource's
// Terraform data model.
func attributeModelFromAPI(ctx context.Context, attr *growthbook.Attribute) (AttributeResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	projects, projDiags := attributeSetValue(ctx, attr.Projects)
	diags.Append(projDiags...)
	tags, tagDiags := attributeSetValue(ctx, attr.Tags)
	diags.Append(tagDiags...)

	model := AttributeResourceModel{
		Property:      types.StringValue(attr.Property),
		Datatype:      types.StringValue(attr.Datatype),
		Description:   types.StringValue(attr.Description),
		HashAttribute: types.BoolValue(attr.HashAttribute),
		Archived:      types.BoolValue(attr.Archived),
		Enum:          types.StringValue(attr.Enum),
		Format:        types.StringValue(attr.Format),
		Projects:      projects,
		Tags:          tags,
	}

	return model, diags
}

// attributeSetValue normalizes the API's list fields into a set value that
// is null exactly when the list is empty, matching the same
// null-vs-empty-array normalization environmentProjectsSetValue applies
// (see its doc comment for why: GrowthBook is inconsistent about which shape
// it returns for "no values" across calls, and Optional-only attributes here
// have no plan modifier to paper over a mismatch).
func attributeSetValue(ctx context.Context, values []string) (types.Set, diag.Diagnostics) {
	if len(values) == 0 {
		return types.SetNull(types.StringType), nil
	}
	return types.SetValueFrom(ctx, types.StringType, values)
}

// attributeEnumRequiredValidator enforces the API's rule (see the "enum"
// field description in the OpenAPI spec) that "enum" is required when
// datatype is "enum".
type attributeEnumRequiredValidator struct{}

func (v *attributeEnumRequiredValidator) Description(_ context.Context) string {
	return "enum must be set when datatype is \"enum\""
}

func (v *attributeEnumRequiredValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v *attributeEnumRequiredValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data AttributeResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Datatype.IsUnknown() || data.Datatype.ValueString() != "enum" {
		return
	}
	if data.Enum.IsUnknown() {
		return
	}
	if data.Enum.IsNull() || data.Enum.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("enum"),
			"Missing required attribute",
			`"enum" must be set to a comma-separated list of allowed values when datatype is "enum".`,
		)
	}
}
