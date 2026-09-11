// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during
// acceptance testing. The factory function is called for each Terraform CLI
// command to create a provider server that the CLI can connect to.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"growthbook": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck verifies the live GrowthBook the acceptance tests need is
// configured. Start one with e2e/docker-compose.yml and mint a key with
// e2e/bootstrap.sh; see e2e/README.md.
func testAccPreCheck(t *testing.T) {
	t.Helper()
	for _, v := range []string{"GROWTHBOOK_API_KEY", "GROWTHBOOK_API_URL"} {
		if os.Getenv(v) == "" {
			t.Fatalf("%s must be set for acceptance tests", v)
		}
	}
}
