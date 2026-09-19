# API Coverage

This document is a field-by-field coverage matrix between the GrowthBook REST API and the Terraform attributes exposed by this provider, current as of the 0.1.0 release. It is meant to make gaps explicit rather than implied: every GrowthBook API field is listed even when this provider does not expose it, and every Terraform attribute is listed even when it has no direct API counterpart.

Sources: the GrowthBook OpenAPI spec (`/v1/projects`, `/v1/environments`, `/v1/sdk-connections`, `/v2/features` paths and their component schemas) cross-checked against `internal/provider/*.go` in this repository. The features API in the spec used for this audit is v2; projects, environments and sdk-connections are v1.

## growthbook_project

| API Field | Terraform Attribute | Supported | Notes |
|---|---|---|---|
| `id` | `id` | yes | Server-assigned, `prj_...`. |
| `name` | `name` | yes | Required in both. |
| `description` | `description` | yes | Optional+computed; explicit `""` clears it. |
| `publicId` | `public_id` | yes | Auto-derived from `name` when unset. |
| `restrictAccess` | `restrict_access` | yes | API notes this requires a Pro or Enterprise plan; the provider does not gate on plan tier, it simply forwards the value and lets the API reject it if unsupported. |
| `settings.statsEngine` | `settings.stats_engine` | yes | |
| `settings.confidenceLevel` | `settings.confidence_level` | yes | |
| `settings.pValueThreshold` | `settings.p_value_threshold` | yes | |
| `dateCreated` | `date_created` | computed-only | Response-only field, no request equivalent. |
| `dateUpdated` | `date_updated` | computed-only | Response-only field, no request equivalent; recomputed on every update. |

## growthbook_environment

| API Field | Terraform Attribute | Supported | Notes |
|---|---|---|---|
| `id` | `id` | yes | Caller-chosen on create; `RequiresReplace` in this provider. |
| `description` | `description` | yes | |
| `toggleOnList` | `toggle_on_list` | yes | |
| `defaultState` | `default_state` | yes | |
| `projects` | `projects` | yes | Empty/omitted normalized to null by the provider regardless of whether the API returns a JSON null or an empty array. |
| `parent` | `parent` | yes | Create-only in both the API (`postEnvironment`, "requires an enterprise license") and this provider (`RequiresReplace`); the API's `putEnvironment` body does not accept it at all, matching the provider's own omission of `parent` from update requests. |

## growthbook_feature

### Feature-level fields

| API Field | Terraform Attribute | Supported | Notes |
|---|---|---|---|
| `id` | `id` | yes | The feature key. `RequiresReplace`. |
| `valueType` | `value_type` | yes | Enum `boolean`/`string`/`number`/`json`. Immutable after create in both the API and this provider (`RequiresReplace`). |
| `defaultValue` | `default_value` | yes | Stored/sent as a string regardless of `valueType`. |
| `description` | `description` | yes | |
| `owner` | `owner` | yes | |
| `project` | `project` | yes | Primary project id only. |
| `tags` | `tags` | yes | Full replace semantics on update, matching the API. |
| `archived` | `archived` | yes | Defaults to `false` in the schema, matching GrowthBook's default. |
| `environments` (request: `{enabled}` map; response: `FeatureEnvironmentV2`, richer per-env object) | `environments` (map of `{enabled}`) | yes | Only `enabled` is modeled per environment; the response's richer per-environment object (compiled SDK payload/definition) is not surfaced - see Not yet modelled. |
| `rules` | `rules` | yes | See rule-level table below; only a subset of rule types and rule fields are modeled. |
| `prerequisites` (feature-level, array of feature-id strings) | `prerequisites` (set of strings) | yes | Each entry is another feature's id, which must evaluate to `true`. No per-entry condition at this level - see rule-level prerequisites below for that. |
| `dateCreated` | `date_created` | computed-only | Response-only. |
| `dateUpdated` | `date_updated` | computed-only | Response-only. |
| `revision` (full `FeatureRevisionSummary`/`FeatureRevisionV2` object: id, featureId, baseVersion, version, comment, date, status, createdBy, publishedBy, reviews, scheduled-publish fields, rampActions, metadata, etc.) | `revision_version` | computed-only | Only `revision.version` is surfaced, as an integer; every other revision/draft field is unmodeled. See Not yet modelled. |
| `targetingAllProjects` | `n/a` | no | Out of scope for 0.1; secondary-project targeting is not modeled at all. |
| `targetingProjects` | `n/a` | no | Out of scope for 0.1; secondary-project targeting is not modeled at all. |
| `baseConfig` | `n/a` | no | "Config mode" (JSON features backed by a shared config resource) is not modeled; out of scope for 0.1. |
| `defaultValueConfig` | `n/a` | no | Part of Config mode; not modeled, same reason as `baseConfig`. |
| `customFields` | `n/a` | no | Org-defined custom metadata map; not modeled, out of scope for 0.1. |
| `jsonSchema` | `n/a` | no | Enterprise-only JSON Schema validation for `json`-type feature values; not modeled, premium-tier gating and out of scope for 0.1. |
| `holdout` | `n/a` | no | Holdout-experiment assignment; not modeled, out of scope for 0.1. |
| `ignoreWarnings` | `n/a` | no | Write-time escape hatch to bypass governance warnings; not modeled, part of the approval/governance workflow this provider does not touch. |
| `skipSchemaValidation` | `n/a` | no | Write-time escape hatch to bypass schema validation; not modeled, same governance-workflow reason. |
| `skipHooks` | `n/a` | no | Write-time escape hatch to bypass Custom Hooks; not modeled, same governance-workflow reason. |

