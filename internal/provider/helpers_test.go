// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func newDiags() *diag.Diagnostics { return &diag.Diagnostics{} }

func strVal(s string) types.String { return types.StringValue(s) }

func nullStr() types.String { return types.StringNull() }

func intVal(i int64) types.Int64 { return types.Int64Value(i) }

func boolVal(b bool) types.Bool { return types.BoolValue(b) }

func nullBool() types.Bool { return types.BoolNull() }

func nullList() types.List { return types.ListNull(types.StringType) }

func nullMap() types.Map { return types.MapNull(types.StringType) }
