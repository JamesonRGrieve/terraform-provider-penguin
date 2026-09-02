// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

// stringOrEnv resolves a provider config string, falling back to an env var.
func stringOrEnv(v types.String, env string) string {
	if !v.IsNull() && !v.IsUnknown() {
		if s := strings.TrimSpace(v.ValueString()); s != "" {
			return s
		}
	}
	return strings.TrimSpace(os.Getenv(env))
}

// boolOrEnv resolves a provider config bool, falling back to an env var.
func boolOrEnv(v types.Bool, env string) bool {
	if !v.IsNull() && !v.IsUnknown() {
		return v.ValueBool()
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv(env))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// clientFrom pulls the configured API client out of provider data. A nil return
// with no diagnostic means the provider is not configured yet (the framework
// calls Configure again before CRUD).
func clientFrom(providerData any, diags *diag.Diagnostics) *client.Client {
	if providerData == nil {
		return nil
	}
	c, ok := providerData.(*client.Client)
	if !ok {
		diags.AddError(
			"Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got %T. This is a bug in the provider.", providerData),
		)
		return nil
	}
	return c
}

// itemID extracts an object's `id` attribute as a string.
func itemID(item *client.Item) (string, bool) {
	if item == nil || item.Attributes == nil {
		return "", false
	}
	raw, ok := item.Attributes["id"]
	if !ok {
		return "", false
	}
	return numberToString(raw), true
}

func numberToString(v any) string {
	switch n := v.(type) {
	case float64:
		return strconv.FormatInt(int64(n), 10)
	case float32:
		return strconv.FormatInt(int64(n), 10)
	case int:
		return strconv.Itoa(n)
	case int64:
		return strconv.FormatInt(n, 10)
	case json.Number:
		return n.String()
	case string:
		return n
	default:
		return fmt.Sprintf("%v", v)
	}
}

// joinPath joins API path segments, trimming stray slashes.
func joinPath(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, "/")
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	return strings.Join(cleaned, "/")
}

// canonicalJSON re-encodes arbitrary JSON with sorted object keys so a computed
// string attribute does not churn on cosmetic formatting differences.
func canonicalJSON(b []byte) (string, error) {
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 {
		return "", nil
	}
	var v any
	if err := json.Unmarshal(trimmed, &v); err != nil {
		return "", fmt.Errorf("parsing JSON: %w", err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encoding JSON: %w", err)
	}
	return string(out), nil
}

// bodyArg returns the request body for a JSON-string attribute, or nil when unset.
func bodyArg(v types.String) any {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	if strings.TrimSpace(v.ValueString()) == "" {
		return nil
	}
	return v.ValueString()
}

// includedString pulls attributes.relationships.<include>.attributes.<field> out
// of an item envelope (e.g. a database password requested via ?include=password).
func includedString(item *client.Item, include, field string) *string {
	rels, ok := item.Attributes["relationships"].(map[string]any)
	if !ok {
		return nil
	}
	rel, ok := rels[include].(map[string]any)
	if !ok {
		return nil
	}
	attrs, ok := rel["attributes"].(map[string]any)
	if !ok {
		return nil
	}
	v, ok := attrs[field].(string)
	if !ok {
		return nil
	}
	return &v
}

// mapToValues converts a Terraform string map into URL query values.
func mapToValues(ctx context.Context, m types.Map, diags *diag.Diagnostics) url.Values {
	out := url.Values{}
	if m.IsNull() || m.IsUnknown() {
		return out
	}
	elems := map[string]types.String{}
	diags.Append(m.ElementsAs(ctx, &elems, false)...)
	for k, v := range elems {
		if !v.IsNull() && !v.IsUnknown() {
			out.Set(k, v.ValueString())
		}
	}
	return out
}