### Rule-level fields (`rules[]`)

The provider models three rule types: `force`, `rollout`, and `experiment-ref`. The API additionally defines `experiment` (inline), `contextual-bandit-ref`, and `safe-rollout`, none of which this provider supports - see Not yet modelled.

| API Field | Terraform Attribute | Supported | Notes |
|---|---|---|---|
| `type` | `rules[].type` | yes | Restricted by the provider's validator to `force`, `rollout`, `experiment-ref` - a strict subset of the API's rule-type enum. |
| `description` | `rules[].description` | yes | |
| `enabled` | `rules[].enabled` | yes | Optional+computed, defaults to `true`, matching GrowthBook's own rule default. |
| `condition` | `rules[].condition` | yes | JSON string; the provider normalizes whitespace/property-order differences to avoid spurious diffs. |
| `savedGroups` / `savedGroupTargeting` (the API uses both names across schema variants for the same targeting-by-saved-group concept) | `rules[].saved_groups[]` (`match`, `ids`) | yes | |
| `id` (rule id) | `rules[].rule_id` | computed-only | Server-assigned; response-only. |
| `allEnvironments` | `rules[].all_environments` | yes | Defaults to `false`. |
| `environments` (rule-level) | `rules[].environments` | yes | |
| `value` | `rules[].value` | yes | Applies to `force` and `rollout`. |
| `coverage` | `rules[].coverage` | yes | Applies to `rollout` only. |
| `hashAttribute` | `rules[].hash_attribute` | yes | Applies to `rollout` only in this provider; the API also uses it on `experiment` (inline) and `safe-rollout`, neither of which is modeled. |
| `experimentId` | `rules[].experiment_id` | yes | Applies to `experiment-ref` only. |
| `variations[].value` | `rules[].variations[].value` | yes | Applies to `experiment-ref` only. |
| `variations[].variationId` | `rules[].variations[].variation_id` | yes | |
| `prerequisites` (rule-level, `{id, condition}`) | `rules[].prerequisites[]` (`id`, `condition`) | yes | Gates the rule on another feature's value; a plain (non-pointer) list, since rules are always replaced wholesale on update. Requires GrowthBook Enterprise (`prerequisite-targeting`); on other plans GrowthBook drops it and the provider reports an error. |
| `variations[].config` | `n/a` | no | Config-mode override pointer on an `experiment-ref` variation; not modeled, same Config-mode reason as `baseConfig` above. |
| `config` | `n/a` | no | Config-mode override pointer on `force`/`rollout`; not modeled. |
| `sparse` | `n/a` | no | JSON-only partial-merge semantics for `force`/`rollout`/`experiment-ref` values; not modeled, out of scope for 0.1. |
| `seed` | `n/a` | no | Hash seed override for `rollout` (defaults to rule id); not modeled, out of scope for 0.1. |
| `hashVersion` | `n/a` | no | Hash algorithm version (1 or 2) for `rollout`; not modeled, out of scope for 0.1. |
| `scheduleRules` | `rules[].schedule_rules[]` (`enabled`, `timestamp`) | yes | Simple time-based on/off schedule; a plain (non-pointer) list, same replaced-wholesale reasoning as rule-level prerequisites. Requires GrowthBook Pro (`schedule-feature-flag`); on other plans GrowthBook drops it and the provider reports an error. |
| `scheduleType` | `rules[].schedule_type` | yes | UI hint for scheduling mode; the provider restricts it to `none`/`schedule` (`ramp` is not modeled, since it depends on the unmodeled ramp-schedule sub-resource below). |
| `rampScheduleId` | `n/a` | no | Link to a multi-step ramp-schedule sub-resource; not modeled, out of scope for 0.1. |
| `pendingRamp` | `n/a` | no | Draft-only signal that a ramp schedule will be created/detached on publish; not modeled, tied to the draft/revision workflow. |
| `allProjects` | `n/a` | no | Rule-level "target all projects" flag; not modeled, out of scope for 0.1. |
| `projects` (rule-level) | `n/a` | no | Rule-level secondary-project targeting; not modeled, out of scope for 0.1. |
| rule type `experiment` (inline) and its fields (`trackingKey`, `fallbackAttribute`, `disableStickyBucketing`, `bucketVersion`, `minBucketVersion`, `namespace`, `coverage`, `value[]` with weights) | `n/a` | no | Entire rule type not modeled; see Not yet modelled. Also structurally unwritable through this stack: the v2 feature write bodies (`postFeatureV2`/`updateFeatureV2`) only accept `force`, `rollout`, `experiment-ref`, and `safe-rollout` rules, so `growthbook-go` cannot round-trip an inline `experiment` rule even if this provider modeled its fields. |
| rule type `contextual-bandit-ref` and its fields (`variations`, `contextualBanditId`) | `n/a` | no | Entire rule type not modeled; see Not yet modelled. |
| rule type `safe-rollout` and its fields (`controlValue`, `variationValue`, `hashAttribute`, `trackingKey`, `seed`, `safeRolloutId`/`status` or `safeRolloutFields`) | `n/a` | no | Entire rule type not modeled; see Not yet modelled. |

