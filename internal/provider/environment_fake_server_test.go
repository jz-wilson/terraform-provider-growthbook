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

// fakeEnvironment mirrors the growthbook-go Environment JSON shape closely
// enough for the httptest server below to round-trip it.
type fakeEnvironment struct {
	ID           string   `json:"id"`
	Description  string   `json:"description"`
	ToggleOnList bool     `json:"toggleOnList"`
	DefaultState bool     `json:"defaultState"`
	Projects     []string `json:"projects"`
	Parent       string   `json:"parent,omitempty"`
}

type fakeEnvironmentRequest struct {
	ID           string   `json:"id,omitempty"`
	Description  *string  `json:"description,omitempty"`
	ToggleOnList *bool    `json:"toggleOnList,omitempty"`
	DefaultState *bool    `json:"defaultState,omitempty"`
	Projects     []string `json:"projects,omitempty"`
	Parent       *string  `json:"parent,omitempty"`
}

// newFakeEnvironmentServer starts an httptest server that implements just
// enough of the GrowthBook /v1/environments surface (list, create, update,
// delete) for the provider's acceptance and data source tests. It is
// pre-seeded with a "production" environment, matching every real
// GrowthBook organization, so data source tests have something to find.
// onPUT, if non-nil, is called with the raw body of every PUT request,
// letting a test assert exactly what an update sent.
func newFakeEnvironmentServer(onPUT func(body []byte)) *httptest.Server {
	mu := sync.Mutex{}
	envs := map[string]*fakeEnvironment{
		"production": {ID: "production", Description: "Production", ToggleOnList: true, DefaultState: false},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/environments", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch r.Method {
		case http.MethodGet:
			list := make([]fakeEnvironment, 0, len(envs))
			for _, e := range envs {
				list = append(list, *e)
			}
			writeJSON(w, http.StatusOK, map[string]any{"environments": list})
		case http.MethodPost:
			var req fakeEnvironmentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": err.Error()})
				return
			}
			if req.ID == "" {
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": "id is required"})
				return
			}
			if _, exists := envs[req.ID]; exists {
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": "environment already exists"})
				return
			}
			e := &fakeEnvironment{ID: req.ID}
			applyFakeEnvironmentRequest(e, req)
			envs[req.ID] = e
			writeJSON(w, http.StatusOK, map[string]any{"environment": *e})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/environments/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		id := strings.TrimPrefix(r.URL.Path, "/v1/environments/")
		e, ok := envs[id]

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
				writeJSON(w, http.StatusNotFound, map[string]any{"message": "could not find environment " + id})
				return
			}
			var req fakeEnvironmentRequest
			if err := json.Unmarshal(body, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": err.Error()})
				return
			}
			applyFakeEnvironmentRequest(e, req)
			writeJSON(w, http.StatusOK, map[string]any{"environment": *e})
		case http.MethodDelete:
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]any{"message": "could not find environment " + id})
				return
			}
			delete(envs, id)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	return httptest.NewServer(mux)
}

func applyFakeEnvironmentRequest(e *fakeEnvironment, req fakeEnvironmentRequest) {
	if req.Description != nil {
		e.Description = *req.Description
	}
	if req.ToggleOnList != nil {
		e.ToggleOnList = *req.ToggleOnList
	}
	if req.DefaultState != nil {
		e.DefaultState = *req.DefaultState
	}
	if req.Projects != nil {
		e.Projects = req.Projects
	}
	if req.Parent != nil {
		e.Parent = *req.Parent
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
