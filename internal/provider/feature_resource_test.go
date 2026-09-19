// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	growthbook "github.com/jz-wilson/growthbook-go"
)

// fakeFeatureServer is a minimal in-memory stand-in for GrowthBook's v2
// features API, enough to drive the resource and data source through a
// full create/read/update/import/delete cycle, including the
// archive-then-delete refusal path.
type fakeFeatureServer struct {
	mu       sync.Mutex
	features map[string]*growthbook.Feature
	// archiveRequiredOnce, when set, makes the next DELETE for that id
	// return 403 "archive the feature first" exactly once.
	archiveRequiredOnce map[string]bool
	// denyRulePrerequisites, when set, silently strips rule-level
	// prerequisites from every write instead of storing them, mirroring
	// GrowthBook's real behavior on a sub-Enterprise plan (see
	// requireRulePrerequisitesPersisted).
	denyRulePrerequisites bool
	// denyScheduleRules, when set, silently strips a rule's schedule_rules
	// from every write instead of storing them, mirroring GrowthBook's real
	// behavior on a sub-Pro plan (see requireScheduleRulesPersisted).
	denyScheduleRules bool
}

func newFakeFeatureServer() (*httptest.Server, *fakeFeatureServer) {
	f := &fakeFeatureServer{
		features:            map[string]*growthbook.Feature{},
		archiveRequiredOnce: map[string]bool{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/features", f.handleCollection)
	mux.HandleFunc("/v2/features/", f.handleItem)
	return httptest.NewServer(mux), f
}

func featureWriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func featureWriteAPIError(w http.ResponseWriter, status int, message string) {
	featureWriteJSON(w, status, map[string]any{"message": message})
}

func (f *fakeFeatureServer) handleCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		featureWriteAPIError(w, http.StatusMethodNotAllowed, "unsupported method")
		return
	}
	var req growthbook.FeatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		featureWriteAPIError(w, http.StatusBadRequest, err.Error())
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.features[req.ID]; exists {
		featureWriteAPIError(w, http.StatusBadRequest, "feature already exists")
		return
	}
	feature := &growthbook.Feature{
		ID:           req.ID,
		ValueType:    req.ValueType,
		DefaultValue: req.DefaultValue,
		Tags:         req.Tags,
		DateCreated:  "2026-01-01T00:00:00Z",
		DateUpdated:  "2026-01-01T00:00:00Z",
		Revision:     &growthbook.FeatureRevision{Version: 1},
	}
	f.applyFeatureRequest(feature, req)
	f.features[feature.ID] = feature
	featureWriteJSON(w, http.StatusOK, map[string]any{"feature": feature})
}

func (f *fakeFeatureServer) applyFeatureRequest(feature *growthbook.Feature, req growthbook.FeatureRequest) {
	if req.Description != nil {
		feature.Description = *req.Description
	}
	if req.Project != nil {
		feature.Project = *req.Project
	}
	if req.Owner != nil {
		feature.Owner = *req.Owner
	}
	if req.Archived != nil {
		feature.Archived = *req.Archived
	}
	if req.Environments != nil {
		if feature.Environments == nil {
			feature.Environments = map[string]growthbook.FeatureEnvironment{}
		}
		for name, e := range req.Environments {
			if e.Enabled != nil {
				feature.Environments[name] = growthbook.FeatureEnvironment{Enabled: *e.Enabled}
			}
		}
	}
	if req.Rules != nil {
		rules := *req.Rules
		for i := range rules {
			if rules[i].ID == "" {
				rules[i].ID = fmt.Sprintf("rule_%d", i)
			}
			if f.denyRulePrerequisites {
				rules[i].Prerequisites = nil
			}
			if f.denyScheduleRules {
				rules[i].ScheduleRules = nil
			}
		}
		feature.Rules = rules
	}
	if req.Prerequisites != nil {
		feature.Prerequisites = *req.Prerequisites
	}
}

