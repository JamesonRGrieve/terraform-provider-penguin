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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

const serversCollection = "servers"

// Server build/limit defaults, applied when the operator omits the attribute.
const (
	defaultServerSwap    = int64(-1)
	defaultServerIO      = int64(500)
	defaultServerFeature = int64(0)
	defaultServerOOM     = false
)

var (
	_ resource.Resource                = (*serverResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverResource)(nil)
	_ resource.ResourceWithImportState = (*serverResource)(nil)
)

// NewServerResource manages a game/application server — the central object that
// runs an egg on a node behind an allocation.
func NewServerResource() resource.Resource { return &serverResource{} }

type serverResource struct {
	client *client.Client
}

type serverModel struct {
	ID                types.Int64  `tfsdk:"id"`
	ExternalID        types.String `tfsdk:"external_id"`
	UUID              types.String `tfsdk:"uuid"`
	Identifier        types.String `tfsdk:"identifier"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	User              types.Int64  `tfsdk:"user"`
	EggID             types.Int64  `tfsdk:"egg"`
	NodeID            types.Int64  `tfsdk:"node"`
	AllocationID      types.Int64  `tfsdk:"allocation"`
	DockerImage       types.String `tfsdk:"docker_image"`
	Startup           types.String `tfsdk:"startup"`
	Environment       types.Map    `tfsdk:"environment"`
	SkipScripts       types.Bool   `tfsdk:"skip_scripts"`
	OomKiller         types.Bool   `tfsdk:"oom_killer"`
	StartOnCompletion types.Bool   `tfsdk:"start_on_completion"`
	Status            types.String `tfsdk:"status"`
	Suspended         types.Bool   `tfsdk:"suspended"`
	LimitMemory       types.Int64  `tfsdk:"limit_memory"`
	LimitSwap         types.Int64  `tfsdk:"limit_swap"`
	LimitDisk         types.Int64  `tfsdk:"limit_disk"`
	LimitIO           types.Int64  `tfsdk:"limit_io"`
	LimitCPU          types.Int64  `tfsdk:"limit_cpu"`
	LimitThreads      types.String `tfsdk:"limit_threads"`
	FeatureDatabases  types.Int64  `tfsdk:"feature_databases"`
	FeatureAllocs     types.Int64  `tfsdk:"feature_allocations"`
	FeatureBackups    types.Int64  `tfsdk:"feature_backups"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
}

type serverAttrs struct {
	ID          int64   `json:"id"`
	ExternalID  *string `json:"external_id"`
	UUID        string  `json:"uuid"`
	Identifier  string  `json:"identifier"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Suspended   bool    `json:"suspended"`
	Limits      struct {
		Memory    int64   `json:"memory"`
		Swap      int64   `json:"swap"`
		Disk      int64   `json:"disk"`
		IO        int64   `json:"io"`
		CPU       int64   `json:"cpu"`
		Threads   *string `json:"threads"`
		OomKiller bool    `json:"oom_killer"`
	} `json:"limits"`
	FeatureLimits struct {
		Databases   int64 `json:"databases"`
		Allocations int64 `json:"allocations"`
		Backups     int64 `json:"backups"`
	} `json:"feature_limits"`
	User      int64 `json:"user"`
	Node      int64 `json:"node"`
	Alloc     int64 `json:"allocation"`
	Egg       int64 `json:"egg"`
	Container struct {
		StartupCommand string         `json:"startup_command"`
		Image          string         `json:"image"`
		Environment    map[string]any `json:"environment"`
	} `json:"container"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

func (r *serverResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server"
}

func (r *serverResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A game or application server — the central object that runs an egg on a node behind an allocation.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric server id.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"external_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional external identifier (unique).",
			},
			"uuid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Server UUID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"identifier": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Short server identifier (used in the client API and SFTP).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Server display name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional description.",
			},
			"user": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Id of the owning user.",
			},
			"egg": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Id of the egg the server runs.",
			},
			"node": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Id of the node the server is deployed on (derived from the default allocation).",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"allocation": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Id of the default (primary) allocation. Must be an unassigned allocation at create time.",
			},
			"docker_image": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Container image / OCI base. Defaults to the egg's image when omitted.",
			},
			"startup": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Startup command. Defaults to the egg's startup when omitted.",
			},
			"environment": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Egg environment variables (name → value). Provide every variable the egg requires.",
			},
			"skip_scripts": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Skip the egg install script on create/reinstall. Write-only; not returned by the API.",
			},
			"oom_killer": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(defaultServerOOM),
				MarkdownDescription: "Enable the out-of-memory killer. Defaults to `false`.",
			},
			"start_on_completion": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Start the server as soon as installation completes. Write-only; not returned by the API.",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current lifecycle status (installing, suspended, etc.); null when running.",
			},
			"suspended": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the server is suspended.",
			},
			"limit_memory": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Memory limit in MiB (0 = unlimited).",
			},
			"limit_swap": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(defaultServerSwap),
				MarkdownDescription: "Swap limit in MiB (`-1` = unlimited, `0` = disabled). Defaults to `-1`.",
			},
			"limit_disk": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Disk limit in MiB (0 = unlimited).",
			},
			"limit_io": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(defaultServerIO),
				MarkdownDescription: "Block IO weight (10–1000). Defaults to `500`.",
			},
			"limit_cpu": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "CPU limit in percent (0 = unlimited, 100 = one core).",
			},
			"limit_threads": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "CPU pinning, e.g. `0`, `0-3`, `0,2`. Null for no pinning.",
			},
			"feature_databases": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(defaultServerFeature),
				MarkdownDescription: "Maximum databases the server may create. Defaults to `0`.",
			},
			"feature_allocations": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(defaultServerFeature),
				MarkdownDescription: "Maximum additional allocations. Defaults to `0`.",
			},
			"feature_backups": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(defaultServerFeature),
				MarkdownDescription: "Maximum backups. Defaults to `0`.",
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

