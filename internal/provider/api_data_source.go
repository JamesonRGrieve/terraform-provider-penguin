// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

var (
	_ datasource.DataSource              = (*apiDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*apiDataSource)(nil)
)

// NewAPIDataSource is the generic read escape hatch: it GETs any Application API
// path (single object or collection) and returns the raw response as canonical
// JSON, giving complete read coverage without a typed data source.
func NewAPIDataSource() datasource.DataSource { return &apiDataSource{} }

type apiDataSource struct {
	client *client.Client
}

type apiDataSourceModel struct {
	Path  types.String `tfsdk:"path"`
	Query types.Map    `tfsdk:"query"`
	ID    types.String `tfsdk:"id"`
	JSON  types.String `tfsdk:"json"`
}

func (d *apiDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api"
}

func (d *apiDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an arbitrary Application API path (single object or collection) and returns " +
			"the raw response body as canonical JSON. Use with Terraform's `jsondecode()` to consume any endpoint " +
			"without a typed data source.",
		Attributes: map[string]schema.Attribute{
			"path": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Path relative to `/api/application`, e.g. `nodes`, `nodes/5`, or " +
					"`servers/3/databases`.",
			},
			"query": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Query parameters to append, e.g. `{ \"per_page\" = \"100\", \"include\" = \"servers\" }`.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Set to the requested path.",
			},
			"json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The API response body as canonical JSON.",
			},
		},
	}
}

func (d *apiDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *apiDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config apiDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := mapToValues(ctx, config.Query, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	body, err := d.client.Do(ctx, http.MethodGet, config.Path.ValueString(), query, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read API path", err.Error())
		return
	}
	canonical, err := canonicalJSON(body)
	if err != nil {
		resp.Diagnostics.AddError("Unable to canonicalize API response", err.Error())
		return
	}

	config.ID = config.Path
	config.JSON = types.StringValue(canonical)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