func (f *fakeFeatureServer) handleItem(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/v2/features/"):]

	f.mu.Lock()
	defer f.mu.Unlock()

	feature, ok := f.features[id]
	switch r.Method {
	case http.MethodGet:
		if !ok {
			featureWriteAPIError(w, http.StatusNotFound, "could not find feature")
			return
		}
		featureWriteJSON(w, http.StatusOK, map[string]any{"feature": feature})
	case http.MethodPost:
		if !ok {
			featureWriteAPIError(w, http.StatusNotFound, "could not find feature")
			return
		}
		var req growthbook.FeatureRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			featureWriteAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.DefaultValue != "" {
			feature.DefaultValue = req.DefaultValue
		}
		f.applyFeatureRequest(feature, req)
		feature.Revision.Version++
		feature.DateUpdated = "2026-01-02T00:00:00Z"
		featureWriteJSON(w, http.StatusOK, map[string]any{"feature": feature})
	case http.MethodDelete:
		if !ok {
			featureWriteAPIError(w, http.StatusNotFound, "could not find feature")
			return
		}
		if f.archiveRequiredOnce[id] {
			delete(f.archiveRequiredOnce, id)
			featureWriteAPIError(w, http.StatusForbidden, "please archive the feature first")
			return
		}
		delete(f.features, id)
		w.WriteHeader(http.StatusOK)
	default:
		featureWriteAPIError(w, http.StatusMethodNotAllowed, "unsupported method")
	}
}

// TestAccFeatureResource_full drives a fake GrowthBook server through
// create, update (description + rule reorder), data source read, import,
// and delete-with-archive-refusal.
func TestAccFeatureResource_full(t *testing.T) {
	server, fake := newFakeFeatureServer()
	defer server.Close()

	fake.mu.Lock()
	fake.archiveRequiredOnce["ft_acc_test"] = true
	fake.mu.Unlock()

	t.Setenv("TF_ACC", "1")
	t.Setenv("GROWTHBOOK_API_KEY", "secret_test")
	t.Setenv("GROWTHBOOK_API_URL", server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fakeAPIProviderConfig() + `
resource "growthbook_feature" "test" {
  id            = "ft_acc_test"
  value_type    = "boolean"
  default_value = "false"
  description   = "initial description"

  environments = {
    production = { enabled = true }
  }

  rules = [
    {
      type             = "force"
      description      = "US force"
      condition        = jsonencode({ country = "US" })
      all_environments = true
      value            = "true"
    },
    {
      type           = "rollout"
      condition      = null
      coverage       = 0.5
      hash_attribute = "id"
      value          = "true"
    },
  ]
}

data "growthbook_feature" "test" {
  id = growthbook_feature.test.id
  depends_on = [growthbook_feature.test]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_feature.test", "description", "initial description"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.#", "2"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.0.type", "force"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "environments.production.enabled", "true"),
					resource.TestCheckResourceAttrSet("growthbook_feature.test", "rules.0.rule_id"),
					resource.TestCheckResourceAttr("data.growthbook_feature.test", "rules.#", "2"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: fakeAPIProviderConfig() + `
resource "growthbook_feature" "test" {
  id            = "ft_acc_test"
  value_type    = "boolean"
  default_value = "false"
  description   = "updated description"

  environments = {
    production = { enabled = true }
  }

  rules = [
    {
      type           = "rollout"
      condition      = null
      coverage       = 0.5
      hash_attribute = "id"
      value          = "true"
    },
    {
      type             = "force"
      description      = "US force"
      condition        = jsonencode({ country = "US" })
      all_environments = true
      value            = "true"
    },
  ]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_feature.test", "description", "updated description"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.0.type", "rollout"),
					resource.TestCheckResourceAttr("growthbook_feature.test", "rules.1.type", "force"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "growthbook_feature.test",
				ImportState:       true,
				ImportStateVerify: true,
				Config: fakeAPIProviderConfig() + `
resource "growthbook_feature" "test" {
  id            = "ft_acc_test"
  value_type    = "boolean"
  default_value = "false"
  description   = "updated description"

  environments = {
    production = { enabled = true }
  }

  rules = [
    {
      type           = "rollout"
      coverage       = 0.5
      hash_attribute = "id"
      value          = "true"
    },
    {
      type             = "force"
      description      = "US force"
      condition        = jsonencode({ country = "US" })
      all_environments = true
      value            = "true"
    },
  ]
}`,
			},
		},
	})
}

