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
	_ datasource.DataSource              = (*serverDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverDataSource)(nil)
)

// NewServerDataSource reads a single server by id.
func NewServerDataSource() datasource.DataSource { return &serverDataSource{} }

type serverDataSource struct {
	client *client.Client
}

func (d *serverDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server"
}

func (d *serverDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single server by its numeric id.",
		Attributes: map[string]schema.Attribute{
			"id":                  schema.Int64Attribute{Required: true, MarkdownDescription: "Numeric server id."},
			"external_id":         schema.StringAttribute{Computed: true, MarkdownDescription: "External identifier."},
			"uuid":                schema.StringAttribute{Computed: true, MarkdownDescription: "Server UUID."},
			"identifier":          schema.StringAttribute{Computed: true, MarkdownDescription: "Short identifier."},
			"name":                schema.StringAttribute{Computed: true, MarkdownDescription: "Server name."},
			"description":         schema.StringAttribute{Computed: true, MarkdownDescription: "Description."},
			"user":                schema.Int64Attribute{Computed: true, MarkdownDescription: "Owner user id."},
			"egg":                 schema.Int64Attribute{Computed: true, MarkdownDescription: "Egg id."},
			"node":                schema.Int64Attribute{Computed: true, MarkdownDescription: "Node id."},
			"allocation":          schema.Int64Attribute{Computed: true, MarkdownDescription: "Default allocation id."},
			"docker_image":        schema.StringAttribute{Computed: true, MarkdownDescription: "Container image."},
			"startup":             schema.StringAttribute{Computed: true, MarkdownDescription: "Startup command."},
			"environment":         schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Effective environment variables."},
			"skip_scripts":        schema.BoolAttribute{Computed: true, MarkdownDescription: "Write-only; not returned by the API."},
			"oom_killer":          schema.BoolAttribute{Computed: true, MarkdownDescription: "OOM killer enabled."},
			"start_on_completion": schema.BoolAttribute{Computed: true, MarkdownDescription: "Write-only; not returned by the API."},
			"status":              schema.StringAttribute{Computed: true, MarkdownDescription: "Lifecycle status."},
			"suspended":           schema.BoolAttribute{Computed: true, MarkdownDescription: "Suspended."},
			"limit_memory":        schema.Int64Attribute{Computed: true, MarkdownDescription: "Memory limit (MiB)."},
			"limit_swap":          schema.Int64Attribute{Computed: true, MarkdownDescription: "Swap limit (MiB)."},
			"limit_disk":          schema.Int64Attribute{Computed: true, MarkdownDescription: "Disk limit (MiB)."},
			"limit_io":            schema.Int64Attribute{Computed: true, MarkdownDescription: "Block IO weight."},
			"limit_cpu":           schema.Int64Attribute{Computed: true, MarkdownDescription: "CPU limit (%)."},
			"limit_threads":       schema.StringAttribute{Computed: true, MarkdownDescription: "CPU pinning."},
			"feature_databases":   schema.Int64Attribute{Computed: true, MarkdownDescription: "Database allowance."},
			"feature_allocations": schema.Int64Attribute{Computed: true, MarkdownDescription: "Allocation allowance."},
			"feature_backups":     schema.Int64Attribute{Computed: true, MarkdownDescription: "Backup allowance."},
			"created_at":          schema.StringAttribute{Computed: true, MarkdownDescription: "Creation timestamp."},
			"updated_at":          schema.StringAttribute{Computed: true, MarkdownDescription: "Last-update timestamp."},
		},
	}
}

func (d *serverDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *serverDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config serverModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, gone := readObject(ctx, d.client, serverItemPath(config.ID), &resp.Diagnostics)
	if gone {
		resp.Diagnostics.AddError("Server not found", "No server with the given id exists.")
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	applyServer(ctx, item, &config, &resp.Diagnostics, true)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
