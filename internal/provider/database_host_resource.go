// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

const databaseHostsCollection = "database-hosts"

var (
	_ resource.Resource                = (*databaseHostResource)(nil)
	_ resource.ResourceWithConfigure   = (*databaseHostResource)(nil)
	_ resource.ResourceWithImportState = (*databaseHostResource)(nil)
)

// NewDatabaseHostResource manages a database host — a MySQL/MariaDB server the
// panel provisions per-server databases on.
func NewDatabaseHostResource() resource.Resource { return &databaseHostResource{} }

type databaseHostResource struct {
	client *client.Client
}

type databaseHostModel struct {
	ID        types.Int64  `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Host      types.String `tfsdk:"host"`
	Port      types.Int64  `tfsdk:"port"`
	Username  types.String `tfsdk:"username"`
	Password  types.String `tfsdk:"password"`
	NodeIDs   types.List   `tfsdk:"node_ids"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

type databaseHostAttrs struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Host      string  `json:"host"`
	Port      int64   `json:"port"`
	Username  string  `json:"username"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

func (r *databaseHostResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_host"
}

func (r *databaseHostResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A database host — a MySQL/MariaDB server the panel provisions per-server databases on.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric database host id.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name.",
			},
			"host": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Hostname or IP of the database server.",
			},
			"port": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Database server port (1–65535).",
			},
			"username": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Privileged username used to create databases and users (max 32 characters).",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Password for the privileged user. Never returned by the API; kept in state to detect changes.",
			},
			"node_ids": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.Int64Type,
				MarkdownDescription: "Node ids allowed to use this database host. Accepted on write; the linked nodes are exposed via the `nodes` include, not this attribute.",
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

func (r *databaseHostResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *databaseHostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan databaseHostModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item := createObject(ctx, r.client, databaseHostsCollection, buildDatabaseHostBody(ctx, plan, &resp.Diagnostics), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	applyDatabaseHost(item, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseHostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state databaseHostModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, gone := readObject(ctx, r.client, databaseHostItemPath(state.ID), &resp.Diagnostics)
	if gone {
		resp.State.RemoveResource(ctx)
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	applyDatabaseHost(item, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *databaseHostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state databaseHostModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	item := updateObject(ctx, r.client, databaseHostItemPath(plan.ID), buildDatabaseHostBody(ctx, plan, &resp.Diagnostics), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	applyDatabaseHost(item, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseHostResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state databaseHostModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteObject(ctx, r.client, databaseHostItemPath(state.ID), &resp.Diagnostics)
}

func (r *databaseHostResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importInt64ID(ctx, req, resp)
}

func databaseHostItemPath(id types.Int64) string {
	return joinPath(databaseHostsCollection, strconv.FormatInt(id.ValueInt64(), 10))
}

func buildDatabaseHostBody(ctx context.Context, m databaseHostModel, diags *diag.Diagnostics) map[string]any {
	body := map[string]any{}
	putString(body, "name", m.Name)
	putString(body, "host", m.Host)
	putInt64(body, "port", m.Port)
	putString(body, "username", m.Username)
	putString(body, "password", m.Password)
	putInt64List(ctx, body, "node_ids", m.NodeIDs, diags)
	return body
}

// applyDatabaseHost maps the read envelope onto the model, leaving password and
// node_ids untouched (the API does not return them).
func applyDatabaseHost(item *client.Item, m *databaseHostModel, diags *diag.Diagnostics) {
	var a databaseHostAttrs
	if err := item.Decode(&a); err != nil {
		diags.AddError("Unable to decode database host", err.Error())
		return
	}
	m.ID = types.Int64Value(a.ID)
	m.Name = types.StringValue(a.Name)
	m.Host = types.StringValue(a.Host)
	m.Port = types.Int64Value(a.Port)
	m.Username = types.StringValue(a.Username)
	m.CreatedAt = stringPtr(a.CreatedAt)
	m.UpdatedAt = stringPtr(a.UpdatedAt)
}
