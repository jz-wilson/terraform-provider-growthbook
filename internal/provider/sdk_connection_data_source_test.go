// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccSDKConnectionDataSource_fake exercises both the singular
// growthbook_sdk_connection and the plural growthbook_sdk_connections data
// sources against the httptest-backed fake GrowthBook API.
func TestAccSDKConnectionDataSource_fake(t *testing.T) {
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
  name        = "tf-acc-sdk-conn-ds"
  language    = "javascript"
  environment = "production"
}

data "growthbook_sdk_connection" "test" {
  id = growthbook_sdk_connection.test.id
}

data "growthbook_sdk_connections" "all" {
  depends_on = [growthbook_sdk_connection.test]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.growthbook_sdk_connection.test", "id", "growthbook_sdk_connection.test", "id"),
					resource.TestCheckResourceAttr("data.growthbook_sdk_connection.test", "name", "tf-acc-sdk-conn-ds"),
					resource.TestCheckResourceAttrSet("data.growthbook_sdk_connection.test", "key"),
					resource.TestCheckResourceAttr("data.growthbook_sdk_connections.all", "sdk_connections.#", "1"),
					resource.TestCheckResourceAttr("data.growthbook_sdk_connections.all", "sdk_connections.0.name", "tf-acc-sdk-conn-ds"),
				),
			},
		},
	})
}
