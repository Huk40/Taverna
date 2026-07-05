package exportjob

import "testing"

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