// TestAccFeatureResource_prerequisites drives a fake GrowthBook server
// through adding feature-level (a set of feature IDs) and rule-level
// ({id, condition}) prerequisites, changing the rule-level condition, and
// then removing both, asserting each step converges to an empty plan and
// that removal actually clears the server-side value rather than leaving
// it behind.
func TestAccFeatureResource_prerequisites(t *testing.T) {
	server, _ := newFakeFeatureServer()
	defer server.Close()

	t.Setenv("TF_ACC", "1")
	t.Setenv("GROWTHBOOK_API_KEY", "secret_test")
	t.Setenv("GROWTHBOOK_API_URL", server.URL)

	parent := `
resource "growthbook_feature" "parent" {
  id            = "ft_prereq_parent"
  value_type    = "boolean"
  default_value = "false"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fakeAPIProviderConfig() + parent + `
resource "growthbook_feature" "child" {
  id            = "ft_prereq_child"
  value_type    = "boolean"
  default_value = "false"

  prerequisites = [growthbook_feature.parent.id]

  rules = [
    {
      type             = "force"
      all_environments = true
      value            = "true"
      prerequisites = [
        {
          id        = growthbook_feature.parent.id
          condition = jsonencode({ value = true })
        },
      ]
    },
  ]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_feature.child", "prerequisites.#", "1"),
					resource.TestCheckTypeSetElemAttr("growthbook_feature.child", "prerequisites.*", "ft_prereq_parent"),
					resource.TestCheckResourceAttr("growthbook_feature.child", "rules.0.prerequisites.#", "1"),
					resource.TestCheckResourceAttr("growthbook_feature.child", "rules.0.prerequisites.0.condition", `{"value":true}`),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// Change the rule-level condition.
				Config: fakeAPIProviderConfig() + parent + `
resource "growthbook_feature" "child" {
  id            = "ft_prereq_child"
  value_type    = "boolean"
  default_value = "false"

  prerequisites = [growthbook_feature.parent.id]

  rules = [
    {
      type             = "force"
      all_environments = true
      value            = "true"
      prerequisites = [
        {
          id        = growthbook_feature.parent.id
          condition = jsonencode({ value = false })
        },
      ]
    },
  ]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_feature.child", "rules.0.prerequisites.0.condition", `{"value":false}`),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// Remove both: the empty lists must clear the server-side
				// values, not leave them behind.
				Config: fakeAPIProviderConfig() + parent + `
resource "growthbook_feature" "child" {
  id            = "ft_prereq_child"
  value_type    = "boolean"
  default_value = "false"

  prerequisites = []

  rules = [
    {
      type             = "force"
      all_environments = true
      value            = "true"
      prerequisites    = []
    },
  ]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_feature.child", "prerequisites.#", "0"),
					resource.TestCheckResourceAttr("growthbook_feature.child", "rules.0.prerequisites.#", "0"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "growthbook_feature.child",
				ImportState:       true,
				ImportStateVerify: true,
				Config: fakeAPIProviderConfig() + parent + `
resource "growthbook_feature" "child" {
  id            = "ft_prereq_child"
  value_type    = "boolean"
  default_value = "false"

  prerequisites = []

  rules = [
    {
      type             = "force"
      all_environments = true
      value            = "true"
      prerequisites    = []
    },
  ]
}
`,
			},
		},
	})
}

