// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	growthbook "github.com/jz-wilson/growthbook-go"
)

// TestSavedGroupModelFromAPI_valuesNilVsEmpty checks that a nil API slice
// (field never returned/absent) and a non-nil empty API slice both round
// into a known, usable Set rather than an error, and that emptiness itself
// is preserved (a non-nil empty slice must not collapse to null, since
// Values is Computed and a caller relies on distinguishing "no values" from
// "unknown/absent" only at the request layer, not here).
func TestSavedGroupModelFromAPI_valuesNilVsEmpty(t *testing.T) {
	ctx := context.Background()

	nilGroup := &growthbook.SavedGroup{ID: "sg_1", Name: "n", Type: "list"}
	nilModel, diags := savedGroupModelFromAPI(ctx, nilGroup)
	if diags.HasError() {
		t.Fatalf("savedGroupModelFromAPI(nil values) diags = %v", diags)
	}
	var got []string
	if err := nilModel.Values.ElementsAs(ctx, &got, false); err != nil {
		t.Fatalf("ElementsAs: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Values from nil API slice = %v, want empty", got)
	}

	emptyGroup := &growthbook.SavedGroup{ID: "sg_1", Name: "n", Type: "list", Values: []string{}}
	emptyModel, diags := savedGroupModelFromAPI(ctx, emptyGroup)
	if diags.HasError() {
		t.Fatalf("savedGroupModelFromAPI(empty values) diags = %v", diags)
	}
	if emptyModel.Values.IsNull() {
		t.Errorf("Values from empty (non-nil) API slice is null, want a known empty set")
	}
}

// TestSavedGroupUpdateRequest_valuesNilVsEmpty is the request-layer half of
// the null-vs-empty contract: a null (unconfigured) Set must omit the field
// entirely (leave the server's value unchanged), while a known empty Set
// must send an explicit empty list (clear the server's value). Both are
// reachable from Terraform config: leaving `values` unset produces the
// former, `values = []` the latter.
func TestSavedGroupUpdateRequest_valuesNilVsEmpty(t *testing.T) {
	ctx := context.Background()

	unconfigured := SavedGroupModel{Name: types.StringValue("n"), Values: types.SetNull(types.StringType)}
	req, diags := savedGroupUpdateRequest(ctx, unconfigured)
	if diags.HasError() {
		t.Fatalf("savedGroupUpdateRequest(unconfigured) diags = %v", diags)
	}
	if req.Values != nil {
		t.Errorf("Values = %v, want nil (omitted) for an unconfigured/null set", req.Values)
	}

	cleared := SavedGroupModel{Name: types.StringValue("n"), Values: types.SetValueMust(types.StringType, nil)}
	req, diags = savedGroupUpdateRequest(ctx, cleared)
	if diags.HasError() {
		t.Fatalf("savedGroupUpdateRequest(cleared) diags = %v", diags)
	}
	if req.Values == nil || len(*req.Values) != 0 {
		t.Errorf("Values = %v, want a non-nil pointer to an empty slice for an explicit empty set", req.Values)
	}
}

// TestSavedGroupCreateRequest_typeAndAttributeKey checks that create sends
// type and attributeKey (required on create), which
// TestSavedGroupUpdateRequest_omitsTypeAndAttributeKey checks update never
// does (the update schema rejects both).
func TestSavedGroupCreateRequest_typeAndAttributeKey(t *testing.T) {
	ctx := context.Background()
	m := SavedGroupModel{
		Name:         types.StringValue("n"),
		Type:         types.StringValue("list"),
		AttributeKey: types.StringValue("userId"),
	}
	req, diags := savedGroupCreateRequest(ctx, m)
	if diags.HasError() {
		t.Fatalf("savedGroupCreateRequest diags = %v", diags)
	}
	if req.Type != "list" || req.AttributeKey != "userId" {
		t.Errorf("CreateRequest = %+v, want type=list attributeKey=userId", req)
	}
}

func TestSavedGroupUpdateRequest_omitsTypeAndAttributeKey(t *testing.T) {
	ctx := context.Background()
	m := SavedGroupModel{
		Name:         types.StringValue("n"),
		Type:         types.StringValue("list"),
		AttributeKey: types.StringValue("userId"),
	}
	req, diags := savedGroupUpdateRequest(ctx, m)
	if diags.HasError() {
		t.Fatalf("savedGroupUpdateRequest diags = %v", diags)
	}
	if req.Type != "" || req.AttributeKey != "" {
		t.Errorf("UpdateRequest = %+v, want type and attributeKey both empty (the update schema rejects them)", req)
	}
}
