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
	_ datasource.DataSource              = (*nodeDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*nodeDataSource)(nil)
)

// NewNodeDataSource reads a single node by id.
func NewNodeDataSource() datasource.DataSource { return &nodeDataSource{} }

type nodeDataSource struct {
	client *client.Client
}

func (d *nodeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node"
}

func (d *nodeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single node by its numeric id.",
		Attributes: map[string]schema.Attribute{
			"id":                  schema.Int64Attribute{Required: true, MarkdownDescription: "Numeric node id."},
			"uuid":                schema.StringAttribute{Computed: true, MarkdownDescription: "Node UUID."},
			"name":                schema.StringAttribute{Computed: true, MarkdownDescription: "Node name."},
			"description":         schema.StringAttribute{Computed: true, MarkdownDescription: "Description."},
			"public":              schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the node is public."},
			"fqdn":                schema.StringAttribute{Computed: true, MarkdownDescription: "FQDN."},
			"scheme":              schema.StringAttribute{Computed: true, MarkdownDescription: "Connection scheme."},
			"behind_proxy":        schema.BoolAttribute{Computed: true, MarkdownDescription: "Behind a reverse proxy."},
			"memory":              schema.Int64Attribute{Computed: true, MarkdownDescription: "Total memory (MiB)."},
			"memory_overallocate": schema.Int64Attribute{Computed: true, MarkdownDescription: "Memory overallocation %."},
			"disk":                schema.Int64Attribute{Computed: true, MarkdownDescription: "Total disk (MiB)."},
			"disk_overallocate":   schema.Int64Attribute{Computed: true, MarkdownDescription: "Disk overallocation %."},
			"cpu":                 schema.Int64Attribute{Computed: true, MarkdownDescription: "Total CPU %."},
			"cpu_overallocate":    schema.Int64Attribute{Computed: true, MarkdownDescription: "CPU overallocation %."},
			"daemon_base":         schema.StringAttribute{Computed: true, MarkdownDescription: "Wings data base directory."},
			"daemon_sftp":         schema.Int64Attribute{Computed: true, MarkdownDescription: "SFTP port."},
			"daemon_sftp_alias":   schema.StringAttribute{Computed: true, MarkdownDescription: "SFTP alias."},
			"daemon_listen":       schema.Int64Attribute{Computed: true, MarkdownDescription: "HTTP port."},
			"daemon_connect":      schema.Int64Attribute{Computed: true, MarkdownDescription: "Panel connect port."},
			"maintenance_mode":    schema.BoolAttribute{Computed: true, MarkdownDescription: "Maintenance mode."},
			"upload_size":         schema.Int64Attribute{Computed: true, MarkdownDescription: "Max upload size (MiB)."},
			"tags":                schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Tags."},
			"created_at":          schema.StringAttribute{Computed: true, MarkdownDescription: "Creation timestamp."},
			"updated_at":          schema.StringAttribute{Computed: true, MarkdownDescription: "Last-update timestamp."},
		},
	}
}

func (d *nodeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *nodeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config nodeModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, gone := readObject(ctx, d.client, nodeItemPath(config.ID), &resp.Diagnostics)
	if gone {
		resp.Diagnostics.AddError("Node not found", "No node with the given id exists.")
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	applyNode(ctx, item, &config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
