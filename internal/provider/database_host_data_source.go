// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

var (
	_ datasource.DataSource              = (*databaseHostDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*databaseHostDataSource)(nil)
)

// NewDatabaseHostDataSource reads a single database host by id.
func NewDatabaseHostDataSource() datasource.DataSource { return &databaseHostDataSource{} }

type databaseHostDataSource struct {
	client *client.Client
}

func (d *databaseHostDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_host"
}

func (d *databaseHostDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single database host by its numeric id.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.Int64Attribute{Required: true, MarkdownDescription: "Numeric database host id."},
			"name":       schema.StringAttribute{Computed: true, MarkdownDescription: "Display name."},
			"host":       schema.StringAttribute{Computed: true, MarkdownDescription: "Hostname or IP."},
			"port":       schema.Int64Attribute{Computed: true, MarkdownDescription: "Port."},
			"username":   schema.StringAttribute{Computed: true, MarkdownDescription: "Privileged username."},
			"password":   schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Never returned by the API; always null."},
			"node_ids":   schema.ListAttribute{Computed: true, ElementType: types.Int64Type, MarkdownDescription: "Not returned by the API; always null."},
			"created_at": schema.StringAttribute{Computed: true, MarkdownDescription: "Creation timestamp."},
			"updated_at": schema.StringAttribute{Computed: true, MarkdownDescription: "Last-update timestamp."},
		},
	}
}

func (d *databaseHostDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *databaseHostDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config databaseHostModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, gone := readObject(ctx, d.client, databaseHostItemPath(config.ID), &resp.Diagnostics)
	if gone {
		resp.Diagnostics.AddError("Database host not found", "No database host with the given id exists.")
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	applyDatabaseHost(item, &config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
