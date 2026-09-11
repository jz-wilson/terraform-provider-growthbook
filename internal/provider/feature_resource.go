// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"

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

	state := featureModelFromAPI(ctx, feature, &resp.Diagnostics)
	echoUnmanagedCollections(&state, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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
	if state.Environments == nil {
		newState.Environments = nil
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

	state := featureModelFromAPI(ctx, feature, &resp.Diagnostics)
	echoUnmanagedCollections(&state, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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
