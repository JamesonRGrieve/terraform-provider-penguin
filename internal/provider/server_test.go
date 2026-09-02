// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/zephyrex/terraform-provider-penguin/internal/client"
)

func sampleServerModel() serverModel {
	return serverModel{
		Name:             strVal("survival"),
		User:             intVal(1),
		EggID:            intVal(3),
		AllocationID:     intVal(7),
		LimitMemory:      intVal(4096),
		LimitSwap:        intVal(-1),
		LimitDisk:        intVal(10240),
		LimitIO:          intVal(500),
		LimitCPU:         intVal(200),
		LimitThreads:     nullStr(),
		FeatureDatabases: intVal(2),
		FeatureAllocs:    intVal(0),
		FeatureBackups:   intVal(5),
		Environment:      nullMap(),
		SkipScripts:      nullBool(),
		OomKiller:        boolVal(false),
	}
}

func TestBuildServerCreateBodyStructure(t *testing.T) {
	t.Parallel()
	body := buildServerCreateBody(context.Background(), sampleServerModel(), newDiags())

	if body["name"] != "survival" || body["user"] != int64(1) || body["egg"] != int64(3) {
		t.Fatalf("top-level fields wrong: %v", body)
	}

	limits, ok := body["limits"].(map[string]any)
	if !ok {
		t.Fatalf("limits not a map: %T", body["limits"])
	}
	for _, k := range []string{"memory", "swap", "disk", "io", "cpu"} {
		if _, ok := limits[k]; !ok {
			t.Errorf("limits missing %q", k)
		}
	}
	if limits["memory"] != int64(4096) {
		t.Errorf("limits.memory = %v", limits["memory"])
	}
	if _, ok := limits["threads"]; ok {
		t.Error("threads should be omitted when null")
	}

	feature, ok := body["feature_limits"].(map[string]any)
	if !ok {
		t.Fatalf("feature_limits not a map: %T", body["feature_limits"])
	}
	for _, k := range []string{"databases", "allocations", "backups"} {
		if _, ok := feature[k]; !ok {
			t.Errorf("feature_limits missing %q (must be present)", k)
		}
	}

	alloc, ok := body["allocation"].(map[string]any)
	if !ok || alloc["default"] != int64(7) {
		t.Fatalf("allocation.default wrong: %v", body["allocation"])
	}

	if _, ok := body["environment"].(map[string]string); !ok {
		t.Errorf("environment must be present (empty map when unset): %T", body["environment"])
	}
}

func TestBuildServerStartupBodyAlwaysHasSkipScripts(t *testing.T) {
	t.Parallel()
	body := buildServerStartupBody(context.Background(), sampleServerModel(), newDiags())

	skip, ok := body["skip_scripts"]
	if !ok {
		t.Fatal("skip_scripts must be present in the /startup body")
	}
	if skip != false {
		t.Errorf("skip_scripts = %v, want false default", skip)
	}
	if _, ok := body["environment"].(map[string]string); !ok {
		t.Error("environment must be present in the /startup body")
	}
	if body["egg"] != int64(3) {
		t.Errorf("egg = %v", body["egg"])
	}
}

func TestApplyServerReadEnv(t *testing.T) {
	t.Parallel()
	item := &client.Item{
		Object: "server",
		Attributes: map[string]any{
			"id": float64(12), "uuid": "u", "identifier": "abc123", "name": "survival",
			"description": nil, "status": nil, "suspended": false,
			"limits": map[string]any{
				"memory": float64(4096), "swap": float64(-1), "disk": float64(10240),
				"io": float64(500), "cpu": float64(200), "threads": nil, "oom_killer": false,
			},
			"feature_limits": map[string]any{"databases": float64(2), "allocations": float64(0), "backups": float64(5)},
			"user":           float64(1), "node": float64(9), "allocation": float64(7), "egg": float64(3),
			"container": map[string]any{
				"startup_command": "java -jar server.jar",
				"image":           "ghcr.io/pelican-eggs/java:21",
				"environment":     map[string]any{"SERVER_JARFILE": "server.jar", "MAX_PLAYERS": float64(20)},
			},
			"created_at": "2026-01-01T00:00:00+00:00", "updated_at": "2026-01-02T00:00:00+00:00",
		},
	}

	// Resource path: environment is NOT read back (readEnv=false).
	var res serverModel
	applyServer(context.Background(), item, &res, newDiags(), false)
	if res.NodeID.ValueInt64() != 9 || res.AllocationID.ValueInt64() != 7 || res.EggID.ValueInt64() != 3 {
		t.Errorf("relations wrong: node=%d alloc=%d egg=%d", res.NodeID.ValueInt64(), res.AllocationID.ValueInt64(), res.EggID.ValueInt64())
	}
	if res.LimitSwap.ValueInt64() != -1 {
		t.Errorf("limit_swap = %d, want -1", res.LimitSwap.ValueInt64())
	}
	if !res.Environment.IsNull() {
		t.Error("environment must be left untouched (null) on the resource read")
	}

	// Data-source path: environment IS read back and stringified.
	var ds serverModel
	applyServer(context.Background(), item, &ds, newDiags(), true)
	env := map[string]types.String{}
	ds.Environment.ElementsAs(context.Background(), &env, false)
	if env["MAX_PLAYERS"].ValueString() != "20" {
		t.Errorf("env MAX_PLAYERS = %q, want \"20\" (stringified)", env["MAX_PLAYERS"].ValueString())
	}
	if env["SERVER_JARFILE"].ValueString() != "server.jar" {
		t.Errorf("env SERVER_JARFILE = %q", env["SERVER_JARFILE"].ValueString())
	}
}
