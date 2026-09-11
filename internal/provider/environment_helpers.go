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

	projects, projDiags := types.SetValueFrom(ctx, types.StringType, env.Projects)
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

// environmentDataSourceModelFromAPI converts an API Environment into the
// data source's Terraform data model (no plan-modifier concerns, so parent
// and description are always plain values).
func environmentDataSourceModelFromAPI(ctx context.Context, env growthbook.Environment) (EnvironmentDataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	projects, projDiags := types.SetValueFrom(ctx, types.StringType, env.Projects)
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