## growthbook_sdk_connection

| API Field | Terraform Attribute | Supported | Notes |
|---|---|---|---|
| `name` | `name` | yes | |
| `language` | `language` | yes | Request-only singular field; the provider deliberately keeps the configured value in state rather than re-deriving it from the response's `languages` array (see code comment in `sdk_connection_model.go`). |
| `environment` | `environment` | yes | |
| `projects` | `projects` | yes | |
| `sdkVersion` | `sdk_version` | yes | |
| `encryptPayload` | `encrypt_payload` | yes | |
| `includeVisualExperiments` | `include_visual_experiments` | yes | |
| `includeDraftExperiments` | `include_draft_experiments` | yes | |
| `includeDraftExperimentRefs` | `include_draft_experiment_refs` | yes | |
| `includeExperimentNames` | `include_experiment_names` | yes | |
| `includeRedirectExperiments` | `include_redirect_experiments` | yes | |
| `includeRuleIds` | `include_rule_ids` | yes | |
| `includeProjectIdInMetadata` | `include_project_id_in_metadata` | yes | |
| `includeCustomFieldsInMetadata` | `include_custom_fields_in_metadata` | yes | |
| `allowedCustomFieldsInMetadata` | `allowed_custom_fields_in_metadata` | yes | |
| `includeTagsInMetadata` | `include_tags_in_metadata` | yes | |
| `includeExperimentScheduleInMetadata` | `include_experiment_schedule_in_metadata` | yes | |
| `proxyEnabled` | `proxy_enabled` | yes | |
| `proxyHost` | `proxy_host` | yes | |
| `hashSecureAttributes` | `hash_secure_attributes` | yes | |
| `remoteEvalEnabled` | `remote_eval_enabled` | yes | |
| `savedGroupReferencesEnabled` | `saved_group_references_enabled` | yes | |
| `includeReferencedPrerequisites` | `include_referenced_prerequisites` | yes | Response schema omits the request's description text but is the same boolean field. |
| `id` | `id` | computed-only | Response-only. |
| `key` | `key` | computed-only | Response-only, sensitive. |
| `encryptionKey` | `encryption_key` | computed-only | Response-only, sensitive. |
| `proxySigningKey` | `proxy_signing_key` | computed-only | Response-only, sensitive. |
| `dateCreated` | `date_created` | computed-only | Response-only. |
| `dateUpdated` | `date_updated` | computed-only | Response-only. |
| `languages` (plural, response array of every SDK language this connection reports) | `n/a` | no | Superseded in this provider by the singular `language` attribute, which round-trips the caller's configured value instead. The full response array is not surfaced. |
| `organization` | `n/a` | no | Organization id the connection belongs to; redundant with the provider config's own scoping, out of scope for 0.1. |
| `project` (singular, legacy/back-compat field, "first project only" per the API's own description) | `n/a` | no | Deprecated alias for `projects`; the provider only models the plural, current field. |
| `sseEnabled` | `n/a` | no | Server-Sent Events streaming toggle for this connection; response-only, not modeled, out of scope for 0.1. |
| `n/a` | `connected` | yes | Terraform-only field with no corresponding property in the `/v1/sdk-connections` OpenAPI schema reviewed for this audit. It is plumbed straight from the Go client's `SDKConnection.Connected`. Could not be verified against the OpenAPI extract used here; flagged as unverified rather than invented. |

## growthbook_saved_group

| API Field | Terraform Attribute | Supported | Notes |
|---|---|---|---|
| `id` | `id` | yes | Server-assigned. |
| `name` | `name` | yes | |
| `type` | `type` | yes | `condition` or `list`. Immutable after creation: the update request schema (`additionalProperties: false`) does not accept `type` at all, so the provider only sends it on create. |
| `condition` | `condition` | yes | Applies when `type = "condition"`. |
| `attributeKey` | `attribute_key` | yes | Applies when `type = "list"`. Immutable after creation, same reason as `type`: absent from the update request schema. Must reference an attribute that already exists in the organization (see `growthbook_attribute`); the API returns HTTP 400 ("Unknown attributeKey") for an attribute that doesn't exist yet. |
| `values` | `values` | yes | Applies when `type = "list"`. `*[]string` on the client: a nil pointer (unconfigured attribute) omits the field and leaves the server value unchanged; a pointer to an empty slice (`values = []`) sends `[]` and clears it. |
| `owner` | `owner` | yes | Optional+computed: defaults to the PAT-associated user when omitted on create. |
| `ownerEmail` | `owner_email` | computed-only | Response-only, resolved from `owner` when possible. |
| `description` | `description` | computed-only | Present in the response schema but not accepted by either the create or update request schema, so it cannot be set through this provider. |
| `projects` | `projects` | yes | Same nil-omits/empty-clears `*[]string` handling as `values`. |
| `archived` | `archived` | computed-only | No archive/unarchive endpoint is modeled; not settable through this resource. |
| `useEmptyListGroup` | `use_empty_list_group` | computed-only | Response-only, GrowthBook-managed. |
| `dateCreated` | `date_created` | computed-only | Response-only. |
| `dateUpdated` | `date_updated` | computed-only | Response-only. |
| `bypassApproval` (request-only) | `n/a` | no | Escape hatch to apply a change immediately under an org that requires draft approvals; not modeled, same governance-workflow reason as the feature-level escape hatches above. |

## Not yet modelled

- **Feature-level secondary-project targeting** (`targetingAllProjects`, `targetingProjects`) - a feature can be served to projects beyond its primary `project`; would need two new attributes on `growthbook_feature` plus request/response wiring.
- **Config mode** (`baseConfig`, `defaultValueConfig`, and the per-value `config` pointer on rules/variations) - JSON features backed by a shared, versioned config resource with override patches; a substantial feature area with its own `/configs` API surface not touched by this provider at all.
- **Ramp schedules** (`rampScheduleId`, `pendingRamp`, and the `rampActions` sub-resource on feature revisions) - multi-step gradual rollout scheduling; a separate sub-resource this provider does not expose. The simple on/off `scheduleRules`/`scheduleType` scheduling is modeled (see the rule-level table above); `scheduleType = "ramp"` is not, since it depends on this sub-resource.
- **Safe-rollout rules** (rule type `safe-rollout`, with `controlValue`, `variationValue`, `guardrailMetricIds`, `maxDuration`, `autoRollback`, `rampUpSchedule`, etc.) - an entire rule type built on the datasource/metrics system, not modeled.
- **Inline experiment rules** (rule type `experiment`, distinct from `experiment-ref`) - defines an experiment directly on the feature rather than referencing an existing one; not modeled. Beyond being out of scope, it is structurally unwritable through this stack: GrowthBook's v2 feature write bodies only accept `force`, `rollout`, `experiment-ref`, and `safe-rollout` rules, so `growthbook-go` has no way to send one even if this provider modeled its fields.
- **Contextual-bandit rules** (rule type `contextual-bandit-ref`) - routes traffic through a contextual bandit rather than a static experiment; not modeled.
- **Custom fields** (`customFields` on features) - org-defined metadata key/value pairs; not modeled.
- **JSON Schema validation** (`jsonSchema` on features, an Enterprise-only field) - schema-validates a `json`-type feature's value; not modeled and premium-gated besides.
- **Holdout groups** (`holdout` on features) - assigns a feature to a holdout experiment/treatment group; not modeled.
- **Write-time governance escape hatches** (`ignoreWarnings`, `skipSchemaValidation`, `skipHooks` on feature writes; `bypassedGates` on toggle responses) - bypass flags for approval gates, schema invariants, and custom hooks; none of the underlying approval/governance workflow is modeled, so these flags have nothing to attach to.
- **Feature revisions/drafts** (`FeatureRevisionV2`: `comment`, `status`, `createdBy`/`publishedBy`, `reviews[]`, the `scheduledPublish*` cluster, `metadata`, `definitions`) - this provider always writes directly to the live feature and only surfaces `revision.version` as a computed integer; the entire draft/review/scheduled-publish workflow, and the `GET /v2/features/{id}` revisions list, are untouched.
- **Feature toggle endpoint** (`POST /v2/features/{id}/toggle`, with its `reason` field and `bypassedGates` response) - a dedicated per-environment enable/disable call distinct from a full feature update; not modeled as its own operation.
- **SDK connection language list** (`languages`, the response's full array of SDK languages) - the provider models only the single configured `language`; a connection's complete reported language set is not surfaced.
- **SDK connection Server-Sent Events flag** (`sseEnabled`) - not modeled.
- **Config-mode API surface generally** (the `/v1/configs-revisions/{key}/{version}/projection` path and any `/configs` resource) - out of scope for 0.1 in its entirety; no resource or data source in this provider touches configs.
- **Experiments API** - GrowthBook's experiments (as opposed to experiment-ref rules pointing at them) have no dedicated resource or data source in this provider at all; only references to experiment ids appear inside feature rules and SDK connection settings.
- **Saved group approval workflow** (`bypassApproval` on create/update) - see the `growthbook_saved_group` table above.
- **Organization/member/team management APIs** - no resource or data source in this provider manages organization settings, members, teams, or roles.

## growthbook_attribute

| API Field | Terraform Attribute | Supported | Notes |
|---|---|---|---|
| `property` | `property` | yes | Identifier. `RequiresReplace`. The API has no GET-by-id; `growthbook-go`'s `GetAttribute` lists and filters. |
| `datatype` | `datatype` | yes | Enum `boolean`/`string`/`number`/`secureString`/`enum`/`string[]`/`number[]`/`secureString[]`. Not `RequiresReplace`: the API's `putAttribute` accepts a datatype change and updates in place. |
| `description` | `description` | yes | Optional+computed; explicit `""` clears it, matching `growthbook_project`. |
| `hashAttribute` | `hash_attribute` | yes | |
| `archived` | `archived` | yes | |
| `enum` | `enum` | yes | Comma-separated string, not a list, per the API schema. Required by the API when `datatype` is `enum`; enforced client-side by a `ConfigValidator` so a bad config fails at `terraform plan` instead of a 400 from the API. |
| `format` | `format` | yes | Enum `version`/`date`/`isoCountryCode` (or unset). |
| `projects` | `projects` | yes | Empty/omitted normalized to null by the provider, matching `growthbook_environment`. |
| `tags` | `tags` | yes | Same normalization as `projects`. |
