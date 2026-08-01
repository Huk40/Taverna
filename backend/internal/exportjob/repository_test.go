package exportjob

import (
	"encoding/json"
	"testing"
)

func TestNormalizeExportDefaults(t *testing.T) {
	tests := []struct {
		name       string
		request    CreateRequest
		wantTarget string
		wantFormat string
	}{
		{
			name:       "empty request defaults to STL print",
			request:    CreateRequest{},
			wantTarget: "stl_print",
			wantFormat: "stl",
		},
		{
			name:       "GLB preview defaults to GLB format",
			request:    CreateRequest{Target: "glb_preview"},
			wantTarget: "glb_preview",
			wantFormat: "glb",
		},
		{
			name:       "explicit format is preserved",
			request:    CreateRequest{Target: "physical_print", Format: "3mf"},
			wantTarget: "physical_print",
			wantFormat: "3mf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := normalizeTarget(tt.request.Target)
			format := normalizeFormat(tt.request.Format, target)
			if target != tt.wantTarget {
				t.Fatalf("target = %q, want %q", target, tt.wantTarget)
			}
			if format != tt.wantFormat {
				t.Fatalf("format = %q, want %q", format, tt.wantFormat)
			}
		})
	}
}

func TestAllowedExportValues(t *testing.T) {
	for _, target := range []string{"stl_print", "glb_preview", "physical_print"} {
		if !allowedTarget(target) {
			t.Fatalf("target %q should be allowed", target)
		}
	}
	if allowedTarget("invoice_pdf") {
		t.Fatal("invoice_pdf target should not be allowed")
	}

	for _, format := range []string{"stl", "glb", "3mf"} {
		if !allowedFormat(format) {
			t.Fatalf("format %q should be allowed", format)
		}
	}
	if allowedFormat("fbx") {
		t.Fatal("fbx format should not be allowed")
	}
}

func TestBuildFakeArtifactIncludesPrintReadiness(t *testing.T) {
	raw, err := buildFakeArtifact("project-1", "stl_print", "stl", map[string]any{"placements": []any{}})
	if err != nil {
		t.Fatalf("buildFakeArtifact() error = %v", err)
	}

	var artifact map[string]any
	if err := json.Unmarshal(raw, &artifact); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if artifact["source_project_id"] != "project-1" {
		t.Fatalf("source_project_id = %v", artifact["source_project_id"])
	}
	if artifact["target"] != "stl_print" || artifact["format"] != "stl" {
		t.Fatalf("unexpected artifact target/format: %v", artifact)
	}
	if _, ok := artifact["print_readiness"].(map[string]any); !ok {
		t.Fatalf("print_readiness missing from artifact: %v", artifact)
	}
}

func TestBuildFakeArtifactIncludesAssemblySummary(t *testing.T) {
	manifest := map[string]any{
		"schema_version": 1,
		"placements": []map[string]any{
			{
				"slot":             "gear",
				"attachment_point": "right_hand",
				"asset": map[string]any{
					"id": "asset-1",
				},
			},
		},
	}

	raw, err := buildFakeArtifact("project-1", "glb_preview", "glb", manifest)
	if err != nil {
		t.Fatalf("buildFakeArtifact() error = %v", err)
	}

	var artifact map[string]any
	if err := json.Unmarshal(raw, &artifact); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	assembly, ok := artifact["assembly"].(map[string]any)
	if !ok {
		t.Fatalf("assembly summary missing from artifact: %v", artifact)
	}
	if assembly["status"] != "assembly_plan_ready" {
		t.Fatalf("assembly status = %v", assembly["status"])
	}
	if assembly["placement_count"].(float64) != 1 {
		t.Fatalf("placement_count = %v", assembly["placement_count"])
	}
	if _, ok := artifact["assembly_manifest"].(map[string]any); !ok {
		t.Fatalf("assembly_manifest missing from artifact: %v", artifact)
	}
}
