// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	growthbook "github.com/jz-wilson/growthbook-go"
)

func TestAttributeModelFromAPI_emptyListsAreNull(t *testing.T) {
	attr := &growthbook.Attribute{Property: "plan_tier", Datatype: "string"}

	model, diags := attributeModelFromAPI(context.Background(), attr)
	if diags.HasError() {
		t.Fatalf("attributeModelFromAPI() diags = %v", diags)
	}
	if !model.Projects.IsNull() {
		t.Errorf("Projects = %v, want null for an empty API list", model.Projects)
	}
	if !model.Tags.IsNull() {
		t.Errorf("Tags = %v, want null for an empty API list", model.Tags)
	}
	// Description/Enum/Format are Optional+Computed plain strings (not
	// pointers) in the API response, so an empty API value must round-trip
	// to exactly "" here, not null, or the framework reports "provider
	// produced inconsistent result after apply" against a config that left
	// them unset.
	if model.Description.IsNull() || model.Description.ValueString() != "" {
		t.Errorf("Description = %v, want empty string, not null", model.Description)
	}
	if model.Enum.IsNull() || model.Enum.ValueString() != "" {
		t.Errorf("Enum = %v, want empty string, not null", model.Enum)
	}
}

func TestAttributeModelFromAPI_populatedLists(t *testing.T) {
	attr := &growthbook.Attribute{
		Property: "plan_tier",
		Datatype: "string",
		Projects: []string{"proj_1"},
		Tags:     []string{"billing"},
	}

	model, diags := attributeModelFromAPI(context.Background(), attr)
	if diags.HasError() {
		t.Fatalf("attributeModelFromAPI() diags = %v", diags)
	}
	var projects []string
	diags = model.Projects.ElementsAs(context.Background(), &projects, false)
	if diags.HasError() || len(projects) != 1 || projects[0] != "proj_1" {
		t.Errorf("Projects = %v, want [proj_1]", projects)
	}
}

func TestAttributeRequestFromModel_createIncludesPropertyAndDatatype(t *testing.T) {
	model := AttributeResourceModel{
		Property: types.StringValue("plan_tier"),
		Datatype: types.StringValue("string"),
		// Every Optional+Computed attribute left unset by config plans as
		// null (Optional-only) or unknown (Optional+Computed without a
		// UseStateForUnknown modifier applying); model these as null to
		// simulate an unconfigured plan.
		Description:   types.StringNull(),
		HashAttribute: types.BoolNull(),
		Archived:      types.BoolNull(),
		Enum:          types.StringNull(),
		Format:        types.StringNull(),
		Projects:      types.SetNull(types.StringType),
		Tags:          types.SetNull(types.StringType),
	}

	req, diags := attributeRequestFromModel(context.Background(), model, true)
	if diags.HasError() {
		t.Fatalf("attributeRequestFromModel() diags = %v", diags)
	}
	if req.Property != "plan_tier" || req.Datatype != "string" {
		t.Errorf("req = %+v, want property/datatype set", req)
	}
	if req.Description != nil || req.HashAttribute != nil || req.Archived != nil || req.Enum != nil || req.Format != nil {
		t.Errorf("req = %+v, want every unconfigured optional field nil", req)
	}
	if req.Projects != nil || req.Tags != nil {
		t.Errorf("req = %+v, want nil Projects/Tags for an unconfigured (null) set, not a pointer to an empty slice", req)
	}
}

func TestAttributeRequestFromModel_updateOmitsProperty(t *testing.T) {
	model := AttributeResourceModel{
		Property: types.StringValue("plan_tier"),
		Datatype: types.StringValue("string"),
	}

	req, diags := attributeRequestFromModel(context.Background(), model, false)
	if diags.HasError() {
		t.Fatalf("attributeRequestFromModel() diags = %v", diags)
	}
	if req.Property != "" {
		t.Errorf("req.Property = %q, want empty on update (PUT schema rejects it)", req.Property)
	}
}

func TestAttributeRequestFromModel_configuredEmptySetClearsList(t *testing.T) {
	empty, diags := types.SetValueFrom(context.Background(), types.StringType, []string{})
	if diags.HasError() {
		t.Fatalf("SetValueFrom() diags = %v", diags)
	}

	model := AttributeResourceModel{
		Property: types.StringValue("plan_tier"),
		Datatype: types.StringValue("string"),
		Projects: empty,
		Tags:     empty,
	}

	req, diags := attributeRequestFromModel(context.Background(), model, false)
	if diags.HasError() {
		t.Fatalf("attributeRequestFromModel() diags = %v", diags)
	}
	if req.Projects == nil || len(*req.Projects) != 0 {
		t.Errorf("req.Projects = %v, want a non-nil pointer to an empty slice to clear the list", req.Projects)
	}
	if req.Tags == nil || len(*req.Tags) != 0 {
		t.Errorf("req.Tags = %v, want a non-nil pointer to an empty slice to clear the list", req.Tags)
	}
}
