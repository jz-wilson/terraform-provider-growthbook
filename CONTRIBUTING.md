# Contributing

## Development setup

Requires:

* Go, at the version pinned in `go.mod`.
* Terraform or OpenTofu, for running acceptance tests and `tfplugindocs` generation.
* `golangci-lint`, for `make lint`.

Common targets:

```bash
make build      # compile the provider
make test       # unit tests (no live GrowthBook, no TF_ACC)
make testacc    # acceptance tests (TF_ACC=1)
make lint       # golangci-lint
make fmt        # gofmt
make generate   # regenerate docs/ from schemas and examples via tfplugindocs
```

To exercise a locally built provider from a Terraform or OpenTofu config, add a
`dev_overrides` block to `~/.terraformrc` pointing `jz-wilson/growthbook` at
`$GOBIN`, then `make install`.

## Test tiers

The test suite has three tiers, in increasing order of what they require:

1. **Unit tests** (`make test`). Plain Go tests with no external dependencies.
   Run on every PR.
2. **Fake-server acceptance tests** (`TF_ACC=1 go test ./...`, or `make testacc`
   without a live backend configured). These drive the real Terraform Plugin
   Framework test harness against an in-process fake GrowthBook server, so
   they exercise the full create/read/update/delete/import cycle without
   needing real infrastructure.
3. **Live acceptance tests** (`GROWTHBOOK_LIVE=1` in addition to `TF_ACC=1`).
   These run against a real GrowthBook instance. Bring one up locally with the
   compose file and bootstrap script under `e2e/`:

   ```bash
   docker compose -f e2e/docker-compose.yml up -d --wait
   export GROWTHBOOK_API_KEY=$(e2e/bootstrap.sh)
   export GROWTHBOOK_API_URL=http://localhost:3100/api
   export GROWTHBOOK_LIVE=1
   make testacc
   ```

   An unlicensed GrowthBook instance enforces the free plan (one project, no
   custom environments) and returns HTTP 402 for anything beyond that. Tests
   that need more than the free plan allows detect the 402 and skip, or adapt
   their assertions, rather than failing.

## Changelog rule

Every user-facing pull request must add an entry to `CHANGELOG.md` under the
`Unreleased` header, in the format described in HashiCorp's [Versioning and
changelogs](https://developer.hashicorp.com/terraform/plugin/best-practices/versioning)
guide. A pull request that only touches internal tooling, tests, or CI can
skip the changelog.

## Versioning and deprecation policy

This provider follows [Semantic Versioning](https://semver.org/), applied the
way HashiCorp's versioning guide defines it for Terraform providers: a version
bump is measured against practitioner-visible state and configuration, not
against internal Go APIs.

* **Patch** releases are bug fixes only, functionally equivalent to the prior
  patch release.
* **Minor** releases add new resources, data sources, or attributes, or mark
  something as deprecated, without breaking existing configurations or state.
* **Major** releases contain breaking changes: removing or renaming a
  resource, data source, or attribute; changing an import ID or resource ID
  format; changing an attribute's type or format in a functionally
  incompatible way; or changing a default in a way that is incompatible with
  existing state. Major releases are cut no more than once per year, to give
  practitioners a reasonable upgrade window.

**Deprecations** are announced in a minor release, never a patch:

1. The attribute or resource is marked with a `DeprecationMessage` in its
   schema, pointing at the replacement.
2. A corresponding `NOTES` entry is added to `CHANGELOG.md` for that release.
3. The deprecated attribute or resource is kept functional for at least one
   further minor release after the one that announced the deprecation.
4. It is removed only in the next major release, never in a minor or patch.

**Pre-1.0 caveat:** while this provider is at `0.x`, minor releases may
contain changes that would otherwise require a major bump under strict semver.
Any such change is called out explicitly in a `NOTES` entry in `CHANGELOG.md`
so it is not silently absorbed into a routine minor upgrade. Once the provider
reaches `1.0.0`, the major/minor/patch rules above apply without this
exception.
