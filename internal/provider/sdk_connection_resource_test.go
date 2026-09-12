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
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// fakeSDKConnectionServer is a minimal in-memory stand-in for the GrowthBook
// /v1/sdk-connections endpoints, sufficient to drive a full CRUD acceptance
// test without a live GrowthBook instance.
type fakeSDKConnectionServer struct {
	mu     sync.Mutex
	nextID int
	byID   map[string]map[string]any
}

func newFakeSDKConnectionServer() *fakeSDKConnectionServer {
	return &fakeSDKConnectionServer{byID: map[string]map[string]any{}}
}

func (f *fakeSDKConnectionServer) httptestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sdk-connections", f.handleCollection)
	mux.HandleFunc("/v1/sdk-connections/", f.handleItem)
	return httptest.NewServer(mux)
}

func (f *fakeSDKConnectionServer) handleCollection(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		list := make([]map[string]any, 0, len(f.byID))
		for _, v := range f.byID {
			list = append(list, v)
		}
		sdkWriteJSON(w, map[string]any{"connections": list})
	case http.MethodPost:
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.nextID++
		id := fmt.Sprintf("sdk_%d", f.nextID)
		conn := sdkConnectionFromRequest(id, req)
		f.byID[id] = conn
		sdkWriteJSON(w, map[string]any{"sdkConnection": conn})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (f *fakeSDKConnectionServer) handleItem(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	id := r.URL.Path[len("/v1/sdk-connections/"):]

	switch r.Method {
	case http.MethodGet:
		conn, ok := f.byID[id]
		if !ok {
			sdkWriteAPIError(w, http.StatusNotFound, "not found")
			return
		}
		sdkWriteJSON(w, map[string]any{"sdkConnection": conn})
	case http.MethodPut:
		existing, ok := f.byID[id]
		if !ok {
			sdkWriteAPIError(w, http.StatusNotFound, "not found")
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		merged := sdkConnectionFromRequest(id, req)
		// Preserve generated secrets across update, mirroring the real API.
		merged["key"] = existing["key"]
		merged["encryptionKey"] = existing["encryptionKey"]
		merged["proxySigningKey"] = existing["proxySigningKey"]
		merged["dateCreated"] = existing["dateCreated"]
		f.byID[id] = merged
		sdkWriteJSON(w, map[string]any{"sdkConnection": merged})
	case http.MethodDelete:
		if _, ok := f.byID[id]; !ok {
			sdkWriteAPIError(w, http.StatusNotFound, "not found")
			return
		}
		delete(f.byID, id)
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func sdkConnectionFromRequest(id string, req map[string]any) map[string]any {
	language, _ := req["language"].(string)
	conn := map[string]any{
		"id":              id,
		"dateCreated":     "2026-01-01T00:00:00.000Z",
		"dateUpdated":     "2026-01-01T00:00:00.000Z",
		"name":            req["name"],
		"organization":    "org_test",
		"languages":       []string{language},
		"environment":     req["environment"],
		"project":         "",
		"encryptPayload":  sdkBoolOr(req["encryptPayload"], false),
		"encryptionKey":   "enc_" + id,
		"key":             "sdk-" + id,
		"proxyEnabled":    sdkBoolOr(req["proxyEnabled"], false),
		"proxyHost":       sdkStringOr(req["proxyHost"], ""),
		"proxySigningKey": "proxysign_" + id,
		"connected":       false,
	}
	if v, ok := req["sdkVersion"]; ok {
		conn["sdkVersion"] = v
	}
	if v, ok := req["projects"]; ok {
		conn["projects"] = v
	}
	if v, ok := req["includeVisualExperiments"]; ok {
		conn["includeVisualExperiments"] = v
	}
	if v, ok := req["includeDraftExperiments"]; ok {
		conn["includeDraftExperiments"] = v
	}
	if v, ok := req["includeExperimentNames"]; ok {
		conn["includeExperimentNames"] = v
	}
	if v, ok := req["includeRedirectExperiments"]; ok {
		conn["includeRedirectExperiments"] = v
	}
	if v, ok := req["includeRuleIds"]; ok {
		conn["includeRuleIds"] = v
	}
	if v, ok := req["hashSecureAttributes"]; ok {
		conn["hashSecureAttributes"] = v
	}
	if v, ok := req["remoteEvalEnabled"]; ok {
		conn["remoteEvalEnabled"] = v
	}
	return conn
}

func sdkBoolOr(v any, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

func sdkStringOr(v any, def string) string {
	if s, ok := v.(string); ok {
		return s
	}
	return def
}

func sdkWriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func sdkWriteAPIError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"message": message})
}

// TestAccSDKConnectionResource_fake drives full CRUD (create, update,
// import, destroy) against an httptest-backed fake GrowthBook API. It runs
// locally under TF_ACC=1 without needing a live GrowthBook instance.
func TestAccSDKConnectionResource_fake(t *testing.T) {
	fake := newFakeSDKConnectionServer()
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
resource "growthbook_sdk_connection" "test" {
  name        = "tf-acc-sdk-conn"
  language    = "javascript"
  environment = "production"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_sdk_connection.test", "name", "tf-acc-sdk-conn"),
					resource.TestCheckResourceAttr("growthbook_sdk_connection.test", "language", "javascript"),
					resource.TestCheckResourceAttr("growthbook_sdk_connection.test", "environment", "production"),
					resource.TestCheckResourceAttrSet("growthbook_sdk_connection.test", "id"),
					resource.TestCheckResourceAttrSet("growthbook_sdk_connection.test", "key"),
					resource.TestCheckResourceAttrSet("growthbook_sdk_connection.test", "encryption_key"),
					resource.TestCheckResourceAttrSet("growthbook_sdk_connection.test", "proxy_signing_key"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: `
resource "growthbook_sdk_connection" "test" {
  name                = "tf-acc-sdk-conn-updated"
  language            = "javascript"
  environment         = "production"
  encrypt_payload     = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_sdk_connection.test", "name", "tf-acc-sdk-conn-updated"),
					resource.TestCheckResourceAttr("growthbook_sdk_connection.test", "encrypt_payload", "true"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "growthbook_sdk_connection.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
