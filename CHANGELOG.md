# Changelog

## 0.2.0 (Unreleased)

FEATURES:

* **New Resource:** `growthbook_attribute`
* **New Data Source:** `growthbook_attribute`
* **`growthbook_feature`:** Add feature-level `prerequisites` (a set of feature IDs) and rule-level `prerequisites` (`id`, `condition`), gating a feature or a rule on another feature's value. Rule-level prerequisites require GrowthBook Enterprise.

BUG FIXES:

* **`growthbook_feature`:** Fix `rules`/`environments` reading back as non-null and drifting forever when neither was ever set in config. GrowthBook gives every feature a default per-environment entry (e.g. `production`) even when a feature never configures `environments`, so a plan check that only cleared these attributes when the API's response was empty missed that case and reported spurious changes on every subsequent plan.

BUG FIXES:

* `growthbook_environment`: removing `projects` from config now clears it on GrowthBook instead of leaving the prior list in place and failing apply with "provider produced inconsistent result after apply".

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
