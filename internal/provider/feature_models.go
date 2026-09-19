// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	growthbook "github.com/jz-wilson/growthbook-go"
)

// savedGroupModel is one entry of a rule's saved_groups list.
type savedGroupModel struct {
	Match types.String `tfsdk:"match"`
	IDs   types.Set    `tfsdk:"ids"`
}

// variationModel is one entry of an experiment-ref rule's variations list.
type variationModel struct {
	VariationID types.String `tfsdk:"variation_id"`
	Value       types.String `tfsdk:"value"`
}

// prerequisiteModel is one entry of a rule's prerequisites list: it gates
// the rule on another feature (ID) matching a condition evaluated against
// that parent feature's value. Feature-level prerequisites have no
// condition (see featureModel.Prerequisites) - only rule-level ones do.
type prerequisiteModel struct {
	ID        types.String `tfsdk:"id"`
	Condition types.String `tfsdk:"condition"`
}

// scheduleRuleModel is one on/off transition in a rule's schedule. A null
// Timestamp is an open-ended transition.
type scheduleRuleModel struct {
	Enabled   types.Bool   `tfsdk:"enabled"`
	Timestamp types.String `tfsdk:"timestamp"`
}

// ruleModel is one entry of a feature's ordered rules list.
type ruleModel struct {
	Type            types.String        `tfsdk:"type"`
	Description     types.String        `tfsdk:"description"`
	Enabled         types.Bool          `tfsdk:"enabled"`
	Condition       types.String        `tfsdk:"condition"`
	SavedGroups     []savedGroupModel   `tfsdk:"saved_groups"`
	Prerequisites   []prerequisiteModel `tfsdk:"prerequisites"`
	AllEnvironments types.Bool          `tfsdk:"all_environments"`
	Environments    types.Set           `tfsdk:"environments"`
	Value           types.String        `tfsdk:"value"`
	Coverage        types.Float64       `tfsdk:"coverage"`
	HashAttribute   types.String        `tfsdk:"hash_attribute"`
	ExperimentID    types.String        `tfsdk:"experiment_id"`
	Variations      []variationModel    `tfsdk:"variations"`
	RuleID          types.String        `tfsdk:"rule_id"`
	ScheduleType    types.String        `tfsdk:"schedule_type"`
	ScheduleRules   []scheduleRuleModel `tfsdk:"schedule_rules"`
}

// environmentModel is one entry of a feature's environments map.
type environmentModel struct {
	Enabled types.Bool `tfsdk:"enabled"`
}

// featureModel is the shared Terraform data model for the growthbook_feature
// resource and data source.
type featureModel struct {
	ID           types.String                `tfsdk:"id"`
	ValueType    types.String                `tfsdk:"value_type"`
	DefaultValue types.String                `tfsdk:"default_value"`
	Description  types.String                `tfsdk:"description"`
	Project      types.String                `tfsdk:"project"`
	Owner        types.String                `tfsdk:"owner"`
	Tags         types.Set                   `tfsdk:"tags"`
	Archived     types.Bool                  `tfsdk:"archived"`
	Environments map[string]environmentModel `tfsdk:"environments"`
	Rules        []ruleModel                 `tfsdk:"rules"`
	// Prerequisites is feature-level: a set of other features' IDs, each
	// of which must evaluate to true. Unlike rules[].prerequisites, there
	// is no per-entry condition here (see growthbook-go's Feature.
	// Prerequisites / the GrowthBook OpenAPI spec).
	Prerequisites   types.Set    `tfsdk:"prerequisites"`
	RevisionVersion types.Int64  `tfsdk:"revision_version"`
	DateCreated     types.String `tfsdk:"date_created"`
	DateUpdated     types.String `tfsdk:"date_updated"`
}

// stringSetValue builds a types.Set of strings, returning a null set for a
// nil/empty input so unset optional fields round-trip as null rather than
// an empty collection.
func stringSetValue(ctx context.Context, values []string, diags *diag.Diagnostics) types.Set {
	// GrowthBook is inconsistent about whether an unset set-valued field
	// comes back as a JSON null (decoded to a nil slice) or an empty JSON
	// array (decoded to a non-nil, zero-length slice); both count as
	// "unset" for our Optional, non-Computed set attributes.
	if len(values) == 0 {
		return types.SetNull(types.StringType)
	}
	set, d := types.SetValueFrom(ctx, types.StringType, values)
	diags.Append(d...)
	return set
}

// emptyStringSet returns a known, empty types.Set of strings, distinct from
// types.SetNull(types.StringType).
func emptyStringSet() types.Set {
	return types.SetValueMust(types.StringType, nil)
}

