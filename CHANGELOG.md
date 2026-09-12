# Changelog

## 0.1.0 (Unreleased)

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
* `growthbook_sdk_connection` exposes `encrypt_payload` and `hash_secure_attributes`. Both are premium-gated in GrowthBook; setting them on an unlicensed organization is accepted by the schema but the API may reject or ignore the value depending on plan.
* Deleting a `growthbook_feature` archives it in GrowthBook before removal rather than performing a hard delete, matching the API's own delete semantics.
* The `rules` attribute on `growthbook_feature` is authoritative per environment: applying a configuration replaces the entire rule list for that environment rather than merging individual rules.
