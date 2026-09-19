// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	growthbook "github.com/jz-wilson/growthbook-go"
)

// fakeSavedGroupServer is a minimal in-memory stand-in for the GrowthBook
// /v1/saved-groups endpoints, sufficient to drive a full CRUD acceptance test
// without a live GrowthBook instance.
type fakeSavedGroupServer struct {
	mu     sync.Mutex
	nextID int
	byID   map[string]map[string]any
	// posts records every update request body (POST /v1/saved-groups/{id}),
	// in order.
	posts []map[string]any
}

func newFakeSavedGroupServer() *fakeSavedGroupServer {
	return &fakeSavedGroupServer{byID: map[string]map[string]any{}}
}

func (f *fakeSavedGroupServer) httptestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/saved-groups", f.handleCollection)
	mux.HandleFunc("/v1/saved-groups/", f.handleItem)
	return httptest.NewServer(mux)
}

func (f *fakeSavedGroupServer) handleCollection(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		list := make([]map[string]any, 0, len(f.byID))
		for _, v := range f.byID {
			list = append(list, v)
		}
		sdkWriteJSON(w, map[string]any{"savedGroups": list})
	case http.MethodPost:
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.nextID++
		id := fmt.Sprintf("sg_%d", f.nextID)
		group := savedGroupFromRequest(id, req, nil)
		f.byID[id] = group
		sdkWriteJSON(w, map[string]any{"savedGroup": group})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (f *fakeSavedGroupServer) handleItem(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	id := r.URL.Path[len("/v1/saved-groups/"):]
	if strings.HasSuffix(id, "/archive") {
		f.handleArchive(w, strings.TrimSuffix(id, "/archive"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		group, ok := f.byID[id]
		if !ok {
			sdkWriteAPIError(w)
			return
		}
		sdkWriteJSON(w, map[string]any{"savedGroup": group})
	case http.MethodPost:
		existing, ok := f.byID[id]
		if !ok {
			sdkWriteAPIError(w)
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// additionalProperties: false on the real API's update schema: type
		// and attributeKey are create-only and must never appear here.
		for _, k := range []string{"type", "attributeKey"} {
			if _, ok := req[k]; ok {
				http.Error(w, fmt.Sprintf("update must not send %q", k), http.StatusBadRequest)
				return
			}
		}
		f.posts = append(f.posts, req)
		merged := savedGroupFromRequest(id, req, existing)
		f.byID[id] = merged
		sdkWriteJSON(w, map[string]any{"savedGroup": merged})
	case http.MethodDelete:
		group, ok := f.byID[id]
		if !ok {
			sdkWriteAPIError(w)
			return
		}
		// Mirrors the real API: refuses to delete a saved group that isn't
		// archived first.
		if archived, _ := group["archived"].(bool); !archived {
			http.Error(w, "must be archived before it can be deleted", http.StatusBadRequest)
			return
		}
		delete(f.byID, id)
		sdkWriteJSON(w, map[string]any{"deletedId": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleArchive serves POST /v1/saved-groups/{id}/archive.
func (f *fakeSavedGroupServer) handleArchive(w http.ResponseWriter, id string) {
	group, ok := f.byID[id]
	if !ok {
		sdkWriteAPIError(w)
		return
	}
	group["archived"] = true
	f.byID[id] = group
	sdkWriteJSON(w, map[string]any{"savedGroup": group})
}

// savedGroupFromRequest builds the stored/response representation of a saved
// group. existing is nil on create; on update it supplies the fields the
// update request schema does not accept (type, attributeKey, dateCreated).
func savedGroupFromRequest(id string, req map[string]any, existing map[string]any) map[string]any {
	group := map[string]any{
		"id":          id,
		"dateCreated": "2026-01-01T00:00:00.000Z",
		"dateUpdated": "2026-01-01T00:00:00.000Z",
		"name":        req["name"],
	}
	if existing != nil {
		group["type"] = existing["type"]
		group["attributeKey"] = existing["attributeKey"]
		group["dateCreated"] = existing["dateCreated"]
	} else {
		group["type"] = req["type"]
		group["attributeKey"] = sdkStringOr(req["attributeKey"], "")
	}
	if v, ok := req["condition"]; ok {
		group["condition"] = v
	} else if existing != nil {
		group["condition"] = existing["condition"]
	}
	if v, ok := req["values"]; ok {
		group["values"] = v
	} else if existing != nil {
		group["values"] = existing["values"]
	}
	if v, ok := req["owner"]; ok {
		group["owner"] = v
	} else if existing != nil {
		group["owner"] = existing["owner"]
	} else {
		group["owner"] = ""
	}
	if v, ok := req["projects"]; ok {
		group["projects"] = v
	} else if existing != nil {
		group["projects"] = existing["projects"]
	}
	return group
}

// TestAccSavedGroupResource_fakeList drives full CRUD (create, update,
// import, destroy) for a type="list" saved group against an httptest-backed
// fake GrowthBook API.
func TestAccSavedGroupResource_fakeList(t *testing.T) {
	fake := newFakeSavedGroupServer()
	srv := fake.httptestServer()
	defer srv.Close()

	t.Setenv("TF_ACC", "1")
	t.Setenv("GROWTHBOOK_API_KEY", "secret_test")
	t.Setenv("GROWTHBOOK_API_URL", srv.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "growthbook_saved_group" "test" {
  name          = "tf-acc-saved-group"
  type          = "list"
  attribute_key = "userId"
  values        = ["user-1", "user-2"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "name", "tf-acc-saved-group"),
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "type", "list"),
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "attribute_key", "userId"),
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "values.#", "2"),
					resource.TestCheckResourceAttrSet("growthbook_saved_group.test", "id"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: `
resource "growthbook_saved_group" "test" {
  name          = "tf-acc-saved-group-renamed"
  type          = "list"
  attribute_key = "userId"
  values        = ["user-3"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "name", "tf-acc-saved-group-renamed"),
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "values.#", "1"),
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "values.0", "user-3"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// Omitting values entirely (unconfigured, distinct from an
				// explicit empty set) must leave the server's list
				// untouched, not clear it: Values is Optional+Computed, so
				// an unconfigured plan is unknown and the update request
				// omits the field rather than sending "[]".
				Config: `
resource "growthbook_saved_group" "test" {
  name          = "tf-acc-saved-group-renamed"
  type          = "list"
  attribute_key = "userId"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "values.#", "1"),
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "values.0", "user-3"),
				),
			},
			{
				// Clearing values to an explicit empty set must clear it on
				// the server, not leave the prior list untouched.
				Config: `
resource "growthbook_saved_group" "test" {
  name          = "tf-acc-saved-group-renamed"
  type          = "list"
  attribute_key = "userId"
  values        = []
}
`,
				Check: resource.TestCheckResourceAttr("growthbook_saved_group.test", "values.#", "0"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "growthbook_saved_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccSavedGroupResource_fakeCondition drives create/update for a
// type="condition" saved group.
func TestAccSavedGroupResource_fakeCondition(t *testing.T) {
	fake := newFakeSavedGroupServer()
	srv := fake.httptestServer()
	defer srv.Close()

	t.Setenv("TF_ACC", "1")
	t.Setenv("GROWTHBOOK_API_KEY", "secret_test")
	t.Setenv("GROWTHBOOK_API_URL", srv.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "growthbook_saved_group" "test" {
  name      = "tf-acc-saved-group-cond"
  type      = "condition"
  condition = "{\"country\":\"US\"}"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "type", "condition"),
					resource.TestCheckResourceAttr("growthbook_saved_group.test", "condition", `{"country":"US"}`),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: `
resource "growthbook_saved_group" "test" {
  name      = "tf-acc-saved-group-cond"
  type      = "condition"
  condition = "{\"country\":\"CA\"}"
}
`,
				Check: resource.TestCheckResourceAttr("growthbook_saved_group.test", "condition", `{"country":"CA"}`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "growthbook_saved_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccSavedGroupResource_updateOmitsUnconfigured checks that an update
// never sends type or attributeKey (the API rejects both on update), and
// omits owner/projects when left unconfigured.
func TestAccSavedGroupResource_updateOmitsUnconfigured(t *testing.T) {
	fake := newFakeSavedGroupServer()
	srv := fake.httptestServer()
	defer srv.Close()

	t.Setenv("TF_ACC", "1")
	t.Setenv("GROWTHBOOK_API_KEY", "secret_test")
	t.Setenv("GROWTHBOOK_API_URL", srv.URL)

	config := func(name string) string {
		return fmt.Sprintf(`
resource "growthbook_saved_group" "test" {
  name          = %q
  type          = "list"
  attribute_key = "userId"
}
`, name)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config("tf-acc-saved-group")},
			{
				Config: config("tf-acc-saved-group-renamed"),
				Check: func(_ *terraform.State) error {
					fake.mu.Lock()
					defer fake.mu.Unlock()
					if len(fake.posts) != 1 {
						return fmt.Errorf("got %d update requests, want 1", len(fake.posts))
					}
					post := fake.posts[0]
					for _, k := range []string{"type", "attributeKey", "owner", "projects"} {
						if v, ok := post[k]; ok {
							return fmt.Errorf("update sent unconfigured/forbidden field %s = %v", k, v)
						}
					}
					if post["name"] != "tf-acc-saved-group-renamed" {
						return fmt.Errorf("update name = %v, want tf-acc-saved-group-renamed", post["name"])
					}
					return nil
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestFakeSavedGroupServer_deleteRequiresArchive checks the fake server's
// contract mirrors the real API: DELETE fails until the group is archived,
// and ArchiveSavedGroup then DeleteSavedGroup (SavedGroupResource.Delete's
// sequence) succeeds.
func TestFakeSavedGroupServer_deleteRequiresArchive(t *testing.T) {
	fake := newFakeSavedGroupServer()
	srv := fake.httptestServer()
	defer srv.Close()

	c, err := growthbook.New(growthbook.Credentials{APIKey: "secret_test", APIURL: srv.URL})
	if err != nil {
		t.Fatalf("growthbook.New() error = %v", err)
	}
	ctx := context.Background()

	created, err := c.CreateSavedGroup(ctx, growthbook.SavedGroupRequest{Name: "n", Type: "list", AttributeKey: "userId"})
	if err != nil {
		t.Fatalf("CreateSavedGroup() error = %v", err)
	}

	if err := c.DeleteSavedGroup(ctx, created.ID); err == nil {
		t.Fatal("DeleteSavedGroup() before archive: got nil error, want a failure")
	}

	if _, err := c.ArchiveSavedGroup(ctx, created.ID); err != nil {
		t.Fatalf("ArchiveSavedGroup() error = %v", err)
	}
	if err := c.DeleteSavedGroup(ctx, created.ID); err != nil {
		t.Fatalf("DeleteSavedGroup() after archive: error = %v", err)
	}
}
