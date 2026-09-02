// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

const nodesCollection = "nodes"

// Node create/config defaults, applied when the operator omits the attribute.
const (
	defaultNodeScheme        = "https"
	defaultNodePublic        = true
	defaultNodeBehindProxy   = false
	defaultNodeMaintenance   = false
	defaultNodeOverallocate  = int64(0)
	defaultNodeDaemonBase    = "/var/lib/pelican/volumes"
	defaultNodeDaemonSFTP    = int64(2022)
	defaultNodeDaemonListen  = int64(8080)
	defaultNodeDaemonConnect = int64(8080)
	defaultNodeUploadSize    = int64(100)
)

var (
	_ resource.Resource                = (*nodeResource)(nil)
	_ resource.ResourceWithConfigure   = (*nodeResource)(nil)
	_ resource.ResourceWithImportState = (*nodeResource)(nil)
)

// NewNodeResource manages a node — a Wings daemon target that hosts servers.
func NewNodeResource() resource.Resource { return &nodeResource{} }

type nodeResource struct {
	client *client.Client
}

type nodeModel struct {
	ID                 types.Int64  `tfsdk:"id"`
	UUID               types.String `tfsdk:"uuid"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	Public             types.Bool   `tfsdk:"public"`
	FQDN               types.String `tfsdk:"fqdn"`
	Scheme             types.String `tfsdk:"scheme"`
	BehindProxy        types.Bool   `tfsdk:"behind_proxy"`
	Memory             types.Int64  `tfsdk:"memory"`
	MemoryOverallocate types.Int64  `tfsdk:"memory_overallocate"`
	Disk               types.Int64  `tfsdk:"disk"`
	DiskOverallocate   types.Int64  `tfsdk:"disk_overallocate"`
	CPU                types.Int64  `tfsdk:"cpu"`
	CPUOverallocate    types.Int64  `tfsdk:"cpu_overallocate"`
	DaemonBase         types.String `tfsdk:"daemon_base"`
	DaemonSFTP         types.Int64  `tfsdk:"daemon_sftp"`
	DaemonSFTPAlias    types.String `tfsdk:"daemon_sftp_alias"`
	DaemonListen       types.Int64  `tfsdk:"daemon_listen"`
	DaemonConnect      types.Int64  `tfsdk:"daemon_connect"`
	MaintenanceMode    types.Bool   `tfsdk:"maintenance_mode"`
	UploadSize         types.Int64  `tfsdk:"upload_size"`
	Tags               types.List   `tfsdk:"tags"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

type nodeAttrs struct {
	ID                 int64    `json:"id"`
	UUID               string   `json:"uuid"`
	Name               string   `json:"name"`
	Description        *string  `json:"description"`
	Public             bool     `json:"public"`
	FQDN               string   `json:"fqdn"`
	Scheme             string   `json:"scheme"`
	BehindProxy        bool     `json:"behind_proxy"`
	Memory             int64    `json:"memory"`
	MemoryOverallocate int64    `json:"memory_overallocate"`
	Disk               int64    `json:"disk"`
	DiskOverallocate   int64    `json:"disk_overallocate"`
	CPU                int64    `json:"cpu"`
	CPUOverallocate    int64    `json:"cpu_overallocate"`
	DaemonBase         string   `json:"daemon_base"`
	DaemonSFTP         int64    `json:"daemon_sftp"`
	DaemonSFTPAlias    *string  `json:"daemon_sftp_alias"`
	DaemonListen       int64    `json:"daemon_listen"`
	DaemonConnect      int64    `json:"daemon_connect"`
	MaintenanceMode    bool     `json:"maintenance_mode"`
	UploadSize         int64    `json:"upload_size"`
	Tags               []string `json:"tags"`
	CreatedAt          *string  `json:"created_at"`
	UpdatedAt          *string  `json:"updated_at"`
}

func (r *nodeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node"
}

func (r *nodeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A node — a Wings daemon target that game servers are deployed onto.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric node id.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"uuid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Node UUID assigned by the panel.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable node name (1–100 characters).",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional description.",
			},
			"public": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(defaultNodePublic),
				MarkdownDescription: "Whether the node is eligible for automatic deployment. Defaults to `true`.",
			},
			"fqdn": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Fully-qualified domain name (or IP) the panel reaches Wings on. Must not be a loopback/unspecified address.",
			},
			"scheme": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(defaultNodeScheme),
				MarkdownDescription: "Connection scheme, `http` or `https`. Defaults to `https`.",
			},
			"behind_proxy": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(defaultNodeBehindProxy),
				MarkdownDescription: "Whether Wings sits behind a reverse proxy. Defaults to `false`.",
			},
			"memory": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Total memory available to servers, in MiB.",
			},
			"memory_overallocate": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(defaultNodeOverallocate),
				MarkdownDescription: "Memory overallocation percentage (`-1` disables the limit). Defaults to `0`.",
			},
			"disk": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Total disk available to servers, in MiB.",
			},
			"disk_overallocate": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(defaultNodeOverallocate),
				MarkdownDescription: "Disk overallocation percentage (`-1` disables the limit). Defaults to `0`.",
			},
			"cpu": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Total CPU available to servers, in percent (100 = one core).",
			},
			"cpu_overallocate": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(defaultNodeOverallocate),
				MarkdownDescription: "CPU overallocation percentage (`-1` disables the limit). Defaults to `0`.",
			},
			"daemon_base": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(defaultNodeDaemonBase),
				MarkdownDescription: "Absolute base directory Wings stores server data under. Defaults to `" + defaultNodeDaemonBase + "`.",
			},
			"daemon_sftp": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(defaultNodeDaemonSFTP),
				MarkdownDescription: "SFTP port Wings listens on. Defaults to `2022`.",
			},
			"daemon_sftp_alias": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional public SFTP alias/hostname.",
			},
			"daemon_listen": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(defaultNodeDaemonListen),
				MarkdownDescription: "HTTP port Wings listens on. Defaults to `8080`.",
			},
			"daemon_connect": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(defaultNodeDaemonConnect),
				MarkdownDescription: "Port the panel connects to Wings on. Defaults to `8080`.",
			},
			"maintenance_mode": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(defaultNodeMaintenance),
				MarkdownDescription: "Whether the node is in maintenance mode. Defaults to `false`.",
			},
			"upload_size": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(defaultNodeUploadSize),
				MarkdownDescription: "Maximum web upload size, in MiB. Defaults to `100`.",
			},
			"tags": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Free-form tags.",
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

