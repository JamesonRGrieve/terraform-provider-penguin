// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

func newProvider(t *testing.T) *penguinProvider {
	t.Helper()
	p, ok := New("test")().(*penguinProvider)
	if !ok {
		t.Fatal("New did not return *penguinProvider")
	}
	return p
}

func TestProviderMetadata(t *testing.T) {
	t.Parallel()
	var resp fwprovider.MetadataResponse
	newProvider(t).Metadata(context.Background(), fwprovider.MetadataRequest{}, &resp)
	if resp.TypeName != "penguin" {
		t.Fatalf("TypeName = %q, want penguin", resp.TypeName)
	}
}

func TestProviderSchemaValid(t *testing.T) {
	t.Parallel()
	var resp fwprovider.SchemaResponse
	newProvider(t).Schema(context.Background(), fwprovider.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("provider schema diagnostics: %v", resp.Diagnostics)
	}
	for _, want := range []string{"endpoint", "api_token", "insecure"} {
		if _, ok := resp.Schema.Attributes[want]; !ok {
			t.Errorf("provider schema missing attribute %q", want)
		}
	}
}

func TestAllResourceSchemas(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := newProvider(t)
	factories := p.Resources(ctx)
	if len(factories) == 0 {
		t.Fatal("no resources registered")
	}
	for _, factory := range factories {
		r := factory()
		var meta resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "penguin"}, &meta)
		t.Run(meta.TypeName, func(t *testing.T) {
			t.Parallel()
			var resp resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("%s schema diagnostics: %v", meta.TypeName, resp.Diagnostics)
			}
			if len(resp.Schema.Attributes) == 0 {
				t.Fatalf("%s schema has no attributes", meta.TypeName)
			}
		})
	}
}

func TestAllDataSourceSchemas(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := newProvider(t)
	factories := p.DataSources(ctx)
	if len(factories) == 0 {
		t.Fatal("no data sources registered")
	}
	for _, factory := range factories {
		d := factory()
		var meta datasource.MetadataResponse
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "penguin"}, &meta)
		t.Run(meta.TypeName, func(t *testing.T) {
			t.Parallel()
			var resp datasource.SchemaResponse
			d.Schema(ctx, datasource.SchemaRequest{}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("%s schema diagnostics: %v", meta.TypeName, resp.Diagnostics)
			}
			if len(resp.Schema.Attributes) == 0 {
				t.Fatalf("%s schema has no attributes", meta.TypeName)
			}
		})
	}
}

// TestProviderServerSchemaValidates runs the framework's own schema validation
// over every resource, data source, and the provider schema — the check that
// catches invalid attribute combinations a hand-rolled Schema() call does not.
func TestProviderServerSchemaValidates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv, err := providerserver.NewProtocol6WithError(New("test")())()
	if err != nil {
		t.Fatalf("build provider server: %v", err)
	}
	resp, err := srv.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema: %v", err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("schema error: %s — %s", d.Summary, d.Detail)
		}
	}
}

func TestApplyNodeMapping(t *testing.T) {
	t.Parallel()
	item := &client.Item{
		Object: "node",
		Attributes: map[string]any{
			"id": float64(7), "uuid": "abc", "name": "node-a",
			"description": nil, "public": true, "fqdn": "node-a.example.com",
			"scheme": "https", "behind_proxy": false,
			"memory": float64(8192), "memory_overallocate": float64(-1),
			"disk": float64(100000), "disk_overallocate": float64(0),
			"cpu": float64(400), "cpu_overallocate": float64(0),
			"daemon_base": "/var/lib/pelican/volumes", "daemon_sftp": float64(2022),
			"daemon_sftp_alias": nil, "daemon_listen": float64(8080),
			"daemon_connect": float64(8080), "maintenance_mode": false,
			"upload_size": float64(100), "tags": []any{"prod", "eu"},
			"created_at": "2026-01-01T00:00:00+00:00", "updated_at": "2026-01-02T00:00:00+00:00",
		},
	}
	var m nodeModel
	var diags = newDiags()
	applyNode(context.Background(), item, &m, diags)
	if diags.HasError() {
		t.Fatalf("applyNode diagnostics: %v", *diags)
	}
	if m.ID.ValueInt64() != 7 {
		t.Errorf("id = %d, want 7", m.ID.ValueInt64())
	}
	if m.Name.ValueString() != "node-a" {
		t.Errorf("name = %q", m.Name.ValueString())
	}
	if !m.Description.IsNull() {
		t.Errorf("description = %v, want null", m.Description)
	}
	if m.MemoryOverallocate.ValueInt64() != -1 {
		t.Errorf("memory_overallocate = %d, want -1", m.MemoryOverallocate.ValueInt64())
	}
	if l := m.Tags.Elements(); len(l) != 2 {
		t.Errorf("tags len = %d, want 2", len(l))
	}
}

func TestBuildNodeBodyOmitsNullOptionals(t *testing.T) {
	t.Parallel()
	m := nodeModel{
		Name:   strVal("node-a"),
		FQDN:   strVal("node-a.example.com"),
		Memory: intVal(8192),
		Disk:   intVal(100000),
		CPU:    intVal(400),
		// Description, DaemonSFTPAlias, Tags left null/unknown.
	}
	m.Description = nullStr()
	m.DaemonSFTPAlias = nullStr()
	m.Tags = nullList()
	body := buildNodeBody(context.Background(), m, newDiags())
	if _, ok := body["description"]; ok {
		t.Error("description should be omitted when null")
	}
	if _, ok := body["tags"]; ok {
		t.Error("tags should be omitted when null")
	}
	if body["name"] != "node-a" {
		t.Errorf("name = %v", body["name"])
	}
	if body["memory"] != int64(8192) {
		t.Errorf("memory = %v (%T)", body["memory"], body["memory"])
	}
}
