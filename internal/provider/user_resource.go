// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

const (
	usersCollection    = "users"
	defaultUserLang    = "en"
	defaultUserManaged = false
)

var (
	_ resource.Resource                = (*userResource)(nil)
	_ resource.ResourceWithConfigure   = (*userResource)(nil)
	_ resource.ResourceWithImportState = (*userResource)(nil)
)

// NewUserResource manages a panel user account.
func NewUserResource() resource.Resource { return &userResource{} }

type userResource struct {
	client *client.Client
}

type userModel struct {
	ID                  types.Int64  `tfsdk:"id"`
	UUID                types.String `tfsdk:"uuid"`
	ExternalID          types.String `tfsdk:"external_id"`
	IsManagedExternally types.Bool   `tfsdk:"is_managed_externally"`
	Email               types.String `tfsdk:"email"`
	Username            types.String `tfsdk:"username"`
	Password            types.String `tfsdk:"password"`
	Language            types.String `tfsdk:"language"`
	Timezone            types.String `tfsdk:"timezone"`
	RootAdmin           types.Bool   `tfsdk:"root_admin"`
	TwoFactorEnabled    types.Bool   `tfsdk:"two_factor_enabled"`
	CreatedAt           types.String `tfsdk:"created_at"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
}

type userAttrs struct {
	ID                  int64   `json:"id"`
	ExternalID          *string `json:"external_id"`
	UUID                string  `json:"uuid"`
	Username            string  `json:"username"`
	Email               string  `json:"email"`
	Language            string  `json:"language"`
	IsManagedExternally bool    `json:"is_managed_externally"`
	RootAdmin           bool    `json:"root_admin"`
	TwoFactorEnabled    bool    `json:"2fa_enabled"`
	CreatedAt           *string `json:"created_at"`
	UpdatedAt           *string `json:"updated_at"`
}

func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A panel user account.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric user id.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"uuid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "User UUID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"external_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional external identifier (must be unique).",
			},
			"is_managed_externally": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(defaultUserManaged),
				MarkdownDescription: "Whether the account is managed by an external identity provider. Defaults to `false`.",
			},
			"email": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Email address (unique).",
			},
			"username": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Username (unique).",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Initial account password. Never returned by the API; kept in state to detect changes. Omit to let the panel send a set-up email.",
			},
			"language": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(defaultUserLang),
				MarkdownDescription: "UI language code. Defaults to `en`.",
			},
			"timezone": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "IANA timezone (e.g. `UTC`). Accepted on write but not returned by the API.",
			},
			"root_admin": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the user is a root administrator (managed via roles, not this resource).",
			},
			"two_factor_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether two-factor authentication is enabled.",
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

func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item := createObject(ctx, r.client, usersCollection, buildUserBody(plan), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	applyUser(item, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, gone := readObject(ctx, r.client, userItemPath(state.ID), &resp.Diagnostics)
	if gone {
		resp.State.RemoveResource(ctx)
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	applyUser(item, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	item := updateObject(ctx, r.client, userItemPath(plan.ID), buildUserBody(plan), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	applyUser(item, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteObject(ctx, r.client, userItemPath(state.ID), &resp.Diagnostics)
}

func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importInt64ID(ctx, req, resp)
}

func userItemPath(id types.Int64) string {
	return joinPath(usersCollection, strconv.FormatInt(id.ValueInt64(), 10))
}

func buildUserBody(m userModel) map[string]any {
	body := map[string]any{}
	putString(body, "email", m.Email)
	putString(body, "username", m.Username)
	putString(body, "external_id", m.ExternalID)
	putBool(body, "is_managed_externally", m.IsManagedExternally)
	putString(body, "language", m.Language)
	putString(body, "timezone", m.Timezone)
	putString(body, "password", m.Password)
	return body
}

// applyUser maps the read envelope onto the model. It deliberately does not
// touch password or timezone — the API never returns them — so their config
// values are preserved in state.
func applyUser(item *client.Item, m *userModel, diags *diag.Diagnostics) {
	var a userAttrs
	if err := item.Decode(&a); err != nil {
		diags.AddError("Unable to decode user", err.Error())
		return
	}
	m.ID = types.Int64Value(a.ID)
	m.UUID = types.StringValue(a.UUID)
	m.ExternalID = stringPtr(a.ExternalID)
	m.IsManagedExternally = types.BoolValue(a.IsManagedExternally)
	m.Email = types.StringValue(a.Email)
	m.Username = types.StringValue(a.Username)
	m.Language = types.StringValue(a.Language)
	m.RootAdmin = types.BoolValue(a.RootAdmin)
	m.TwoFactorEnabled = types.BoolValue(a.TwoFactorEnabled)
	m.CreatedAt = stringPtr(a.CreatedAt)
	m.UpdatedAt = stringPtr(a.UpdatedAt)
}
