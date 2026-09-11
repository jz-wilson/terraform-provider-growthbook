// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccProjectResource_live imports the organization's existing default
// project rather than creating one. A licensed-free GrowthBook instance
// allows exactly one project ("My First Project", created automatically);
// CreateProject on such a plan returns HTTP 402. So this test finds that
// project, imports it, edits and reverts its description, then removes it
// from state (without deleting the real project) via a `removed` block so
// the framework's end-of-test destroy has nothing left to destroy.
//
// Gated on GROWTHBOOK_LIVE=1, set by the CI acceptance job; skipped locally
// by default so `go test` needs no live GrowthBook.
func TestAccProjectResource_live(t *testing.T) {
	if os.Getenv("GROWTHBOOK_LIVE") != "1" {
		t.Skip("set GROWTHBOOK_LIVE=1 to run against a live GrowthBook instance")
	}
	testAccPreCheck(t)

	id, name, description := findExistingProject(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		// The imported project is never created or destroyed by this test,
		// so there is nothing for the framework to clean up.
		CheckDestroy: func(*terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{
				// ImportStateVerify has nothing to compare against on a bare
				// import step (no prior Config-created resource is in
				// state), so it is skipped here; the imported id/name are
				// checked explicitly once the resource is in state below.
				Config:             fmt.Sprintf("resource \"growthbook_project\" \"imported\" {\n  name = %q\n}", name),
				ResourceName:       "growthbook_project.imported",
				ImportState:        true,
				ImportStateId:      id,
				ImportStatePersist: true,
			},
			{
				Config: fmt.Sprintf(`
resource "growthbook_project" "imported" {
  name        = %q
  description = "set by terraform-provider-growthbook acceptance test"
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_project.imported", "id", id),
					resource.TestCheckResourceAttr("growthbook_project.imported", "name", name),
					resource.TestCheckResourceAttr("growthbook_project.imported", "description", "set by terraform-provider-growthbook acceptance test"),
				),
			},
			{
				// description is Optional+Computed, so reverting requires
				// setting it back explicitly: omitting it here would leave
				// the test-set value in place instead of restoring it.
				Config: fmt.Sprintf("resource \"growthbook_project\" \"imported\" {\n  name        = %q\n  description = %q\n}", name, description),
				Check:  resource.TestCheckResourceAttr("growthbook_project.imported", "description", description),
			},
			{
				Config: `
removed {
  from = growthbook_project.imported

  lifecycle {
    destroy = false
  }
}`,
			},
		},
	})
}

// findExistingProject looks up the id and name of the organization's
// existing project with a tiny direct HTTP call: growthbook-go does not
// expose a list-projects method, and a free-plan organization is guaranteed
// to have exactly one.
func findExistingProject(t *testing.T) (id, name, description string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, os.Getenv("GROWTHBOOK_API_URL")+"/v1/projects", nil)
	if err != nil {
		t.Fatalf("building projects list request: %s", err)
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("GROWTHBOOK_API_KEY"))
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("listing projects: %s", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("listing projects: unexpected status %d", resp.StatusCode)
	}

	var out struct {
		Projects []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"projects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding projects list: %s", err)
	}
	if len(out.Projects) == 0 {
		t.Fatal("live GrowthBook instance has no existing project to import")
	}
	return out.Projects[0].ID, out.Projects[0].Name, out.Projects[0].Description
}
