// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// sdkConnectionKeyPrefixRegexp matches the "sdk-" prefix GrowthBook uses for
// generated SDK connection client keys.
func sdkConnectionKeyPrefixRegexp() *regexp.Regexp {
	return regexp.MustCompile(`^sdk-`)
}

// TestAccSDKConnectionResource_live exercises full CRUD against a real
// GrowthBook organization. It only runs when GROWTHBOOK_LIVE=1, in addition
// to the usual TF_ACC=1 and GROWTHBOOK_API_KEY / GROWTHBOOK_API_URL that
// testAccPreCheck requires. SDK connections are creatable on the GrowthBook
// free plan, so this does not require a paid organization.
func TestAccSDKConnectionResource_live(t *testing.T) {
	if os.Getenv("GROWTHBOOK_LIVE") != "1" {
		t.Skip("set GROWTHBOOK_LIVE=1 to run live GrowthBook acceptance tests")
	}
	testAccPreCheck(t)

	name := fmt.Sprintf("tf-acc-sdk-conn-%d", time.Now().UnixNano())
	updatedName := name + "-updated"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "growthbook_sdk_connection" "test" {
  name        = %q
  language    = "javascript"
  environment = "production"
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_sdk_connection.test", "name", name),
					resource.TestCheckResourceAttr("growthbook_sdk_connection.test", "language", "javascript"),
					resource.TestCheckResourceAttrSet("growthbook_sdk_connection.test", "id"),
					resource.TestMatchResourceAttr("growthbook_sdk_connection.test", "key", sdkConnectionKeyPrefixRegexp()),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "growthbook_sdk_connection" "test" {
  name            = %q
  language        = "javascript"
  environment     = "production"
  encrypt_payload = true
}
`, updatedName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("growthbook_sdk_connection.test", "name", updatedName),
					resource.TestCheckResourceAttr("growthbook_sdk_connection.test", "encrypt_payload", "true"),
				),
			},
			{
				ResourceName:            "growthbook_sdk_connection.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{},
			},
		},
	})
}
