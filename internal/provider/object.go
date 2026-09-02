// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

// The functions below factor the identical create/read/update/delete plumbing
// shared by every typed resource: one client call, uniform error diagnostics,
// and self-healing on absence. Each typed resource supplies only its schema, its
// request-body builder, and its item→model mapping.

func createObject(ctx context.Context, c *client.Client, collection string, body any, diags *diag.Diagnostics) *client.Item {
	if c == nil {
		diags.AddError("Provider not configured", "The Penguin API client is unset. This is a bug in the provider.")
		return nil
	}
	item, err := c.CreateItem(ctx, collection, body)
	if err != nil {
		diags.AddError("Unable to create "+collection, err.Error())
		return nil
	}
	if item == nil {
		diags.AddError("Empty create response", "POST /api/application/"+collection+" returned no object envelope.")
		return nil
	}
	return item
}

// readObject GETs an item path. gone is true when the object is absent (HTTP
// 404) so the caller can self-heal by dropping the resource from state.
func readObject(ctx context.Context, c *client.Client, itemPath string, diags *diag.Diagnostics) (item *client.Item, gone bool) {
	if c == nil {
		diags.AddError("Provider not configured", "The Penguin API client is unset. This is a bug in the provider.")
		return nil, false
	}
	item, err := c.GetItem(ctx, itemPath, nil)
	if err != nil {
		if client.IsNotFound(err) {
			return nil, true
		}
		diags.AddError("Unable to read "+itemPath, err.Error())
		return nil, false
	}
	return item, false
}

func updateObject(ctx context.Context, c *client.Client, itemPath string, body any, diags *diag.Diagnostics) *client.Item {
	if c == nil {
		diags.AddError("Provider not configured", "The Penguin API client is unset. This is a bug in the provider.")
		return nil
	}
	item, err := c.UpdateItem(ctx, itemPath, body)
	if err != nil {
		diags.AddError("Unable to update "+itemPath, err.Error())
		return nil
	}
	if item == nil {
		item, err = c.GetItem(ctx, itemPath, nil)
		if err != nil {
			diags.AddError("Unable to read "+itemPath+" after update", err.Error())
			return nil
		}
	}
	return item
}

func deleteObject(ctx context.Context, c *client.Client, itemPath string, diags *diag.Diagnostics) {
	if c == nil {
		diags.AddError("Provider not configured", "The Penguin API client is unset. This is a bug in the provider.")
		return
	}
	if err := c.DeleteItem(ctx, itemPath); err != nil && !client.IsNotFound(err) {
		diags.AddError("Unable to delete "+itemPath, err.Error())
	}
}

// importInt64ID imports a resource whose identity is a single numeric id.
func importInt64ID(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(strings.TrimSpace(req.ID), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "Expected a numeric id, got "+strconv.Quote(req.ID)+".")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
