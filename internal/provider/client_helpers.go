// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"

	growthbook "github.com/jz-wilson/growthbook-go"
)

// clientFromProviderData converts the value the provider stored in
// ConfigureResponse.ResourceData / DataSourceData back into a client. It
// returns nil (and no diagnostic) when the provider has not been configured
// yet, which the framework allows during validation.
func clientFromProviderData(providerData any, diags *diag.Diagnostics) *growthbook.Client {
	if providerData == nil {
		return nil
	}
	client, ok := providerData.(*growthbook.Client)
	if !ok {
		diags.AddError(
			"Unexpected provider data type",
			"Expected *growthbook.Client; this is a bug in the provider, please report it.",
		)
		return nil
	}
	return client
}
