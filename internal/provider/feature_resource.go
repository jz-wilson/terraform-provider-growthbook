// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	growthbook "github.com/jz-wilson/growthbook-go"
)

var (
	_ resource.Resource                = &featureResource{}
	_ resource.ResourceWithConfigure   = &featureResource{}
	_ resource.ResourceWithImportState = &featureResource{}
)

func NewFeatureResource() resource.Resource {
	return &featureResource{}
}

type featureResource struct {
	client *growthbook.Client
}

func (r *featureResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_feature"
}

func (r *featureResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *featureResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan featureModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := featureCreateRequest(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	feature, err := r.client.CreateFeature(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create GrowthBook feature", err.Error())
		return
	}
	resp.Diagnostics.Append(requireRulePrerequisitesPersisted(plan, feature)...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := featureModelFromAPI(ctx, feature, &resp.Diagnostics)
	echoUnmanagedCollections(&state, plan)
	reconcilePrerequisites(&state, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// requireRulePrerequisitesPersisted reports a clear error when GrowthBook
// silently drops rule-level prerequisites instead of storing them, which
// happens on any plan below Enterprise (the license gates rule-level
// "prerequisite-targeting" separately from feature-level "prerequisites",
// and a sub-Enterprise org's write succeeds but the value never lands).
// Without this check, the Plugin Framework instead reports a confusing
// "element 0 has vanished" consistency error.
func requireRulePrerequisitesPersisted(plan featureModel, feature *growthbook.Feature) diag.Diagnostics {
	var diags diag.Diagnostics
	for i, r := range plan.Rules {
		if len(r.Prerequisites) == 0 {
			continue
		}
		if i >= len(feature.Rules) || len(feature.Rules[i].Prerequisites) == 0 {
			diags.AddError(
				"GrowthBook did not store rule-level prerequisites",
				fmt.Sprintf("GrowthBook did not store rules[%d].prerequisites. Rule-level prerequisite targeting "+
					"requires an Enterprise plan (commercial feature \"prerequisite-targeting\").", i),
			)
		}
	}
	return diags
}

func (r *featureResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state featureModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	feature, err := r.client.GetFeature(ctx, state.ID.ValueString())
	if err != nil {
		if growthbook.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read GrowthBook feature", err.Error())
		return
	}

	newState := featureModelFromAPI(ctx, feature, &resp.Diagnostics)
	// rules and environments are unmanaged when absent from a prior state
	// that itself was never given them by config; preserve that instead of
	// forcing the API's current values onto a resource that doesn't own
	// them.
	if state.Rules == nil && len(newState.Rules) == 0 {
		newState.Rules = nil
	}
	if state.Environments == nil && len(newState.Environments) == 0 {
		newState.Environments = nil
	}
	if !state.Prerequisites.IsNull() && newState.Prerequisites.IsNull() {
		// state.Prerequisites was declared (possibly []); the API omits an
		// empty prerequisites array the same way it omits an absent one, so
		// preserve "declared but empty" instead of flipping it to null.
		newState.Prerequisites = emptyStringSet()
	}
	for i := range newState.Rules {
		if i >= len(state.Rules) {
			break
		}
		if state.Rules[i].Prerequisites != nil && newState.Rules[i].Prerequisites == nil {
			newState.Rules[i].Prerequisites = []prerequisiteModel{}
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *featureResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan featureModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := featureUpdateRequest(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	feature, err := r.client.UpdateFeature(ctx, plan.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update GrowthBook feature", err.Error())
		return
	}
	resp.Diagnostics.Append(requireRulePrerequisitesPersisted(plan, feature)...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := featureModelFromAPI(ctx, feature, &resp.Diagnostics)
	echoUnmanagedCollections(&state, plan)
	reconcilePrerequisites(&state, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// reconcilePrerequisites forces a declared-but-now-empty prerequisites list
// (feature-level or on a rule) to encode as [] rather than null in the
// returned state. GrowthBook's response omits an empty prerequisites array
// the same way an absent one is omitted, so a plan that explicitly cleared
// prerequisites (config sets [] rather than leaving the attribute unset)
// would otherwise read back as null and produce "inconsistent result after
// apply", since [] and null are distinct values for this Optional,
// non-Computed attribute.
func reconcilePrerequisites(state *featureModel, plan featureModel) {
	if !plan.Prerequisites.IsNull() && !plan.Prerequisites.IsUnknown() && state.Prerequisites.IsNull() {
		state.Prerequisites = emptyStringSet()
	}
	for i := range state.Rules {
		if i >= len(plan.Rules) {
			break
		}
		if plan.Rules[i].Prerequisites != nil && state.Rules[i].Prerequisites == nil {
			state.Rules[i].Prerequisites = []prerequisiteModel{}
		}
	}
}

// echoUnmanagedCollections keeps rules/environments null in the returned
// state whenever the plan (i.e. the config) never declared them: both are
// Optional, non-Computed attributes, so the framework requires the applied
// state to match the plan exactly, but GrowthBook's API always reports
// whatever rules/environments the feature currently has regardless of
// whether this request touched them.
func echoUnmanagedCollections(state *featureModel, plan featureModel) {
	if plan.Rules == nil {
		state.Rules = nil
	}
	if plan.Environments == nil {
		state.Environments = nil
	}
	if plan.Prerequisites.IsNull() {
		state.Prerequisites = types.SetNull(types.StringType)
	}
}

func (r *featureResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state featureModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	err := r.client.DeleteFeature(ctx, id)
	if err != nil && growthbook.IsArchiveRequired(err) {
		archived := true
		_, updateErr := r.client.UpdateFeature(ctx, id, growthbook.FeatureRequest{Archived: &archived})
		if updateErr != nil {
			resp.Diagnostics.AddError("Unable to archive GrowthBook feature before delete", updateErr.Error())
			return
		}
		err = r.client.DeleteFeature(ctx, id)
	}
	if err != nil && !growthbook.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete GrowthBook feature", err.Error())
	}
}

func (r *featureResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
