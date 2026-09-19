# Changelog

## 0.2.0 (Unreleased)

FEATURES:

* **New Resource:** `growthbook_attribute`
* **New Data Source:** `growthbook_attribute`
* **New Resource:** `growthbook_saved_group`
* **New Data Source:** `growthbook_saved_group`
* resource/growthbook_feature: Add feature-level `prerequisites` (a set of feature IDs that must evaluate to `true`) and rule-level `rules.prerequisites` (`id`, `condition`). Rule-level prerequisites require GrowthBook Enterprise; on other plans GrowthBook drops them and the provider reports an error.
* resource/growthbook_feature: Add rule-level `rules.schedule_type` and `rules.schedule_rules` (`enabled`, `timestamp`) for simple time-based on/off scheduling. Requires GrowthBook Pro; on lower plans GrowthBook drops `schedule_rules` and the provider reports an error. `timestamp` uses an RFC3339 custom type with semantic equality, so GrowthBook reformatting a configured timestamp (e.g. adding a `.000` fraction, or `Z` vs. `+00:00`) doesn't produce a diff.

BUG FIXES:

* resource/growthbook_environment: Removing `projects` from configuration now clears them in GrowthBook instead of failing apply with "Provider produced inconsistent result after apply".
* resource/growthbook_feature: Fix perpetual diffs on `environments` and `rules` when they are not set in configuration. GrowthBook returns a default per-environment entry for every feature.

## 0.1.0 (September 18, 2026)

FEATURES:

* **New Resource:** `growthbook_project`
* **New Data Source:** `growthbook_project`
* **New Resource:** `growthbook_environment`
* **New Data Source:** `growthbook_environment`
* **New Data Source:** `growthbook_environments`
* **New Resource:** `growthbook_feature`
* **New Data Source:** `growthbook_feature`
* **New Resource:** `growthbook_sdk_connection`
* **New Data Source:** `growthbook_sdk_connection`
* **New Data Source:** `growthbook_sdk_connections`

NOTES:

* An unlicensed GrowthBook organization is limited to one project and no custom environments. Creating a second `growthbook_project`, or an environment beyond the default set, on such an organization returns HTTP 402 from the GrowthBook API and the apply fails.
* `growthbook_sdk_connection` exposes `encrypt_payload` and `hash_secure_attributes`, which require a paid GrowthBook plan. On an unlicensed organization, setting either to `true` fails the apply with HTTP 400 ("requires premium subscription").
* Deleting a `growthbook_feature` performs a hard delete. If GrowthBook refuses because the feature is live (HTTP 403, "archive the feature first"), the provider archives the feature and retries the delete once.
* When `rules` is set on `growthbook_feature`, it is authoritative: each apply sends the full ordered list and replaces every rule on the feature across all environments (each rule carries its own `all_environments` / `environments` scope). Leave `rules` unset to manage rules outside Terraform; set `rules = []` to remove them all.
