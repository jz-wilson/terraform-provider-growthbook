// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccSavedGroupDataSource_fake looks up a saved group by id against the
// httptest-backed fake GrowthBook API.
func TestAccSavedGroupDataSource_fake(t *testing.T) {
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
  name          = "tf-acc-saved-group-ds"
  type          = "list"
  attribute_key = "userId"
  values        = ["user-1"]
}

data "growthbook_saved_group" "test" {
  id = growthbook_saved_group.test.id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.growthbook_saved_group.test", "id", "growthbook_saved_group.test", "id"),
					resource.TestCheckResourceAttr("data.growthbook_saved_group.test", "name", "tf-acc-saved-group-ds"),
					resource.TestCheckResourceAttr("data.growthbook_saved_group.test", "type", "list"),
					resource.TestCheckResourceAttr("data.growthbook_saved_group.test", "values.#", "1"),
				),
			},
		},
	})
}
