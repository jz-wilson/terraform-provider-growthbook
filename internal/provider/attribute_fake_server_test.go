// Copyright (c) 2026 Johnzell Wilson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// fakeAttribute mirrors the growthbook-go Attribute JSON shape closely
// enough for the httptest server below to round-trip it.
type fakeAttribute struct {
	Property      string   `json:"property"`
	Datatype      string   `json:"datatype"`
	Description   string   `json:"description,omitempty"`
	HashAttribute bool     `json:"hashAttribute,omitempty"`
	Archived      bool     `json:"archived,omitempty"`
	Enum          string   `json:"enum,omitempty"`
	Format        string   `json:"format,omitempty"`
	Projects      []string `json:"projects,omitempty"`
	Tags          []string `json:"tags,omitempty"`
}

type fakeAttributeRequest struct {
	Property      string    `json:"property,omitempty"`
	Datatype      string    `json:"datatype,omitempty"`
	Description   *string   `json:"description,omitempty"`
	Archived      *bool     `json:"archived,omitempty"`
	HashAttribute *bool     `json:"hashAttribute,omitempty"`
	Enum          *string   `json:"enum,omitempty"`
	Format        *string   `json:"format,omitempty"`
	Projects      *[]string `json:"projects,omitempty"`
	Tags          *[]string `json:"tags,omitempty"`
}

// newFakeAttributeServer starts an httptest server that implements just
// enough of the GrowthBook /v1/attributes surface (list, create, update,
// delete; there is no GET-by-id, matching the real API) for the provider's
// acceptance and data source tests. onPUT, if non-nil, is called with the
// raw body of every PUT request, letting a test assert exactly what an
// update sent.
func newFakeAttributeServer(onPUT func(body []byte)) *httptest.Server {
	mu := sync.Mutex{}
	attrs := map[string]*fakeAttribute{}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/attributes", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch r.Method {
		case http.MethodGet:
			list := make([]fakeAttribute, 0, len(attrs))
			for _, a := range attrs {
				list = append(list, *a)
			}
			writeJSON(w, http.StatusOK, map[string]any{"attributes": list})
		case http.MethodPost:
			var req fakeAttributeRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": err.Error()})
				return
			}
			if req.Property == "" {
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": "property is required"})
				return
			}
			if _, exists := attrs[req.Property]; exists {
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": "attribute already exists"})
				return
			}
			if req.Datatype == "enum" && (req.Enum == nil || *req.Enum == "") {
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": "enum is required for the enum datatype"})
				return
			}
			a := &fakeAttribute{Property: req.Property, Datatype: req.Datatype}
			applyFakeAttributeRequest(a, req)
			attrs[req.Property] = a
			writeJSON(w, http.StatusOK, map[string]any{"attribute": *a})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/attributes/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		property := strings.TrimPrefix(r.URL.Path, "/v1/attributes/")
		a, ok := attrs[property]

		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": err.Error()})
				return
			}
			if onPUT != nil {
				onPUT(body)
			}
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]any{"message": "could not find attribute " + property})
				return
			}
			var req fakeAttributeRequest
			if err := json.Unmarshal(body, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": err.Error()})
				return
			}
			if req.Property != "" {
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": "property is not an updatable field"})
				return
			}
			applyFakeAttributeRequest(a, req)
			writeJSON(w, http.StatusOK, map[string]any{"attribute": *a})
		case http.MethodDelete:
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]any{"message": "could not find attribute " + property})
				return
			}
			delete(attrs, property)
			writeJSON(w, http.StatusOK, map[string]any{"deletedProperty": property})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	return httptest.NewServer(mux)
}

func applyFakeAttributeRequest(a *fakeAttribute, req fakeAttributeRequest) {
	if req.Datatype != "" {
		a.Datatype = req.Datatype
	}
	if req.Description != nil {
		a.Description = *req.Description
	}
	if req.HashAttribute != nil {
		a.HashAttribute = *req.HashAttribute
	}
	if req.Archived != nil {
		a.Archived = *req.Archived
	}
	if req.Enum != nil {
		a.Enum = *req.Enum
	}
	if req.Format != nil {
		a.Format = *req.Format
	}
	if req.Projects != nil {
		a.Projects = *req.Projects
	}
	if req.Tags != nil {
		a.Tags = *req.Tags
	}
}
