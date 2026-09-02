// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

var (
	_ datasource.DataSource              = (*allocationDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*allocationDataSource)(nil)
)

// NewAllocationDataSource reads a single allocation by node id and allocation id.
func NewAllocationDataSource() datasource.DataSource { return &allocationDataSource{} }

type allocationDataSource struct {
	client *client.Client
}

func (d *allocationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allocation"
}

func (d *allocationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single allocation by its node id and allocation id.",
		Attributes: map[string]schema.Attribute{
			"id":       schema.Int64Attribute{Required: true, MarkdownDescription: "Numeric allocation id."},
			"node_id":  schema.Int64Attribute{Required: true, MarkdownDescription: "Id of the node the allocation belongs to."},
			"ip":       schema.StringAttribute{Computed: true, MarkdownDescription: "Allocated IP address."},
			"port":     schema.Int64Attribute{Computed: true, MarkdownDescription: "Allocated port."},
			"alias":    schema.StringAttribute{Computed: true, MarkdownDescription: "Public alias/hostname."},
			"notes":    schema.StringAttribute{Computed: true, MarkdownDescription: "Notes."},
			"assigned": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether assigned to a server."},
		},
	}
}

func (d *allocationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *allocationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config allocationModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, gone := findAllocationByID(ctx, d.client, config.NodeID.ValueInt64(), config.ID.ValueInt64(), &resp.Diagnostics)
	if gone {
		resp.Diagnostics.AddError("Allocation not found", "No allocation with the given id exists on the node.")
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	applyAllocation(item, &config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
