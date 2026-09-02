// SPDX-License-Identifier: AGPL-3.0-or-later

// Package client is a framework-free transport client for the Penguin / Pelican
// Application API. It handles bearer authentication, the Fractal-style response
// envelope, list pagination, typed errors, and retries, and is deliberately
// free of any terraform-plugin-framework import so it stays unit-testable in
// isolation. The provider glue in internal/provider adapts it to the framework.
package client

import (
	"encoding/json"
	"fmt"
)

// Item is a single Application API object in its Fractal envelope form —
// {"object": "<type>", "attributes": {...}} — as returned by item endpoints and
// by each element of a list response.
type Item struct {
	Object     string         `json:"object"`
	Attributes map[string]any `json:"attributes"`
	Meta       map[string]any `json:"meta,omitempty"`
}

// AttributesJSON returns the item's attributes as canonical JSON (map keys are
// emitted in sorted order by encoding/json), suitable for a computed string
// attribute that must not churn between reads.
func (i *Item) AttributesJSON() (string, error) {
	if i.Attributes == nil {
		return "{}", nil
	}
	b, err := json.Marshal(i.Attributes)
	if err != nil {
		return "", fmt.Errorf("encoding attributes: %w", err)
	}
	return string(b), nil
}

// Decode re-marshals the item's attributes and unmarshals them into v, letting a
// typed resource map the envelope into a struct without per-key coercion.
func (i *Item) Decode(v any) error {
	b, err := json.Marshal(i.Attributes)
	if err != nil {
		return fmt.Errorf("encoding attributes: %w", err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("decoding attributes into %T: %w", v, err)
	}
	return nil
}

// list is the collection envelope {"object":"list","data":[...],"meta":{...}}.
type list struct {
	Object string   `json:"object"`
	Data   []Item   `json:"data"`
	Meta   listMeta `json:"meta"`
}

type listMeta struct {
	Pagination Pagination `json:"pagination"`
}

// Pagination mirrors meta.pagination on a list response.
type Pagination struct {
	Total       int               `json:"total"`
	Count       int               `json:"count"`
	PerPage     int               `json:"per_page"`
	CurrentPage int               `json:"current_page"`
	TotalPages  int               `json:"total_pages"`
	Links       map[string]string `json:"links"`
}

func decodeItem(b []byte) (*Item, error) {
	var it Item
	if err := json.Unmarshal(b, &it); err != nil {
		return nil, fmt.Errorf("decoding item envelope: %w", err)
	}
	return &it, nil
}

func decodeList(b []byte) (*list, error) {
	var l list
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("decoding list envelope: %w", err)
	}
	return &l, nil
}