// TestAccFeatureResource_unconfiguredEnvironmentsStayUnmanaged proves that
// leaving `environments` (and `rules`) unset in config stays unmanaged
// through a refresh, even when the API's response includes a default
// per-environment entry the request never asked for - real GrowthBook does
// exactly this (every feature gets a "production" entry regardless of what
// was requested). This uses its own minimal server, separate from
// fakeFeatureServer, so the default doesn't change what every other test
// in this file sees on import. Before the fix, Read() only nulled
// rules/environments out when the API happened to echo back nothing, so a
// never-configured attribute with a non-empty default would drift forever.
func TestAccFeatureResource_unconfiguredEnvironmentsStayUnmanaged(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/features", func(w http.ResponseWriter, r *http.Request) {
		featureWriteJSON(w, http.StatusOK, map[string]any{"feature": defaultEnvFeature})
	})
	mux.HandleFunc("/v2/features/", func(w http.ResponseWriter, r *http.Request) {
		featureWriteJSON(w, http.StatusOK, map[string]any{"feature": defaultEnvFeature})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Setenv("TF_ACC", "1")
	t.Setenv("GROWTHBOOK_API_KEY", "secret_test")
	t.Setenv("GROWTHBOOK_API_URL", server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fakeAPIProviderConfig() + `
resource "growthbook_feature" "bare" {
  id            = "ft_bare"
  value_type    = "boolean"
  default_value = "false"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("growthbook_feature.bare", "environments.%"),
					resource.TestCheckNoResourceAttr("growthbook_feature.bare", "rules.#"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// defaultEnvFeature is the canned response TestAccFeatureResource_
// unconfiguredEnvironmentsStayUnmanaged's server always returns: a feature
// with a "production" environment entry despite no request ever having
// asked for one, matching real GrowthBook's default.
var defaultEnvFeature = growthbook.Feature{
	ID:           "ft_bare",
	ValueType:    "boolean",
	DefaultValue: "false",
	Environments: map[string]growthbook.FeatureEnvironment{"production": {Enabled: false}},
	Revision:     &growthbook.FeatureRevision{Version: 1},
}

// TestAccFeatureResource_rulePrerequisitesEnterpriseErrorTracksState proves
// that requireRulePrerequisitesPersisted's error doesn't orphan the
// feature: Create still calls resp.State.Set before appending the error,
// so the resource is tracked (tainted) rather than left in GrowthBook with
// no Terraform record of it. A follow-up apply that removes the rule-level
// prerequisites succeeds as an Update (not "feature already exists"), and
// CheckDestroy confirms the fake server no longer has the feature once the
// test's automatic destroy runs.
func TestAccFeatureResource_rulePrerequisitesEnterpriseErrorTracksState(t *testing.T) {
	server, fake := newFakeFeatureServer()
	defer server.Close()

	fake.mu.Lock()
	fake.denyRulePrerequisites = true
	fake.mu.Unlock()

	t.Setenv("TF_ACC", "1")
	t.Setenv("GROWTHBOOK_API_KEY", "secret_test")
	t.Setenv("GROWTHBOOK_API_URL", server.URL)

	const deniedID = "ft_prereq_denied"
	ruleWithPrereq := `
  rules = [
    {
      type             = "force"
      all_environments = true
      value            = "true"
      prerequisites = [
        { id = "some_parent", condition = jsonencode({ value = true }) },
      ]
    },
  ]
`
	ruleWithoutPrereq := `
  rules = [
    {
      type             = "force"
      all_environments = true
      value            = "true"
    },
  ]
`
	featureConfig := func(rules string) string {
		return fakeAPIProviderConfig() + fmt.Sprintf(`
resource "growthbook_feature" "denied" {
  id            = %q
  value_type    = "boolean"
  default_value = "false"

  %s
}
`, deniedID, rules)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			fake.mu.Lock()
			defer fake.mu.Unlock()
			if _, ok := fake.features[deniedID]; ok {
				return fmt.Errorf("feature %q still exists on the fake server after destroy", deniedID)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config:      featureConfig(ruleWithPrereq),
				ExpectError: regexp.MustCompile(`Enterprise plan`),
			},
			{
				// If Create hadn't tracked state, this would fail with
				// "feature already exists" instead of applying as an
				// Update.
				Config: featureConfig(ruleWithoutPrereq),
				Check:  resource.TestCheckResourceAttr("growthbook_feature.denied", "rules.0.type", "force"),
			},
		},
	})
}

func fakeAPIProviderConfig() string {
	return `provider "growthbook" {}
`
}
