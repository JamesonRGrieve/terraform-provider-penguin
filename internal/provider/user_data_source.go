// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

var (
	_ datasource.DataSource              = (*userDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*userDataSource)(nil)
)

// NewUserDataSource reads a single user by id.
func NewUserDataSource() datasource.DataSource { return &userDataSource{} }

type userDataSource struct {
	client *client.Client
}

func (d *userDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single user by its numeric id.",
		Attributes: map[string]schema.Attribute{
			"id":                    schema.Int64Attribute{Required: true, MarkdownDescription: "Numeric user id."},
			"uuid":                  schema.StringAttribute{Computed: true, MarkdownDescription: "User UUID."},
			"external_id":           schema.StringAttribute{Computed: true, MarkdownDescription: "External identifier."},
			"is_managed_externally": schema.BoolAttribute{Computed: true, MarkdownDescription: "Externally managed."},
			"email":                 schema.StringAttribute{Computed: true, MarkdownDescription: "Email address."},
			"username":              schema.StringAttribute{Computed: true, MarkdownDescription: "Username."},
			"password":              schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Never returned by the API; always null."},
			"language":              schema.StringAttribute{Computed: true, MarkdownDescription: "UI language code."},
			"timezone":              schema.StringAttribute{Computed: true, MarkdownDescription: "Not returned by the API; always null."},
			"root_admin":            schema.BoolAttribute{Computed: true, MarkdownDescription: "Root administrator."},
			"two_factor_enabled":    schema.BoolAttribute{Computed: true, MarkdownDescription: "Two-factor authentication enabled."},
			"created_at":            schema.StringAttribute{Computed: true, MarkdownDescription: "Creation timestamp."},
			"updated_at":            schema.StringAttribute{Computed: true, MarkdownDescription: "Last-update timestamp."},
		},
	}
}

func (d *userDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *userDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config userModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, gone := readObject(ctx, d.client, userItemPath(config.ID), &resp.Diagnostics)
	if gone {
		resp.Diagnostics.AddError("User not found", "No user with the given id exists.")
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	applyUser(item, &config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
