// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

var (
	_ datasource.DataSource              = (*serverDatabaseDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverDatabaseDataSource)(nil)
)

// NewServerDatabaseDataSource reads a single server database by server id and id.
func NewServerDatabaseDataSource() datasource.DataSource { return &serverDatabaseDataSource{} }

type serverDatabaseDataSource struct {
	client *client.Client
}

func (d *serverDatabaseDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_database"
}

func (d *serverDatabaseDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single server database by its server id and database id.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.Int64Attribute{Required: true, MarkdownDescription: "Numeric database id."},
			"server_id":       schema.Int64Attribute{Required: true, MarkdownDescription: "Id of the owning server."},
			"host_id":         schema.Int64Attribute{Computed: true, MarkdownDescription: "Database host id."},
			"database":        schema.StringAttribute{Computed: true, MarkdownDescription: "Database name."},
			"remote":          schema.StringAttribute{Computed: true, MarkdownDescription: "Allowed remote pattern."},
			"username":        schema.StringAttribute{Computed: true, MarkdownDescription: "Generated username."},
			"password":        schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Generated password."},
			"max_connections": schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum connections."},
			"created_at":      schema.StringAttribute{Computed: true, MarkdownDescription: "Creation timestamp."},
			"updated_at":      schema.StringAttribute{Computed: true, MarkdownDescription: "Last-update timestamp."},
		},
	}
}

func (d *serverDatabaseDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *serverDatabaseDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config serverDatabaseModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, gone := readServerDatabase(ctx, d.client, config.ServerID.ValueInt64(), config.ID.ValueInt64(), &resp.Diagnostics)
	if gone {
		resp.Diagnostics.AddError("Server database not found", "No database with the given id exists on the server.")
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	applyServerDatabase(item, &config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
