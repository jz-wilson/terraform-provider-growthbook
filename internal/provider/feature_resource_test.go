// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

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

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"message": message})
}

func (f *fakeFeatureServer) handleCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "unsupported method")
		return
	}
	var req growthbook.FeatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.features[req.ID]; exists {
		writeAPIError(w, http.StatusBadRequest, "feature already exists")
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
	applyRequest(feature, req)
	f.features[feature.ID] = feature
	writeJSON(w, http.StatusOK, map[string]any{"feature": feature})
}

func applyRequest(feature *growthbook.Feature, req growthbook.FeatureRequest) {
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
		}
		feature.Rules = rules
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
			writeAPIError(w, http.StatusNotFound, "could not find feature")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"feature": feature})
	case http.MethodPost:
		if !ok {
			writeAPIError(w, http.StatusNotFound, "could not find feature")
			return
		}
		var req growthbook.FeatureRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.DefaultValue != "" {
			feature.DefaultValue = req.DefaultValue
		}
		applyRequest(feature, req)
		feature.Revision.Version++
		feature.DateUpdated = "2026-01-02T00:00:00Z"
		writeJSON(w, http.StatusOK, map[string]any{"feature": feature})
	case http.MethodDelete:
		if !ok {
			writeAPIError(w, http.StatusNotFound, "could not find feature")
			return
		}
		if f.archiveRequiredOnce[id] {
			delete(f.archiveRequiredOnce, id)
			writeAPIError(w, http.StatusForbidden, "please archive the feature first")
			return
		}
		delete(f.features, id)
		w.WriteHeader(http.StatusOK)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "unsupported method")
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

func fakeAPIProviderConfig() string {
	return `provider "growthbook" {}
`
}