func stringSetToSlice(ctx context.Context, set types.Set, diags *diag.Diagnostics) []string {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(set.ElementsAs(ctx, &out, false)...)
	return out
}

// prerequisitesToAPI converts a rule's or feature's prerequisites list into
// the wire representation. Unlike rulesToAPI/featurePrerequisitesToAPI, this
// returns a plain (possibly nil) slice: it backs ruleModel.Prerequisites,
// which is embedded in a rule that's always replaced wholesale, so there is
// no separate "leave unmanaged" state to preserve at this level.
func prerequisitesToAPI(prereqs []prerequisiteModel) []growthbook.FeaturePrerequisite {
	if prereqs == nil {
		return nil
	}
	out := make([]growthbook.FeaturePrerequisite, 0, len(prereqs))
	for _, p := range prereqs {
		out = append(out, growthbook.FeaturePrerequisite{
			ID:        p.ID.ValueString(),
			Condition: p.Condition.ValueString(),
		})
	}
	return out
}

func prerequisitesFromAPI(prereqs []growthbook.FeaturePrerequisite) []prerequisiteModel {
	// Treat an empty response array the same as an absent one: GrowthBook's
	// live API always sends "prerequisites" (as [] when a rule/feature has
	// none), unlike this package's own fake test server, which omits the
	// field entirely when empty. Normalizing both to nil here keeps an
	// unconfigured attribute reading back as null instead of drifting to a
	// server-asserted [].
	if len(prereqs) == 0 {
		return nil
	}
	out := make([]prerequisiteModel, 0, len(prereqs))
	for _, p := range prereqs {
		out = append(out, prerequisiteModel{
			ID:        types.StringValue(p.ID),
			Condition: types.StringValue(p.Condition),
		})
	}
	return out
}

// featurePrerequisitesToAPI converts the plan's feature-level prerequisites
// set (feature IDs only, no condition) into the API's clear-vs-leave
// pointer form, the same convention as rulesToAPI: a null/unknown set
// means the plan omitted prerequisites entirely (leave unmanaged), a
// known set - even an empty one - clears/replaces them.
func featurePrerequisitesToAPI(ctx context.Context, set types.Set, diags *diag.Diagnostics) *[]string {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}
	out := stringSetToSlice(ctx, set, diags)
	if out == nil {
		out = []string{}
	}
	return &out
}

// scheduleRulesToAPI converts a rule's schedule_rules list into the wire
// representation. Like prerequisitesToAPI, this is a plain (possibly nil)
// slice: it backs ruleModel.ScheduleRules, embedded in a rule that's always
// replaced wholesale on update.
func scheduleRulesToAPI(rules []scheduleRuleModel) []growthbook.ScheduleRule {
	if rules == nil {
		return nil
	}
	out := make([]growthbook.ScheduleRule, 0, len(rules))
	for _, sr := range rules {
		var ts *string
		if !sr.Timestamp.IsNull() && !sr.Timestamp.IsUnknown() {
			v := sr.Timestamp.ValueString()
			ts = &v
		}
		out = append(out, growthbook.ScheduleRule{Enabled: sr.Enabled.ValueBool(), Timestamp: ts})
	}
	return out
}

func scheduleRulesFromAPI(rules []growthbook.ScheduleRule) []scheduleRuleModel {
	// Same normalization as prerequisitesFromAPI: treat an empty response
	// array as absent so an unconfigured attribute reads back as null.
	if len(rules) == 0 {
		return nil
	}
	out := make([]scheduleRuleModel, 0, len(rules))
	for _, sr := range rules {
		m := scheduleRuleModel{Enabled: types.BoolValue(sr.Enabled)}
		if sr.Timestamp != nil {
			m.Timestamp = types.StringValue(*sr.Timestamp)
		} else {
			m.Timestamp = types.StringNull()
		}
		out = append(out, m)
	}
	return out
}

