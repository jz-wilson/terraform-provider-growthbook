// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	growthbook "github.com/jz-wilson/growthbook-go"
)

// sdkConnectionModel is the Terraform representation of a GrowthBook SDK
// connection, shared by the resource and both data sources. The API request
// takes a single "language" string, but every response reports the set as
// "languages"; sdkConnectionFromAPI keeps the configured/queried language
// value rather than re-deriving it from the response slice.
type sdkConnectionModel struct {
	ID                                  types.String `tfsdk:"id"`
	Name                                types.String `tfsdk:"name"`
	Language                            types.String `tfsdk:"language"`
	Environment                         types.String `tfsdk:"environment"`
	Projects                            types.Set    `tfsdk:"projects"`
	SDKVersion                          types.String `tfsdk:"sdk_version"`
	EncryptPayload                      types.Bool   `tfsdk:"encrypt_payload"`
	IncludeVisualExperiments            types.Bool   `tfsdk:"include_visual_experiments"`
	IncludeDraftExperiments             types.Bool   `tfsdk:"include_draft_experiments"`
	IncludeDraftExperimentRefs          types.Bool   `tfsdk:"include_draft_experiment_refs"`
	IncludeExperimentNames              types.Bool   `tfsdk:"include_experiment_names"`
	IncludeRedirectExperiments          types.Bool   `tfsdk:"include_redirect_experiments"`
	IncludeRuleIds                      types.Bool   `tfsdk:"include_rule_ids"`
	IncludeProjectIDInMetadata          types.Bool   `tfsdk:"include_project_id_in_metadata"`
	IncludeCustomFieldsInMetadata       types.Bool   `tfsdk:"include_custom_fields_in_metadata"`
	AllowedCustomFieldsInMetadata       types.Set    `tfsdk:"allowed_custom_fields_in_metadata"`
	IncludeTagsInMetadata               types.Bool   `tfsdk:"include_tags_in_metadata"`
	IncludeExperimentScheduleInMetadata types.Bool   `tfsdk:"include_experiment_schedule_in_metadata"`
	ProxyEnabled                        types.Bool   `tfsdk:"proxy_enabled"`
	ProxyHost                           types.String `tfsdk:"proxy_host"`
	HashSecureAttributes                types.Bool   `tfsdk:"hash_secure_attributes"`
	RemoteEvalEnabled                   types.Bool   `tfsdk:"remote_eval_enabled"`
	SavedGroupReferencesEnabled         types.Bool   `tfsdk:"saved_group_references_enabled"`
	IncludeReferencedPrerequisites      types.Bool   `tfsdk:"include_referenced_prerequisites"`
	Key                                 types.String `tfsdk:"key"`
	EncryptionKey                       types.String `tfsdk:"encryption_key"`
	ProxySigningKey                     types.String `tfsdk:"proxy_signing_key"`
	Connected                           types.Bool   `tfsdk:"connected"`
	DateCreated                         types.String `tfsdk:"date_created"`
	DateUpdated                         types.String `tfsdk:"date_updated"`
}

func boolPtrToType(b *bool) types.Bool {
	if b == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*b)
}

func typeToBoolPtr(b types.Bool) *bool {
	if b.IsNull() || b.IsUnknown() {
		return nil
	}
	v := b.ValueBool()
	return &v
}

func stringPtrOrNil(s types.String) *string {
	if s.IsNull() || s.IsUnknown() || s.ValueString() == "" {
		return nil
	}
	v := s.ValueString()
	return &v
}

// languageKnown reports whether the configured language is present in the
// set of languages the API returned for this connection.
func languageKnown(language string, languages []string) bool {
	for _, l := range languages {
		if l == language {
			return true
		}
	}
	return false
}