func (r *serverResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *serverResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item := createObject(ctx, r.client, serversCollection, buildServerCreateBody(ctx, plan, &resp.Diagnostics), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	applyServer(ctx, item, &plan, &resp.Diagnostics, false)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, gone := readObject(ctx, r.client, serverItemPath(state.ID), &resp.Diagnostics)
	if gone {
		resp.State.RemoveResource(ctx)
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	applyServer(ctx, item, &state, &resp.Diagnostics, false)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serverResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state serverModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	id := plan.ID.ValueInt64()

	// The panel splits server mutation across three endpoints; send all three so
	// any changed field lands regardless of which group it belongs to.
	r.patch(ctx, id, "details", buildServerDetailsBody(plan), &resp.Diagnostics)
	r.patch(ctx, id, "build", buildServerBuildBody(plan), &resp.Diagnostics)
	r.patch(ctx, id, "startup", buildServerStartupBody(ctx, plan, &resp.Diagnostics), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	item, gone := readObject(ctx, r.client, serverItemPath(plan.ID), &resp.Diagnostics)
	if gone {
		resp.Diagnostics.AddError("Server vanished during update", "The server could not be read back after updating.")
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	applyServer(ctx, item, &plan, &resp.Diagnostics, false)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serverModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteObject(ctx, r.client, serverItemPath(state.ID), &resp.Diagnostics)
}

func (r *serverResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importInt64ID(ctx, req, resp)
}

func (r *serverResource) patch(ctx context.Context, id int64, sub string, body any, diags *diag.Diagnostics) {
	if diags.HasError() || r.client == nil {
		if r.client == nil {
			diags.AddError("Provider not configured", "The Penguin API client is unset. This is a bug in the provider.")
		}
		return
	}
	itemPath := joinPath(serversCollection, strconv.FormatInt(id, 10), sub)
	if _, err := r.client.UpdateItem(ctx, itemPath, body); err != nil {
		diags.AddError("Unable to update server "+sub, err.Error())
	}
}

func serverItemPath(id types.Int64) string {
	return joinPath(serversCollection, strconv.FormatInt(id.ValueInt64(), 10))
}

func buildServerCreateBody(ctx context.Context, m serverModel, diags *diag.Diagnostics) map[string]any {
	body := map[string]any{
		"name":           m.Name.ValueString(),
		"user":           m.User.ValueInt64(),
		"egg":            m.EggID.ValueInt64(),
		"limits":         serverLimits(m),
		"feature_limits": serverFeatureLimits(m),
		"allocation":     map[string]any{"default": m.AllocationID.ValueInt64()},
		"environment":    envMap(ctx, m.Environment, diags),
	}
	putString(body, "description", m.Description)
	putString(body, "external_id", m.ExternalID)
	putString(body, "docker_image", m.DockerImage)
	putString(body, "startup", m.Startup)
	putBool(body, "oom_killer", m.OomKiller)
	putBool(body, "skip_scripts", m.SkipScripts)
	putBool(body, "start_on_completion", m.StartOnCompletion)
	return body
}

func buildServerDetailsBody(m serverModel) map[string]any {
	body := map[string]any{
		"name": m.Name.ValueString(),
		"user": m.User.ValueInt64(),
	}
	putString(body, "external_id", m.ExternalID)
	putString(body, "description", m.Description)
	return body
}

func buildServerBuildBody(m serverModel) map[string]any {
	body := map[string]any{
		"allocation":     m.AllocationID.ValueInt64(),
		"limits":         serverLimits(m),
		"feature_limits": serverFeatureLimits(m),
	}
	putBool(body, "oom_killer", m.OomKiller)
	return body
}

func buildServerStartupBody(ctx context.Context, m serverModel, diags *diag.Diagnostics) map[string]any {
	// The /startup endpoint requires skip_scripts to be present, so always send
	// it (defaulting to false when the operator left it unset).
	skipScripts := false
	if !m.SkipScripts.IsNull() && !m.SkipScripts.IsUnknown() {
		skipScripts = m.SkipScripts.ValueBool()
	}
	body := map[string]any{
		"egg":          m.EggID.ValueInt64(),
		"environment":  envMap(ctx, m.Environment, diags),
		"skip_scripts": skipScripts,
	}
	putString(body, "startup", m.Startup)
	putString(body, "image", m.DockerImage)
	return body
}

func serverLimits(m serverModel) map[string]any {
	limits := map[string]any{
		"memory": m.LimitMemory.ValueInt64(),
		"swap":   m.LimitSwap.ValueInt64(),
		"disk":   m.LimitDisk.ValueInt64(),
		"io":     m.LimitIO.ValueInt64(),
		"cpu":    m.LimitCPU.ValueInt64(),
	}
	if !m.LimitThreads.IsNull() && !m.LimitThreads.IsUnknown() {
		limits["threads"] = m.LimitThreads.ValueString()
	}
	return limits
}

func serverFeatureLimits(m serverModel) map[string]any {
	return map[string]any{
		"databases":   m.FeatureDatabases.ValueInt64(),
		"allocations": m.FeatureAllocs.ValueInt64(),
		"backups":     m.FeatureBackups.ValueInt64(),
	}
}

// applyServer maps the read envelope onto the model. It deliberately leaves the
// write-only fields (skip_scripts, start_on_completion) untouched, and only sets
// environment when readEnv is true — the resource keeps the operator's declared
// environment to avoid churn against egg-injected defaults, while the data source
// exposes the full effective environment.
func applyServer(ctx context.Context, item *client.Item, m *serverModel, diags *diag.Diagnostics, readEnv bool) {
	var a serverAttrs
	if err := item.Decode(&a); err != nil {
		diags.AddError("Unable to decode server", err.Error())
		return
	}
	m.ID = types.Int64Value(a.ID)
	m.ExternalID = stringPtr(a.ExternalID)
	m.UUID = types.StringValue(a.UUID)
	m.Identifier = types.StringValue(a.Identifier)
	m.Name = types.StringValue(a.Name)
	m.Description = stringPtr(a.Description)
	m.User = types.Int64Value(a.User)
	m.EggID = types.Int64Value(a.Egg)
	m.NodeID = types.Int64Value(a.Node)
	m.AllocationID = types.Int64Value(a.Alloc)
	m.DockerImage = types.StringValue(a.Container.Image)
	m.Startup = types.StringValue(a.Container.StartupCommand)
	m.OomKiller = types.BoolValue(a.Limits.OomKiller)
	m.Status = stringPtr(a.Status)
	m.Suspended = types.BoolValue(a.Suspended)
	m.LimitMemory = types.Int64Value(a.Limits.Memory)
	m.LimitSwap = types.Int64Value(a.Limits.Swap)
	m.LimitDisk = types.Int64Value(a.Limits.Disk)
	m.LimitIO = types.Int64Value(a.Limits.IO)
	m.LimitCPU = types.Int64Value(a.Limits.CPU)
	m.LimitThreads = stringPtr(a.Limits.Threads)
	m.FeatureDatabases = types.Int64Value(a.FeatureLimits.Databases)
	m.FeatureAllocs = types.Int64Value(a.FeatureLimits.Allocations)
	m.FeatureBackups = types.Int64Value(a.FeatureLimits.Backups)
	m.CreatedAt = stringPtr(a.CreatedAt)
	m.UpdatedAt = stringPtr(a.UpdatedAt)
	if readEnv {
		m.Environment = mapFromStringMap(ctx, stringifyAnyMap(a.Container.Environment), diags)
	}
}
