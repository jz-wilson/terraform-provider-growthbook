// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	growthbook "github.com/jz-wilson/growthbook-go"
)

// fakeProjectServer is a minimal in-memory GrowthBook double covering the
// four project endpoints, so full CRUD + import can be exercised without a
// live (and license-limited) GrowthBook instance.
type fakeProjectServer struct {
	mu       sync.Mutex
	projects map[string]growthbook.Project
	nextID   int
}

func newFakeProjectServer() *httptest.Server {
	f := &fakeProjectServer{projects: map[string]growthbook.Project{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects", f.handleCollection)
	mux.HandleFunc("/api/v1/projects/", f.handleItem)
	return httptest.NewServer(mux)
}

func (f *fakeProjectServer) handleCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"message":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var req growthbook.ProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"message":"bad request"}`, http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	f.nextID++
	id := "prj_test" + strconv.Itoa(f.nextID)
	p := growthbook.Project{
		ID:          id,
		Name:        req.Name,
		PublicID:    "pub_" + id,
		DateCreated: "2026-01-01T00:00:00Z",
		DateUpdated: "2026-01-01T00:00:00Z",
	}
	applyRequest(&p, req)
	f.projects[id] = p
	f.mu.Unlock()

	writeProject(w, p)
}

func (f *fakeProjectServer) handleItem(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/projects/"):]

	f.mu.Lock()
	defer f.mu.Unlock()

	p, ok := f.projects[id]
	switch r.Method {
	case http.MethodGet:
		if !ok {
			http.Error(w, `{"message":"Could not find project"}`, http.StatusBadRequest)
			return
		}
		writeProject(w, p)
	case http.MethodPut:
		if !ok {
			http.Error(w, `{"message":"Could not find project"}`, http.StatusBadRequest)
			return
		}
		var req growthbook.ProjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"message":"bad request"}`, http.StatusBadRequest)
			return
		}
		if req.Name != "" {
			p.Name = req.Name
		}
		applyRequest(&p, req)
		p.DateUpdated = "2026-01-02T00:00:00Z"
		f.projects[id] = p
		writeProject(w, p)
	case http.MethodDelete:
		if !ok {
			http.Error(w, `{"message":"Could not find project"}`, http.StatusBadRequest)
			return
		}
		delete(f.projects, id)
		w.Write([]byte(`{"deletedId":"` + id + `"}`)) //nolint:errcheck
	default:
		http.Error(w, `{"message":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func applyRequest(p *growthbook.Project, req growthbook.ProjectRequest) {
	if req.Description != nil {
		p.Description = *req.Description
	}
	if req.PublicID != nil {
		p.PublicID = *req.PublicID
	}
	if req.RestrictAccess != nil {
		p.RestrictAccess = req.RestrictAccess
	}
	if req.Settings != nil {
		p.Settings = req.Settings
	}
}

func writeProject(w http.ResponseWriter, p growthbook.Project) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"project": p})
}

// TestAccProjectResource_httptest covers create, update, import and destroy
// against the fake server above, so it needs neither Docker nor a licensed
// GrowthBook instance and runs in any environment with TF_ACC set.
func TestAccProjectResource_httptest(t *testing.T) {
	srv := newFakeProjectServer()
	defer srv.Close()

	t.Setenv("TF_ACC", "1")
	t.Setenv("GROWTHBOOK_API_URL", srv.URL+"/api")
	t.Setenv("GROWTHBOOK_API_KEY", "secret_test")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "growthbook_project" "test" {
  name        = "tf-acc-project"
  description = "created by acceptance test"
  settings = {
    stats_engine      = "bayesian"
    confidence_level  = 0.95
    p_value_threshold = 0.05
  }
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("growthbook_project.test", "id"),
					resource.TestCheckResourceAttr("growthbook_project.test", "name", "tf-acc-project"),
					resource.TestCheckResourceAttr("growthbook_project.test", "description", "created by acceptance test"),
					resource.TestCheckResourceAttrSet("growthbook_project.test", "public_id"),
					resource.TestCheckResourceAttr("growthbook_project.test", "settings.stats_engine", "bayesian"),
					resource.TestCheckResourceAttr("growthbook_project.test", "settings.confidence_level", "0.95"),
				),
			},
			{
				Config: `
data "growthbook_project" "test" {
  id = growthbook_project.test.id
}

resource "growthbook_project" "test" {
  name        = "tf-acc-project"
  description = "updated by acceptance test"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_project.test", "description", "updated by acceptance test"),
					resource.TestCheckResourceAttrPair("data.growthbook_project.test", "id", "growthbook_project.test", "id"),
					resource.TestCheckResourceAttrPair("data.growthbook_project.test", "name", "growthbook_project.test", "name"),
					resource.TestCheckResourceAttrPair("data.growthbook_project.test", "description", "growthbook_project.test", "description"),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				ResourceName:      "growthbook_project.test",
				ImportState:       true,
				ImportStateVerify: true,
				Config: `
resource "growthbook_project" "test" {
  name        = "tf-acc-project"
  description = "updated by acceptance test"
}`,
			},
		},
	})
}