// ruleToAPI converts one Terraform rule model into the wire representation.
func ruleToAPI(ctx context.Context, r ruleModel, diags *diag.Diagnostics) growthbook.FeatureRule {
	condition := ""
	if !r.Condition.IsNull() && !r.Condition.IsUnknown() {
		condition = r.Condition.ValueString()
	}
	out := growthbook.FeatureRule{
		Type:            r.Type.ValueString(),
		Description:     r.Description.ValueString(),
		Condition:       condition,
		AllEnvironments: r.AllEnvironments.ValueBool(),
		Value:           r.Value.ValueString(),
		HashAttribute:   r.HashAttribute.ValueString(),
		ExperimentID:    r.ExperimentID.ValueString(),
	}
	if !r.Enabled.IsNull() && !r.Enabled.IsUnknown() {
		v := r.Enabled.ValueBool()
		out.Enabled = &v
	}
	if !r.Coverage.IsNull() && !r.Coverage.IsUnknown() {
		v := r.Coverage.ValueFloat64()
		out.Coverage = &v
	}
	out.Environments = stringSetToSlice(ctx, r.Environments, diags)
	out.Prerequisites = prerequisitesToAPI(r.Prerequisites)
	out.ScheduleType = r.ScheduleType.ValueString()
	out.ScheduleRules = scheduleRulesToAPI(r.ScheduleRules)
	for _, sg := range r.SavedGroups {
		out.SavedGroups = append(out.SavedGroups, growthbook.FeatureSavedGroupTargeting{
			Match: sg.Match.ValueString(),
			IDs:   stringSetToSlice(ctx, sg.IDs, diags),
		})
	}
	for _, v := range r.Variations {
		out.Variations = append(out.Variations, growthbook.FeatureRuleVariation{
			VariationID: v.VariationID.ValueString(),
			Value:       v.Value.ValueString(),
		})
	}
	return out
}

// ruleFromAPI converts one API rule into the Terraform rule model.
func ruleFromAPI(ctx context.Context, r growthbook.FeatureRule, diags *diag.Diagnostics) ruleModel {
	out := ruleModel{
		Type:            types.StringValue(r.Type),
		Description:     optionalString(r.Description),
		Condition:       optionalString(r.Condition),
		AllEnvironments: types.BoolValue(r.AllEnvironments),
		Environments:    stringSetValue(ctx, r.Environments, diags),
		Prerequisites:   prerequisitesFromAPI(r.Prerequisites),
		Value:           optionalString(r.Value),
		HashAttribute:   optionalString(r.HashAttribute),
		ExperimentID:    optionalString(r.ExperimentID),
		RuleID:          types.StringValue(r.ID),
		ScheduleType:    optionalString(r.ScheduleType),
		ScheduleRules:   scheduleRulesFromAPI(r.ScheduleRules),
	}
	// enabled is Optional+Computed with a default of true, matching
	// GrowthBook's own default for a rule that doesn't specify it.
	if r.Enabled != nil {
		out.Enabled = types.BoolValue(*r.Enabled)
	} else {
		out.Enabled = types.BoolValue(true)
	}
	if r.Coverage != nil {
		out.Coverage = types.Float64Value(*r.Coverage)
	} else {
		out.Coverage = types.Float64Null()
	}
	for _, sg := range r.SavedGroups {
		out.SavedGroups = append(out.SavedGroups, savedGroupModel{
			Match: types.StringValue(sg.Match),
			IDs:   stringSetValue(ctx, sg.IDs, diags),
		})
	}
	for _, v := range r.Variations {
		out.Variations = append(out.Variations, variationModel{
			VariationID: optionalString(v.VariationID),
			Value:       types.StringValue(v.Value),
		})
	}
	return out
}

// rulesToAPI converts the plan's rules list into the API's clear-vs-leave
// pointer form: nil means the plan omitted rules entirely (leave unmanaged),
// a non-nil pointer to an empty slice clears the feature's rules.
func rulesToAPI(ctx context.Context, rules []ruleModel, diags *diag.Diagnostics) *[]growthbook.FeatureRule {
	if rules == nil {
		return nil
	}
	out := make([]growthbook.FeatureRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, ruleToAPI(ctx, r, diags))
	}
	return &out
}

func rulesFromAPI(ctx context.Context, rules []growthbook.FeatureRule, diags *diag.Diagnostics) []ruleModel {
	if rules == nil {
		return nil
	}
	out := make([]ruleModel, 0, len(rules))
	for _, r := range rules {
		out = append(out, ruleFromAPI(ctx, r, diags))
	}
	return out
}

func environmentsToAPI(envs map[string]environmentModel) map[string]growthbook.FeatureEnvironmentRequest {
	if envs == nil {
		return nil
	}
	out := make(map[string]growthbook.FeatureEnvironmentRequest, len(envs))
	for name, e := range envs {
		v := e.Enabled.ValueBool()
		out[name] = growthbook.FeatureEnvironmentRequest{Enabled: &v}
	}
	return out
}

