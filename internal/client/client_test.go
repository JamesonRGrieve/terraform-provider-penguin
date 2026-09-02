// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := New(Config{Endpoint: srv.URL, Token: "papp_test", MaxRetries: -1})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNewValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "missing endpoint", cfg: Config{Token: "t"}, wantErr: true},
		{name: "missing token", cfg: Config{Endpoint: "https://p"}, wantErr: true},
		{name: "bare host gets https", cfg: Config{Endpoint: "panel.example.com", Token: "t"}},
		{name: "full url ok", cfg: Config{Endpoint: "https://panel.example.com/", Token: "t"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := New(tc.cfg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("New(%+v) err = %v, wantErr = %v", tc.cfg, err, tc.wantErr)
			}
		})
	}
}

func TestNewAppendsBasePathOnce(t *testing.T) {
	t.Parallel()
	c, err := New(Config{Endpoint: "https://panel.example.com", Token: "t"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got, want := c.baseURL.String(), "https://panel.example.com/api/application"; got != want {
		t.Fatalf("baseURL = %q, want %q", got, want)
	}
}

func TestGetItemDecodesEnvelopeAndSetsHeaders(t *testing.T) {
	t.Parallel()
	var gotAuth, gotAccept, gotPath string
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object":     "node",
			"attributes": map[string]any{"id": 7, "name": "node-a"},
		})
	}))

	item, err := c.GetItem(context.Background(), "nodes/7", nil)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if item.Object != "node" {
		t.Fatalf("object = %q, want node", item.Object)
	}
	if item.Attributes["name"] != "node-a" {
		t.Fatalf("name = %v, want node-a", item.Attributes["name"])
	}
	if gotAuth != "Bearer papp_test" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept = %q", gotAccept)
	}
	if gotPath != "/api/application/nodes/7" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestItemDecode(t *testing.T) {
	t.Parallel()
	item := &Item{Object: "node", Attributes: map[string]any{"id": float64(7), "name": "node-a", "public": true}}
	var attrs struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Public bool   `json:"public"`
	}
	if err := item.Decode(&attrs); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if attrs.ID != 7 || attrs.Name != "node-a" || !attrs.Public {
		t.Fatalf("decoded = %+v", attrs)
	}
}

func TestCreateItemPostsBody(t *testing.T) {
	t.Parallel()
	var gotMethod, gotContentType string
	var gotBody map[string]any
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object":     "user",
			"attributes": map[string]any{"id": 3, "username": "alice"},
		})
	}))

	item, err := c.CreateItem(context.Background(), "users", map[string]any{"username": "alice"})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Fatalf("content-type = %q", gotContentType)
	}
	if gotBody["username"] != "alice" {
		t.Fatalf("body = %v", gotBody)
	}
	if item == nil || item.Attributes["username"] != "alice" {
		t.Fatalf("item = %+v", item)
	}
}

func TestUpdateItemEmptyBodyReturnsNil(t *testing.T) {
	t.Parallel()
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	item, err := c.UpdateItem(context.Background(), "roles/1", map[string]any{"name": "ops"})
	if err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}
	if item != nil {
		t.Fatalf("item = %+v, want nil on 204", item)
	}
}

func TestErrorEnvelopeParsed(t *testing.T) {
	t.Parallel()
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]any{
				{"code": "ValidationException", "status": "422", "detail": "The name field is required."},
			},
		})
	}))

	_, err := c.GetItem(context.Background(), "nodes/1", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is not *APIError: %v", err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", apiErr.StatusCode)
	}
	if len(apiErr.Errors) != 1 || apiErr.Errors[0].Code != "ValidationException" {
		t.Fatalf("errors = %+v", apiErr.Errors)
	}
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	_, err := c.GetItem(context.Background(), "nodes/999", nil)
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound = false, err = %v", err)
	}
}

func TestRetryOn429ThenSuccess(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "node", "attributes": map[string]any{"id": 1}})
	}))
	t.Cleanup(srv.Close)

	c, err := New(Config{Endpoint: srv.URL, Token: "papp_test", MaxRetries: 3})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.GetItem(context.Background(), "nodes/1", nil); err != nil {
		t.Fatalf("GetItem after retry: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("calls = %d, want 2", got)
	}
}

func TestNoRetryOn422(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	t.Cleanup(srv.Close)
	c, err := New(Config{Endpoint: srv.URL, Token: "papp_test", MaxRetries: 3})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.GetItem(context.Background(), "nodes/1", nil); err == nil {
		t.Fatal("expected error")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("calls = %d, want 1 (no retry on 4xx)", got)
	}
}

func TestListItemsPaginates(t *testing.T) {
	t.Parallel()
	var gotPerPage string
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPerPage = r.URL.Query().Get("per_page")
		page := r.URL.Query().Get("page")
		id := 1
		if page == "2" {
			id = 2
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"object": "node", "attributes": map[string]any{"id": id}},
			},
			"meta": map[string]any{
				"pagination": map[string]any{"current_page": id, "total_pages": 2},
			},
		})
	}))

	q := url.Values{}
	q.Set("per_page", "1")
	items, err := c.ListItems(context.Background(), "nodes", q)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if gotPerPage != "1" {
		t.Fatalf("per_page not preserved: %q", gotPerPage)
	}
}

func TestContextCancelStopsRetry(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	c, err := New(Config{Endpoint: srv.URL, Token: "papp_test", MaxRetries: 5})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.GetItem(ctx, "nodes/1", nil); err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestAttributesJSONStable(t *testing.T) {
	t.Parallel()
	item := &Item{Attributes: map[string]any{"b": 2, "a": 1}}
	got, err := item.AttributesJSON()
	if err != nil {
		t.Fatalf("AttributesJSON: %v", err)
	}
	if want := `{"a":1,"b":2}`; got != want {
		t.Fatalf("AttributesJSON = %q, want %q (sorted keys)", got, want)
	}
}
