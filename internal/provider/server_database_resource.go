// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

var (
	_ resource.Resource                = (*serverDatabaseResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverDatabaseResource)(nil)
	_ resource.ResourceWithImportState = (*serverDatabaseResource)(nil)
)

// NewServerDatabaseResource manages a database belonging to a server. Its fields
// are immutable on the panel, so any change forces replacement.
func NewServerDatabaseResource() resource.Resource { return &serverDatabaseResource{} }

type serverDatabaseResource struct {
	client *client.Client
}

type serverDatabaseModel struct {
	ID             types.Int64  `tfsdk:"id"`
	ServerID       types.Int64  `tfsdk:"server_id"`
	HostID         types.Int64  `tfsdk:"host_id"`
	Database       types.String `tfsdk:"database"`
	Remote         types.String `tfsdk:"remote"`
	Username       types.String `tfsdk:"username"`
	Password       types.String `tfsdk:"password"`
	MaxConnections types.Int64  `tfsdk:"max_connections"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

type serverDatabaseAttrs struct {
	ID             int64   `json:"id"`
	Host           int64   `json:"host"`
	Database       string  `json:"database"`
	Username       string  `json:"username"`
	Remote         string  `json:"remote"`
	MaxConnections *int64  `json:"max_connections"`
	CreatedAt      *string `json:"created_at"`
	UpdatedAt      *string `json:"updated_at"`
}

func (r *serverDatabaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_database"
}

func (r *serverDatabaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replaceInt := []planmodifier.Int64{int64planmodifier.RequiresReplace()}
	replaceStr := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "A database belonging to a server, provisioned on a database host. Database fields cannot be edited in place, so any change replaces the resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric database id.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"server_id": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Id of the server the database belongs to.",
				PlanModifiers:       replaceInt,
			},
			"host_id": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Id of the database host to provision on.",
				PlanModifiers:       replaceInt,
			},
			"database": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Database name (alphanumeric/underscore/dash, 1–48 characters).",
				PlanModifiers:       replaceStr,
			},
			"remote": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Remote host pattern allowed to connect, e.g. `%` or `10.0.%`.",
				PlanModifiers:       replaceStr,
			},
			"username": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Generated database username.",
			},
			"password": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Generated database password (read via `?include=password`).",
			},
			"max_connections": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Maximum concurrent connections (0 = unlimited).",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last-update timestamp.",
			},
		},
	}
}

func (r *serverDatabaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *serverDatabaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverDatabaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	serverID := plan.ServerID.ValueInt64()
	body := map[string]any{
		"database": plan.Database.ValueString(),
		"remote":   plan.Remote.ValueString(),
		"host":     plan.HostID.ValueInt64(),
	}
	created := createObject(ctx, r.client, serverDatabaseCollection(serverID), body, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	id, ok := itemID(created)
	if !ok {
		resp.Diagnostics.AddError("Missing database id", "The created database has no id attribute.")
		return
	}
	idNum, _ := strconv.ParseInt(id, 10, 64)
	// Re-read with the password include so the generated credentials are captured.
	item, gone := readServerDatabase(ctx, r.client, serverID, idNum, &resp.Diagnostics)
	if resp.Diagnostics.HasError() || gone {
		if gone {
			resp.Diagnostics.AddError("Database vanished after create", "The database could not be read back immediately after creation.")
		}
		return
	}
	applyServerDatabase(item, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverDatabaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverDatabaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, gone := readServerDatabase(ctx, r.client, state.ServerID.ValueInt64(), state.ID.ValueInt64(), &resp.Diagnostics)
	if gone {
		resp.State.RemoveResource(ctx)
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	applyServerDatabase(item, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is unreachable — every writable attribute forces replacement.
func (r *serverDatabaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverDatabaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverDatabaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serverDatabaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := joinPath(serverDatabaseCollection(state.ServerID.ValueInt64()), strconv.FormatInt(state.ID.ValueInt64(), 10))
	deleteObject(ctx, r.client, itemPath, &resp.Diagnostics)
}

func (r *serverDatabaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serverID, id, ok := splitCompositeID(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `<server_id>/<database_id>`, e.g. `3/7`.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("server_id"), serverID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func serverDatabaseCollection(serverID int64) string {
	return joinPath(serversCollection, strconv.FormatInt(serverID, 10), "databases")
}

func readServerDatabase(ctx context.Context, c *client.Client, serverID, id int64, diags *diag.Diagnostics) (*client.Item, bool) {
	if c == nil {
		diags.AddError("Provider not configured", "The Penguin API client is unset. This is a bug in the provider.")
		return nil, false
	}
	itemPath := joinPath(serverDatabaseCollection(serverID), strconv.FormatInt(id, 10))
	q := url.Values{}
	q.Set("include", "password")
	item, err := c.GetItem(ctx, itemPath, q)
	if err != nil {
		if client.IsNotFound(err) {
			return nil, true
		}
		diags.AddError("Unable to read "+itemPath, err.Error())
		return nil, false
	}
	return item, false
}

func applyServerDatabase(item *client.Item, m *serverDatabaseModel, diags *diag.Diagnostics) {
	var a serverDatabaseAttrs
	if err := item.Decode(&a); err != nil {
		diags.AddError("Unable to decode server database", err.Error())
		return
	}
	m.ID = types.Int64Value(a.ID)
	if a.Host != 0 {
		m.HostID = types.Int64Value(a.Host)
	}
	m.Database = types.StringValue(a.Database)
	m.Username = types.StringValue(a.Username)
	m.Remote = types.StringValue(a.Remote)
	m.MaxConnections = int64Ptr(a.MaxConnections)
	m.Password = stringPtr(includedString(item, "password", "password"))
	m.CreatedAt = stringPtr(a.CreatedAt)
	m.UpdatedAt = stringPtr(a.UpdatedAt)
}