// sdkConnectionFromAPI populates a model from an API response. language is
// the value to keep in state for the "language" attribute: the configured
// value on create/update, or the first entry of Languages when reading a
// connection that was not just written by this resource (e.g. import, or a
// data source lookup by id).
func sdkConnectionFromAPI(ctx context.Context, conn *growthbook.SDKConnection, language string) (sdkConnectionModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	if language == "" || !languageKnown(language, conn.Languages) {
		if len(conn.Languages) > 0 {
			language = conn.Languages[0]
		}
	}

	model := sdkConnectionModel{
		ID:                                  types.StringValue(conn.ID),
		Name:                                types.StringValue(conn.Name),
		Language:                            types.StringValue(language),
		Environment:                         types.StringValue(conn.Environment),
		SDKVersion:                          types.StringValue(conn.SDKVersion),
		EncryptPayload:                      types.BoolValue(conn.EncryptPayload),
		IncludeVisualExperiments:            boolPtrToType(conn.IncludeVisualExperiments),
		IncludeDraftExperiments:             boolPtrToType(conn.IncludeDraftExperiments),
		IncludeDraftExperimentRefs:          boolPtrToType(conn.IncludeDraftExperimentRefs),
		IncludeExperimentNames:              boolPtrToType(conn.IncludeExperimentNames),
		IncludeRedirectExperiments:          boolPtrToType(conn.IncludeRedirectExperiments),
		IncludeRuleIds:                      boolPtrToType(conn.IncludeRuleIds),
		IncludeProjectIDInMetadata:          boolPtrToType(conn.IncludeProjectIdInMetadata),
		IncludeCustomFieldsInMetadata:       boolPtrToType(conn.IncludeCustomFieldsInMetadata),
		IncludeTagsInMetadata:               boolPtrToType(conn.IncludeTagsInMetadata),
		IncludeExperimentScheduleInMetadata: boolPtrToType(conn.IncludeExperimentScheduleInMetadata),
		ProxyEnabled:                        types.BoolValue(conn.ProxyEnabled),
		ProxyHost:                           types.StringValue(conn.ProxyHost),
		HashSecureAttributes:                boolPtrToType(conn.HashSecureAttributes),
		RemoteEvalEnabled:                   boolPtrToType(conn.RemoteEvalEnabled),
		SavedGroupReferencesEnabled:         boolPtrToType(conn.SavedGroupReferencesEnabled),
		IncludeReferencedPrerequisites:      boolPtrToType(conn.IncludeReferencedPrerequisites),
		Key:                                 types.StringValue(conn.Key),
		EncryptionKey:                       types.StringValue(conn.EncryptionKey),
		ProxySigningKey:                     types.StringValue(conn.ProxySigningKey),
		Connected:                           boolPtrToType(conn.Connected),
		DateCreated:                         types.StringValue(conn.DateCreated),
		DateUpdated:                         types.StringValue(conn.DateUpdated),
	}

	projects, d := types.SetValueFrom(ctx, types.StringType, conn.Projects)
	diags.Append(d...)
	model.Projects = projects

	allowed, d := types.SetValueFrom(ctx, types.StringType, conn.AllowedCustomFieldsInMetadata)
	diags.Append(d...)
	model.AllowedCustomFieldsInMetadata = allowed

	return model, diags
}

// sdkConnectionToRequest converts a resource/data-source model into the
// request body used for create and update calls.
func sdkConnectionToRequest(ctx context.Context, m sdkConnectionModel) (growthbook.SDKConnectionRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := growthbook.SDKConnectionRequest{
		Name:                                m.Name.ValueString(),
		Language:                            m.Language.ValueString(),
		Environment:                         m.Environment.ValueString(),
		SDKVersion:                          stringPtrOrNil(m.SDKVersion),
		EncryptPayload:                      typeToBoolPtr(m.EncryptPayload),
		IncludeVisualExperiments:            typeToBoolPtr(m.IncludeVisualExperiments),
		IncludeDraftExperiments:             typeToBoolPtr(m.IncludeDraftExperiments),
		IncludeDraftExperimentRefs:          typeToBoolPtr(m.IncludeDraftExperimentRefs),
		IncludeExperimentNames:              typeToBoolPtr(m.IncludeExperimentNames),
		IncludeRedirectExperiments:          typeToBoolPtr(m.IncludeRedirectExperiments),
		IncludeRuleIds:                      typeToBoolPtr(m.IncludeRuleIds),
		IncludeProjectIdInMetadata:          typeToBoolPtr(m.IncludeProjectIDInMetadata),
		IncludeCustomFieldsInMetadata:       typeToBoolPtr(m.IncludeCustomFieldsInMetadata),
		IncludeTagsInMetadata:               typeToBoolPtr(m.IncludeTagsInMetadata),
		IncludeExperimentScheduleInMetadata: typeToBoolPtr(m.IncludeExperimentScheduleInMetadata),
		ProxyEnabled:                        typeToBoolPtr(m.ProxyEnabled),
		ProxyHost:                           stringPtrOrNil(m.ProxyHost),
		HashSecureAttributes:                typeToBoolPtr(m.HashSecureAttributes),
		RemoteEvalEnabled:                   typeToBoolPtr(m.RemoteEvalEnabled),
		SavedGroupReferencesEnabled:         typeToBoolPtr(m.SavedGroupReferencesEnabled),
		IncludeReferencedPrerequisites:      typeToBoolPtr(m.IncludeReferencedPrerequisites),
	}

	if !m.Projects.IsNull() && !m.Projects.IsUnknown() {
		var projects []string
		diags.Append(m.Projects.ElementsAs(ctx, &projects, false)...)
		req.Projects = projects
	}
	if !m.AllowedCustomFieldsInMetadata.IsNull() && !m.AllowedCustomFieldsInMetadata.IsUnknown() {
		var allowed []string
		diags.Append(m.AllowedCustomFieldsInMetadata.ElementsAs(ctx, &allowed, false)...)
		req.AllowedCustomFieldsInMetadata = allowed
	}

	return req, diags
}