func (r *nodeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *nodeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan nodeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item := createObject(ctx, r.client, nodesCollection, buildNodeBody(ctx, plan, &resp.Diagnostics), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	applyNode(ctx, item, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *nodeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state nodeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, gone := readObject(ctx, r.client, nodeItemPath(state.ID), &resp.Diagnostics)
	if gone {
		resp.State.RemoveResource(ctx)
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	applyNode(ctx, item, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *nodeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state nodeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	item := updateObject(ctx, r.client, nodeItemPath(plan.ID), buildNodeBody(ctx, plan, &resp.Diagnostics), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	applyNode(ctx, item, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *nodeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state nodeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteObject(ctx, r.client, nodeItemPath(state.ID), &resp.Diagnostics)
}

func (r *nodeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importInt64ID(ctx, req, resp)
}

func nodeItemPath(id types.Int64) string {
	return joinPath(nodesCollection, strconv.FormatInt(id.ValueInt64(), 10))
}

func buildNodeBody(ctx context.Context, m nodeModel, diags *diag.Diagnostics) map[string]any {
	body := map[string]any{}
	putString(body, "name", m.Name)
	putString(body, "description", m.Description)
	putBool(body, "public", m.Public)
	putString(body, "fqdn", m.FQDN)
	putString(body, "scheme", m.Scheme)
	putBool(body, "behind_proxy", m.BehindProxy)
	putInt64(body, "memory", m.Memory)
	putInt64(body, "memory_overallocate", m.MemoryOverallocate)
	putInt64(body, "disk", m.Disk)
	putInt64(body, "disk_overallocate", m.DiskOverallocate)
	putInt64(body, "cpu", m.CPU)
	putInt64(body, "cpu_overallocate", m.CPUOverallocate)
	putString(body, "daemon_base", m.DaemonBase)
	putInt64(body, "daemon_sftp", m.DaemonSFTP)
	putString(body, "daemon_sftp_alias", m.DaemonSFTPAlias)
	putInt64(body, "daemon_listen", m.DaemonListen)
	putInt64(body, "daemon_connect", m.DaemonConnect)
	putBool(body, "maintenance_mode", m.MaintenanceMode)
	putInt64(body, "upload_size", m.UploadSize)
	putStringList(ctx, body, "tags", m.Tags, diags)
	return body
}

func applyNode(ctx context.Context, item *client.Item, m *nodeModel, diags *diag.Diagnostics) {
	var a nodeAttrs
	if err := item.Decode(&a); err != nil {
		diags.AddError("Unable to decode node", err.Error())
		return
	}
	m.ID = types.Int64Value(a.ID)
	m.UUID = types.StringValue(a.UUID)
	m.Name = types.StringValue(a.Name)
	m.Description = stringPtr(a.Description)
	m.Public = types.BoolValue(a.Public)
	m.FQDN = types.StringValue(a.FQDN)
	m.Scheme = types.StringValue(a.Scheme)
	m.BehindProxy = types.BoolValue(a.BehindProxy)
	m.Memory = types.Int64Value(a.Memory)
	m.MemoryOverallocate = types.Int64Value(a.MemoryOverallocate)
	m.Disk = types.Int64Value(a.Disk)
	m.DiskOverallocate = types.Int64Value(a.DiskOverallocate)
	m.CPU = types.Int64Value(a.CPU)
	m.CPUOverallocate = types.Int64Value(a.CPUOverallocate)
	m.DaemonBase = types.StringValue(a.DaemonBase)
	m.DaemonSFTP = types.Int64Value(a.DaemonSFTP)
	m.DaemonSFTPAlias = stringPtr(a.DaemonSFTPAlias)
	m.DaemonListen = types.Int64Value(a.DaemonListen)
	m.DaemonConnect = types.Int64Value(a.DaemonConnect)
	m.MaintenanceMode = types.BoolValue(a.MaintenanceMode)
	m.UploadSize = types.Int64Value(a.UploadSize)
	m.Tags = listFromStringSlice(ctx, a.Tags, diags)
	m.CreatedAt = stringPtr(a.CreatedAt)
	m.UpdatedAt = stringPtr(a.UpdatedAt)
}
