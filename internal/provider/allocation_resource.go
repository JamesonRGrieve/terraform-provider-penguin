// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"net/url"
	"strconv"
	"strings"

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
	_ resource.Resource                = (*allocationResource)(nil)
	_ resource.ResourceWithConfigure   = (*allocationResource)(nil)
	_ resource.ResourceWithImportState = (*allocationResource)(nil)
)

// NewAllocationResource manages a single IP:port network allocation on a node.
// Allocations are immutable on the panel, so any change forces replacement.
func NewAllocationResource() resource.Resource { return &allocationResource{} }

type allocationResource struct {
	client *client.Client
}

type allocationModel struct {
	ID       types.Int64  `tfsdk:"id"`
	NodeID   types.Int64  `tfsdk:"node_id"`
	IP       types.String `tfsdk:"ip"`
	Port     types.Int64  `tfsdk:"port"`
	Alias    types.String `tfsdk:"alias"`
	Notes    types.String `tfsdk:"notes"`
	Assigned types.Bool   `tfsdk:"assigned"`
}

type allocationAttrs struct {
	ID       int64   `json:"id"`
	IP       string  `json:"ip"`
	Alias    *string `json:"alias"`
	Port     int64   `json:"port"`
	Notes    *string `json:"notes"`
	Assigned bool    `json:"assigned"`
}

func (r *allocationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allocation"
}

func (r *allocationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A single IP:port allocation on a node. Allocations cannot be edited in place, so any attribute change replaces the resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric allocation id.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"node_id": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Id of the node the allocation belongs to.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"ip": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "IP address to allocate.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"port": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Port to allocate (1–65535).",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"alias": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional public alias/hostname for the allocation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"notes": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Notes attached to the allocation (set when assigned to a server).",
			},
			"assigned": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the allocation is assigned to a server.",
			},
		},
	}
}

func (r *allocationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *allocationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan allocationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The Penguin API client is unset. This is a bug in the provider.")
		return
	}

	nodeID := plan.NodeID.ValueInt64()
	body := map[string]any{
		"ip":    plan.IP.ValueString(),
		"ports": []string{strconv.FormatInt(plan.Port.ValueInt64(), 10)},
	}
	putString(body, "alias", plan.Alias)

	if _, err := r.client.Post(ctx, allocationCollection(nodeID), body); err != nil {
		resp.Diagnostics.AddError("Unable to create allocation", err.Error())
		return
	}

	item := findAllocationByIPPort(ctx, r.client, nodeID, plan.IP.ValueString(), plan.Port.ValueInt64(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if item == nil {
		resp.Diagnostics.AddError(
			"Allocation not found after create",
			"The allocation was created but could not be located by ip/port; it may collide with an existing allocation.",
		)
		return
	}
	applyAllocation(item, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *allocationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state allocationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, gone := findAllocationByID(ctx, r.client, state.NodeID.ValueInt64(), state.ID.ValueInt64(), &resp.Diagnostics)
	if gone {
		resp.State.RemoveResource(ctx)
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	applyAllocation(item, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is unreachable — every writable attribute forces replacement — but the
// resource.Resource interface requires it.
func (r *allocationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan allocationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *allocationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state allocationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := joinPath(allocationCollection(state.NodeID.ValueInt64()), strconv.FormatInt(state.ID.ValueInt64(), 10))
	deleteObject(ctx, r.client, itemPath, &resp.Diagnostics)
}

func (r *allocationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	nodeID, id, ok := splitCompositeID(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `<node_id>/<allocation_id>`, e.g. `5/42`.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("node_id"), nodeID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func allocationCollection(nodeID int64) string {
	return joinPath(nodesCollection, strconv.FormatInt(nodeID, 10), "allocations")
}

func findAllocationByID(ctx context.Context, c *client.Client, nodeID, id int64, diags *diag.Diagnostics) (*client.Item, bool) {
	if c == nil {
		diags.AddError("Provider not configured", "The Penguin API client is unset. This is a bug in the provider.")
		return nil, false
	}
	items, err := c.ListItems(ctx, allocationCollection(nodeID), nil)
	if err != nil {
		if client.IsNotFound(err) {
			return nil, true
		}
		diags.AddError("Unable to list allocations", err.Error())
		return nil, false
	}
	want := strconv.FormatInt(id, 10)
	for i := range items {
		if aid, ok := itemID(&items[i]); ok && aid == want {
			return &items[i], false
		}
	}
	return nil, true
}

func findAllocationByIPPort(ctx context.Context, c *client.Client, nodeID int64, ip string, port int64, diags *diag.Diagnostics) *client.Item {
	q := url.Values{}
	q.Set("filter[ip]", ip)
	items, err := c.ListItems(ctx, allocationCollection(nodeID), q)
	if err != nil {
		diags.AddError("Unable to list allocations", err.Error())
		return nil
	}
	for i := range items {
		var a allocationAttrs
		if err := items[i].Decode(&a); err != nil {
			continue
		}
		if a.IP == ip && a.Port == port {
			return &items[i]
		}
	}
	return nil
}

func applyAllocation(item *client.Item, m *allocationModel, diags *diag.Diagnostics) {
	var a allocationAttrs
	if err := item.Decode(&a); err != nil {
		diags.AddError("Unable to decode allocation", err.Error())
		return
	}
	m.ID = types.Int64Value(a.ID)
	m.IP = types.StringValue(a.IP)
	m.Port = types.Int64Value(a.Port)
	m.Alias = stringPtr(a.Alias)
	m.Notes = stringPtr(a.Notes)
	m.Assigned = types.BoolValue(a.Assigned)
}

// splitCompositeID parses a "<parent>/<child>" import id into two int64s.
func splitCompositeID(raw string) (parent, child int64, ok bool) {
	parts := strings.Split(strings.Trim(raw, "/"), "/")
	if len(parts) != 2 {
		return 0, 0, false
	}
	p, err1 := strconv.ParseInt(parts[0], 10, 64)
	c, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return p, c, true
}
