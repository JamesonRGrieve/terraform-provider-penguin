// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// stringSliceFromList converts a Terraform list of strings to a Go slice.
func stringSliceFromList(ctx context.Context, l types.List, diags *diag.Diagnostics) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	out := make([]string, 0, len(l.Elements()))
	diags.Append(l.ElementsAs(ctx, &out, false)...)
	return out
}

// listFromStringSlice converts a Go slice to a Terraform list of strings. A nil
// slice becomes an empty (non-null) list so it matches an API-returned [].
func listFromStringSlice(ctx context.Context, s []string, diags *diag.Diagnostics) types.List {
	if s == nil {
		s = []string{}
	}
	l, d := types.ListValueFrom(ctx, types.StringType, s)
	diags.Append(d...)
	return l
}

// int64SliceFromList converts a Terraform list of int64 to a Go slice.
func int64SliceFromList(ctx context.Context, l types.List, diags *diag.Diagnostics) []int64 {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	out := make([]int64, 0, len(l.Elements()))
	diags.Append(l.ElementsAs(ctx, &out, false)...)
	return out
}

// listFromInt64Slice converts a Go slice to a Terraform list of int64. A nil
// slice becomes an empty (non-null) list.
func listFromInt64Slice(ctx context.Context, s []int64, diags *diag.Diagnostics) types.List {
	if s == nil {
		s = []int64{}
	}
	l, d := types.ListValueFrom(ctx, types.Int64Type, s)
	diags.Append(d...)
	return l
}

// stringPtr returns a nullable Terraform string from a *string.
func stringPtr(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}

// int64Ptr returns a nullable Terraform int64 from a *int64.
func int64Ptr(v *int64) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*v)
}

// The put* helpers append a value to a request-body map only when it is known
// and non-null, so an omitted optional field is left out of the request rather
// than sent as null.

func putString(body map[string]any, key string, v types.String) {
	if !v.IsNull() && !v.IsUnknown() {
		body[key] = v.ValueString()
	}
}

func putInt64(body map[string]any, key string, v types.Int64) {
	if !v.IsNull() && !v.IsUnknown() {
		body[key] = v.ValueInt64()
	}
}

func putBool(body map[string]any, key string, v types.Bool) {
	if !v.IsNull() && !v.IsUnknown() {
		body[key] = v.ValueBool()
	}
}

func putStringList(ctx context.Context, body map[string]any, key string, v types.List, diags *diag.Diagnostics) {
	if v.IsNull() || v.IsUnknown() {
		return
	}
	body[key] = stringSliceFromList(ctx, v, diags)
}

func putInt64List(ctx context.Context, body map[string]any, key string, v types.List, diags *diag.Diagnostics) {
	if v.IsNull() || v.IsUnknown() {
		return
	}
	body[key] = int64SliceFromList(ctx, v, diags)
}

// envMap converts a Terraform string map to a Go map for a request body,
// returning an empty (non-nil) map when unset — the environment field is
// required-present on server create/startup.
func envMap(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]string {
	out := map[string]string{}
	if m.IsNull() || m.IsUnknown() {
		return out
	}
	diags.Append(m.ElementsAs(ctx, &out, false)...)
	return out
}

// mapFromStringMap builds a Terraform string map from a Go map.
func mapFromStringMap(ctx context.Context, m map[string]string, diags *diag.Diagnostics) types.Map {
	if m == nil {
		m = map[string]string{}
	}
	v, d := types.MapValueFrom(ctx, types.StringType, m)
	diags.Append(d...)
	return v
}

// stringifyAnyMap coerces a decoded JSON object's scalar values to strings, so a
// mixed-type environment map (strings, numbers, bools) becomes map[string]string.
func stringifyAnyMap(m map[string]any) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		switch t := v.(type) {
		case string:
			out[k] = t
		case bool:
			out[k] = strconv.FormatBool(t)
		case nil:
			out[k] = ""
		default:
			out[k] = numberToString(v)
		}
	}
	return out
}
