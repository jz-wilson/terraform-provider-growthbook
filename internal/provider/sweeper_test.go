// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	growthbook "github.com/jz-wilson/growthbook-go"
)

// acctestNamePrefix is the required prefix for every object a live
// acceptance test creates (see acctest.RandomWithPrefix("tf-acc") at each
// call site). Sweepers use it as an allow-list: it is the only thing that
// makes "delete everything the test run touched" safe to run unattended in
// CI without risk of eating hand-created organization data.
const acctestNamePrefix = "tf-acc-"

func TestMain(m *testing.M) {
	resource.TestMain(m)
}

func init() {
	resource.AddTestSweepers("growthbook_feature", &resource.Sweeper{
		Name: "growthbook_feature",
		F:    sweepFeatures,
	})
	resource.AddTestSweepers("growthbook_sdk_connection", &resource.Sweeper{
		Name: "growthbook_sdk_connection",
		F:    sweepSDKConnections,
	})
	resource.AddTestSweepers("growthbook_attribute", &resource.Sweeper{
		Name: "growthbook_attribute",
		F:    sweepAttributes,
	})
	resource.AddTestSweepers("growthbook_saved_group", &resource.Sweeper{
		Name: "growthbook_saved_group",
		F:    sweepSavedGroups,
	})
}

// sweeperClient builds a client from the same GROWTHBOOK_API_KEY /
// GROWTHBOOK_API_URL environment variables the provider itself reads
// (see provider.go Configure), so the sweeper always targets the exact
// instance the acceptance tests just ran against.
func sweeperClient() (*growthbook.Client, error) {
	apiKey := os.Getenv("GROWTHBOOK_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GROWTHBOOK_API_KEY must be set to run sweepers")
	}
	apiURL := os.Getenv("GROWTHBOOK_API_URL")
	return growthbook.New(growthbook.Credentials{APIKey: apiKey, APIURL: apiURL})
}

// sweepFeatures deletes every feature whose key starts with tf-acc-,
// archiving first when GrowthBook demands it (see
// growthbook.IsArchiveRequired, and the identical dance in
// featureResource.Delete). Only tf-acc-* keys are ever touched.
func sweepFeatures(_ string) error {
	client, err := sweeperClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	keys, err := listFeatureKeys(ctx)
	if err != nil {
		return fmt.Errorf("listing features to sweep: %w", err)
	}

	var errs []error
	for _, key := range keys {
		if !strings.HasPrefix(key, acctestNamePrefix) {
			continue
		}
		if delErr := client.DeleteFeature(ctx, key); delErr != nil {
			if growthbook.IsArchiveRequired(delErr) {
				archived := true
				if _, updErr := client.UpdateFeature(ctx, key, growthbook.FeatureRequest{Archived: &archived}); updErr != nil {
					errs = append(errs, fmt.Errorf("archiving feature %q before delete: %w", key, updErr))
					continue
				}
				delErr = client.DeleteFeature(ctx, key)
			}
			if delErr != nil && !growthbook.IsNotFound(delErr) {
				errs = append(errs, fmt.Errorf("deleting feature %q: %w", key, delErr))
			}
		}
	}
	return joinErrors(errs)
}

// sweepSDKConnections deletes every SDK connection whose name starts with
// tf-acc-.
func sweepSDKConnections(_ string) error {
	client, err := sweeperClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	conns, err := client.ListSDKConnections(ctx)
	if err != nil {
		return fmt.Errorf("listing SDK connections to sweep: %w", err)
	}

	var errs []error
	for _, conn := range conns {
		if !strings.HasPrefix(conn.Name, acctestNamePrefix) {
			continue
		}
		if delErr := client.DeleteSDKConnection(ctx, conn.ID); delErr != nil && !growthbook.IsNotFound(delErr) {
			errs = append(errs, fmt.Errorf("deleting SDK connection %q (%s): %w", conn.Name, conn.ID, delErr))
		}
	}
	return joinErrors(errs)
}

// sweepAttributes deletes every attribute whose property starts with
// tf-acc-.
func sweepAttributes(_ string) error {
	client, err := sweeperClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	attrs, err := client.ListAttributes(ctx)
	if err != nil {
		return fmt.Errorf("listing attributes to sweep: %w", err)
	}

	var errs []error
	for _, attr := range attrs {
		if !strings.HasPrefix(attr.Property, acctestNamePrefix) {
			continue
		}
		if delErr := client.DeleteAttribute(ctx, attr.Property); delErr != nil && !growthbook.IsNotFound(delErr) {
			errs = append(errs, fmt.Errorf("deleting attribute %q: %w", attr.Property, delErr))
		}
	}
	return joinErrors(errs)
}

// sweepSavedGroups deletes every saved group whose name starts with tf-acc-.
func sweepSavedGroups(_ string) error {
	client, err := sweeperClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	groups, err := client.ListSavedGroups(ctx)
	if err != nil {
		return fmt.Errorf("listing saved groups to sweep: %w", err)
	}

	var errs []error
	for _, g := range groups {
		if !strings.HasPrefix(g.Name, acctestNamePrefix) {
			continue
		}
		if delErr := client.DeleteSavedGroup(ctx, g.ID); delErr != nil && !growthbook.IsNotFound(delErr) {
			errs = append(errs, fmt.Errorf("deleting saved group %q (%s): %w", g.Name, g.ID, delErr))
		}
	}
	return joinErrors(errs)
}

// featureListEnvelope mirrors the subset of the paginated GET /v2/features
// response the sweeper needs. growthbook-go (v0.1.0) does not expose a
// ListFeatures method — only Get/Create/Update/Delete by id/key — and this
// task is scoped to not change growthbook-go, so the sweeper makes its own
// minimal, read-only GET request against the same API root and key the
// provider itself uses. If a ListFeatures method is added to growthbook-go
// later, this can be replaced with a call to it.
type featureListEnvelope struct {
	Features []struct {
		ID string `json:"id"`
	} `json:"features"`
	NextOffset *int `json:"nextOffset"`
	HasMore    bool `json:"hasMore"`
}

// listFeatureKeys pages through GET /v2/features and returns every feature
// key (id) in the organization. It is read-only.
func listFeatureKeys(ctx context.Context) ([]string, error) {
	apiKey := os.Getenv("GROWTHBOOK_API_KEY")
	apiURL := strings.TrimRight(os.Getenv("GROWTHBOOK_API_URL"), "/")
	if apiURL == "" {
		apiURL = strings.TrimSuffix(growthbook.DefaultAPIURL, "/")
	}

	var keys []string
	offset := 0
	httpClient := &http.Client{}
	for i := 0; i < 100; i++ { // hard cap: never loop forever on an unexpected response shape
		url := fmt.Sprintf("%s/v2/features?limit=100&offset=%d", apiURL, offset)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Accept", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close() //nolint:errcheck
			return nil, fmt.Errorf("GET /v2/features: unexpected status %d", resp.StatusCode)
		}

		var out featureListEnvelope
		decErr := json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close() //nolint:errcheck
		if decErr != nil {
			return nil, fmt.Errorf("decoding features list: %w", decErr)
		}

		for _, f := range out.Features {
			keys = append(keys, f.ID)
		}

		if !out.HasMore || out.NextOffset == nil || len(out.Features) == 0 {
			break
		}
		offset = *out.NextOffset
	}
	return keys, nil
}

func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}
	return fmt.Errorf("%d error(s) sweeping: %s", len(errs), strings.Join(msgs, "; "))
}
