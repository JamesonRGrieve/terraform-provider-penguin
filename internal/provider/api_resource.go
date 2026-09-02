// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

var (
	_ resource.Resource                = (*apiResource)(nil)
	_ resource.ResourceWithConfigure   = (*apiResource)(nil)
	_ resource.ResourceWithImportState = (*apiResource)(nil)
)

// NewAPIResource is the generic escape hatch: it manages any Application API
// object addressable at a collection path, giving complete API coverage without
// a typed resource. The typed resources are ergonomic sugar over these same
// endpoints.
func NewAPIResource() resource.Resource { return &apiResource{} }

type apiResource struct {
	client *client.Client
}

type apiResourceModel struct {
	Path       types.String `tfsdk:"path"`
	Body       types.String `tfsdk:"body"`
	ID         types.String `tfsdk:"id"`
	Object     types.String `tfsdk:"object"`
	Attributes types.String `tfsdk:"attributes"`
}

func (r *apiResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_resource"
}

func (r *apiResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an arbitrary Application API object by collection path. This generic " +
			"resource provides complete API coverage; the typed resources (`penguin_node`, `penguin_user`, …) " +
			"are ergonomic sugar over the same endpoints. Create issues `POST /api/application/<path>`; read, " +
			"update and delete address `<path>/<id>`.",
		Attributes: map[string]schema.Attribute{
			"path": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Collection path relative to `/api/application`, e.g. `users` or " +
					"`nodes/5/allocations`. Changing it forces replacement.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"body": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "JSON request body sent on create and update. Use `jsonencode({...})` for " +
					"a stable, canonical value.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier of the managed object.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"object": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Object type reported by the API envelope.",
			},
			"attributes": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Canonical JSON of the object's attributes as last read from the API.",
			},
		},
	}
}

func (r *apiResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *apiResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	item, err := r.client.CreateItem(ctx, plan.Path.ValueString(), bodyArg(plan.Body))
	if err != nil {
		resp.Diagnostics.AddError("Unable to create object", err.Error())
		return
	}
	if item == nil {
		resp.Diagnostics.AddError(
			"Empty create response",
			fmt.Sprintf("POST /api/application/%s returned no object envelope.", plan.Path.ValueString()),
		)
		return
	}
	if !r.applyItem(&plan, item, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *apiResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	itemPath := joinPath(state.Path.ValueString(), state.ID.ValueString())
	item, err := r.client.GetItem(ctx, itemPath, nil)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read object", err.Error())
		return
	}
	if !r.applyItem(&state, item, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *apiResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state apiResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	itemPath := joinPath(plan.Path.ValueString(), plan.ID.ValueString())
	item, err := r.client.UpdateItem(ctx, itemPath, bodyArg(plan.Body))
	if err != nil {
		resp.Diagnostics.AddError("Unable to update object", err.Error())
		return
	}
	if item == nil {
		item, err = r.client.GetItem(ctx, itemPath, nil)
		if err != nil {
			resp.Diagnostics.AddError("Unable to read object after update", err.Error())
			return
		}
	}
	if !r.applyItem(&plan, item, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *apiResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	itemPath := joinPath(state.Path.ValueString(), state.ID.ValueString())
	if err := r.client.DeleteItem(ctx, itemPath); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete object", err.Error())
	}
}

func (r *apiResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	full := strings.Trim(req.ID, "/")
	idx := strings.LastIndex(full, "/")
	if idx <= 0 || idx == len(full)-1 {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"Expected `<path>/<id>`, e.g. `nodes/5` or `servers/3/databases/7`.",
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("path"), full[:idx])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), full[idx+1:])...)
}

func (r *apiResource) applyItem(m *apiResourceModel, item *client.Item, diags *diag.Diagnostics) bool {
	id, ok := itemID(item)
	if !ok {
		diags.AddError("Missing object id", "The API object has no `id` attribute; it cannot be tracked as a resource.")
		return false
	}
	attrsJSON, err := item.AttributesJSON()
	if err != nil {
		diags.AddError("Unable to encode attributes", err.Error())
		return false
	}
	m.ID = types.StringValue(id)
	m.Object = types.StringValue(item.Object)
	m.Attributes = types.StringValue(attrsJSON)
	return true
}
