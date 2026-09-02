// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

const (
	envEndpoint = "PENGUIN_ENDPOINT"
	envToken    = "PENGUIN_API_TOKEN"
	envInsecure = "PENGUIN_INSECURE"
)

var _ provider.Provider = (*penguinProvider)(nil)

type penguinProvider struct {
	version string
}

// New returns a provider factory for the given build version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &penguinProvider{version: version}
	}
}

type penguinProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIToken types.String `tfsdk:"api_token"`
	Insecure types.Bool   `tfsdk:"insecure"`
}

func (p *penguinProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "penguin"
	resp.Version = p.version
}

func (p *penguinProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Penguin provider manages a Penguin / Pelican game-server control panel through its admin Application API. " +
			"Configure it with the panel `endpoint` and an application API token (`papp_...`).",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Base URL of the Penguin Panel, e.g. `https://panel.example.com`. " +
					"May also be set via the `" + envEndpoint + "` environment variable.",
			},
			"api_token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "Application API token (`papp_...`) with the required admin ACL scopes. " +
					"May also be set via the `" + envToken + "` environment variable.",
			},
			"insecure": schema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "Skip TLS certificate verification when talking to the panel. " +
					"May also be set via `" + envInsecure + "`. Defaults to `false`.",
			},
		},
	}
}

func (p *penguinProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg penguinProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := stringOrEnv(cfg.Endpoint, envEndpoint)
	token := stringOrEnv(cfg.APIToken, envToken)
	insecure := boolOrEnv(cfg.Insecure, envInsecure)

	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Missing panel endpoint",
			"Set the `endpoint` attribute or the "+envEndpoint+" environment variable to the panel base URL.",
		)
	}
	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Missing API token",
			"Set the `api_token` attribute or the "+envToken+" environment variable to an application API token (papp_...).",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := client.New(client.Config{
		Endpoint:  endpoint,
		Token:     token,
		Insecure:  insecure,
		UserAgent: "terraform-provider-penguin/" + p.version,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Penguin API client", err.Error())
		return
	}

	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *penguinProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAPIResource,
		NewNodeResource,
		NewUserResource,
		NewRoleResource,
		NewDatabaseHostResource,
		NewAllocationResource,
		NewServerResource,
		NewServerDatabaseResource,
	}
}

func (p *penguinProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewAPIDataSource,
		NewNodeDataSource,
		NewUserDataSource,
		NewRoleDataSource,
		NewDatabaseHostDataSource,
		NewAllocationDataSource,
		NewServerDataSource,
		NewServerDatabaseDataSource,
	}
}