func environmentsFromAPI(envs map[string]growthbook.FeatureEnvironment) map[string]environmentModel {
	if envs == nil {
		return nil
	}
	out := make(map[string]environmentModel, len(envs))
	for name, e := range envs {
		out[name] = environmentModel{Enabled: types.BoolValue(e.Enabled)}
	}
	return out
}

// optionalString renders an API string into a Terraform value, treating the
// empty string as null so optional+computed attributes the caller never set
// don't show permanent drift.
func optionalString(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// featureModelFromAPI builds the full Terraform model from an API feature.
func featureModelFromAPI(ctx context.Context, f *growthbook.Feature, diags *diag.Diagnostics) featureModel {
	m := featureModel{
		ID:              types.StringValue(f.ID),
		ValueType:       types.StringValue(f.ValueType),
		DefaultValue:    types.StringValue(f.DefaultValue),
		Description:     optionalString(f.Description),
		Project:         optionalString(f.Project),
		Owner:           optionalString(f.Owner),
		Archived:        types.BoolValue(f.Archived),
		Tags:            stringSetValue(ctx, f.Tags, diags),
		Environments:    environmentsFromAPI(f.Environments),
		Rules:           rulesFromAPI(ctx, f.Rules, diags),
		Prerequisites:   stringSetValue(ctx, f.Prerequisites, diags),
		DateCreated:     types.StringValue(f.DateCreated),
		DateUpdated:     types.StringValue(f.DateUpdated),
		RevisionVersion: types.Int64Null(),
	}
	if f.Revision != nil {
		m.RevisionVersion = types.Int64Value(int64(f.Revision.Version))
	}
	return m
}

// featureCreateRequest builds the create request body from the plan model.
func featureCreateRequest(ctx context.Context, m featureModel, diags *diag.Diagnostics) growthbook.FeatureRequest {
	req := growthbook.FeatureRequest{
		ID:            m.ID.ValueString(),
		ValueType:     m.ValueType.ValueString(),
		DefaultValue:  m.DefaultValue.ValueString(),
		Tags:          stringSetToSlice(ctx, m.Tags, diags),
		Environments:  environmentsToAPI(m.Environments),
		Rules:         rulesToAPI(ctx, m.Rules, diags),
		Prerequisites: featurePrerequisitesToAPI(ctx, m.Prerequisites, diags),
	}
	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		v := m.Description.ValueString()
		req.Description = &v
	}
	if !m.Project.IsNull() && !m.Project.IsUnknown() {
		v := m.Project.ValueString()
		req.Project = &v
	}
	if !m.Owner.IsNull() && !m.Owner.IsUnknown() {
		v := m.Owner.ValueString()
		req.Owner = &v
	}
	if !m.Archived.IsNull() && !m.Archived.IsUnknown() {
		v := m.Archived.ValueBool()
		req.Archived = &v
	}
	return req
}

// featureUpdateRequest builds the update request body from the plan model.
// It is identical to the create body except ID/ValueType, which
// UpdateFeature strips before sending.
func featureUpdateRequest(ctx context.Context, m featureModel, diags *diag.Diagnostics) growthbook.FeatureRequest {
	return featureCreateRequest(ctx, m, diags)
}

// jsonStringValidator requires an optional string attribute to be valid
// JSON (RFC 7159) when set.
type jsonStringValidator struct{}

func (v jsonStringValidator) Description(_ context.Context) string {
	return "value must be valid JSON"
}

func (v jsonStringValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v jsonStringValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if !json.Valid([]byte(req.ConfigValue.ValueString())) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid JSON String Value",
			"A string value was provided that is not valid JSON (RFC 7159): "+req.ConfigValue.ValueString(),
		)
	}
}

// jsonNormalizePlanModifier suppresses a diff on an optional JSON string
// attribute when the planned and prior state values are semantically equal
// JSON (differing only in whitespace or property order).
type jsonNormalizePlanModifier struct{}

func (m jsonNormalizePlanModifier) Description(_ context.Context) string {
	return "normalizes JSON so inconsequential formatting differences don't produce a diff"
}

func (m jsonNormalizePlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m jsonNormalizePlanModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if jsonSemanticEqual(req.StateValue.ValueString(), req.ConfigValue.ValueString()) {
		resp.PlanValue = req.StateValue
	}
}

func jsonSemanticEqual(a, b string) bool {
	var av, bv any
	if json.Unmarshal([]byte(a), &av) != nil || json.Unmarshal([]byte(b), &bv) != nil {
		return a == b
	}
	na, errA := json.Marshal(av)
	nb, errB := json.Marshal(bv)
	if errA != nil || errB != nil {
		return a == b
	}
	return string(na) == string(nb)
}
