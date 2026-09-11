// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	growthbook "github.com/jz-wilson/growthbook-go"
)

// environmentModelFromAPI converts an API Environment into the resource's
// Terraform data model.
func environmentModelFromAPI(ctx context.Context, env *growthbook.Environment) (EnvironmentResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	projects, projDiags := environmentProjectsSetValue(ctx, env.Projects)
	diags.Append(projDiags...)

	model := EnvironmentResourceModel{
		ID:           types.StringValue(env.ID),
		Description:  types.StringValue(env.Description),
		ToggleOnList: types.BoolValue(env.ToggleOnList),
		DefaultState: types.BoolValue(env.DefaultState),
		Projects:     projects,
	}
	if env.Parent == "" {
		model.Parent = types.StringNull()
	} else {
		model.Parent = types.StringValue(env.Parent)
	}

	return model, diags
}

// environmentProjectsSetValue normalizes the API's "projects" list into a
// set value that is null exactly when the list is empty, regardless of
// whether GrowthBook represented "no projects" as a missing field, a JSON
// null, or an empty array on a given call. GrowthBook is inconsistent about
// which of those it returns for the same environment across calls (observed
// on a live instance's "production" environment: an import produced a nil
// slice, a later read after an update produced a non-nil empty slice). Since
// "projects" is Optional+Computed, an unconfigured plan simply carries the
// prior state value forward; if Create/Read/Update ever disagree on null vs.
// empty-set for "no projects" the provider trips Terraform's
// inconsistent-result-after-apply check. Collapsing both to null here keeps
// every call site consistent.
func environmentProjectsSetValue(ctx context.Context, projects []string) (types.Set, diag.Diagnostics) {
	if len(projects) == 0 {
		return types.SetNull(types.StringType), nil
	}
	return types.SetValueFrom(ctx, types.StringType, projects)
}

// environmentDataSourceModelFromAPI converts an API Environment into the
// data source's Terraform data model (no plan-modifier concerns, so parent
// and description are always plain values).
func environmentDataSourceModelFromAPI(ctx context.Context, env growthbook.Environment) (EnvironmentDataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	projects, projDiags := environmentProjectsSetValue(ctx, env.Projects)
	diags.Append(projDiags...)

	model := EnvironmentDataSourceModel{
		ID:           types.StringValue(env.ID),
		Description:  types.StringValue(env.Description),
		ToggleOnList: types.BoolValue(env.ToggleOnList),
		DefaultState: types.BoolValue(env.DefaultState),
		Projects:     projects,
		Parent:       types.StringValue(env.Parent),
	}

	return model, diags
}