// sdkConnectionDataSourceOptionAttributes returns the non-identifying,
// non-secret option attributes shared by both data sources, all computed.
// The resource defines its own equivalent optional+computed attributes
// (see sdkConnectionResourceOptionAttributes), since the framework's
// resource and data source schema attribute types are distinct.
func sdkConnectionDataSourceOptionAttributes() map[string]dschema.Attribute {
	boolAttr := func(desc string) dschema.Attribute {
		return dschema.BoolAttribute{MarkdownDescription: desc, Computed: true}
	}
	return map[string]dschema.Attribute{
		"sdk_version":                             dschema.StringAttribute{MarkdownDescription: "SDK version reported to GrowthBook for payload compatibility.", Computed: true},
		"encrypt_payload":                         boolAttr("Whether the SDK payload is encrypted."),
		"include_visual_experiments":              boolAttr("Whether visual editor experiments are included in the payload."),
		"include_draft_experiments":               boolAttr("Whether draft experiments are included in the payload."),
		"include_draft_experiment_refs":           boolAttr("Whether draft experiment references are included in the payload."),
		"include_experiment_names":                boolAttr("Whether experiment names are included in the payload."),
		"include_redirect_experiments":            boolAttr("Whether URL redirect experiments are included in the payload."),
		"include_rule_ids":                        boolAttr("Whether rule ids are included in the payload."),
		"include_project_id_in_metadata":          boolAttr("Whether project ids are included in feature metadata."),
		"include_custom_fields_in_metadata":       boolAttr("Whether custom fields are included in feature metadata."),
		"include_tags_in_metadata":                boolAttr("Whether tags are included in feature metadata."),
		"include_experiment_schedule_in_metadata": boolAttr("Whether experiment schedules are included in feature metadata."),
		"proxy_enabled":                           boolAttr("Whether GrowthBook Proxy is enabled for this connection."),
		"proxy_host":                              dschema.StringAttribute{MarkdownDescription: "GrowthBook Proxy host URL.", Computed: true},
		"hash_secure_attributes":                  boolAttr("Whether secure attributes are hashed before being sent to the SDK."),
		"remote_eval_enabled":                     boolAttr("Whether remote evaluation is enabled for this connection."),
		"saved_group_references_enabled":          boolAttr("Whether saved group references are enabled in the payload."),
		"include_referenced_prerequisites":        boolAttr("Whether referenced prerequisite features are included in the payload."),
		"connected":                               boolAttr("Whether this SDK connection has received at least one request."),
		"date_created":                            dschema.StringAttribute{MarkdownDescription: "Timestamp the connection was created.", Computed: true},
		"date_updated":                            dschema.StringAttribute{MarkdownDescription: "Timestamp the connection was last updated.", Computed: true},
		"key":                                     dschema.StringAttribute{MarkdownDescription: "Client key used by SDKs to fetch this connection's payload.", Computed: true, Sensitive: true},
		"encryption_key":                          dschema.StringAttribute{MarkdownDescription: "Key used to decrypt an encrypted payload.", Computed: true, Sensitive: true},
		"proxy_signing_key":                       dschema.StringAttribute{MarkdownDescription: "Signing key used by GrowthBook Proxy.", Computed: true, Sensitive: true},
		"projects":                                dschema.SetAttribute{MarkdownDescription: "Project ids this connection is scoped to. Empty means all projects.", ElementType: types.StringType, Computed: true},
		"allowed_custom_fields_in_metadata": dschema.SetAttribute{
			MarkdownDescription: "Custom field keys allowed in feature metadata.",
			ElementType:         types.StringType,
			Computed:            true,
		},
	}
}
