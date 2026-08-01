package exportjob

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taverna/backend/internal/assembly"
	"taverna/backend/internal/modelasset"
)

func TestArtifactBuilderAssemblesOBJPlacement(t *testing.T) {
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "blade.obj")
	if err := os.WriteFile(sourcePath, []byte("o blade\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o600); err != nil {
		t.Fatalf("write source OBJ: %v", err)
	}

	builder := NewArtifactBuilder(t.TempDir())
	result, err := builder.Build("job-1", assembly.Manifest{
		ProjectID: "project-1",
		Placements: []assembly.ResolvedPlacement{{
			ID: "placement-1",
			Asset: assembly.ModelAssetRef{
				ID:          "asset-1",
				Label:       "Blade",
				FileName:    "blade.obj",
				Format:      modelasset.FormatOBJ,
				StoragePath: sourcePath,
			},
			WorldTransform: assembly.Transform{
				Position: [3]float64{1, 2, 3},
				Rotation: [3]float64{0, 0, 0},
				Scale:    2,
			},
		}},
	}, "obj")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if result.URL != "/api/exports/job-1/artifact" {
		t.Fatalf("artifact url = %q", result.URL)
	}
	if len(result.IncludedAssets) != 1 || result.IncludedAssets[0] != "asset-1" {
		t.Fatalf("included assets = %v", result.IncludedAssets)
	}

	raw, err := os.ReadFile(result.StoragePath)
	if err != nil {
		t.Fatalf("read assembled OBJ: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "v 1.000000 2.000000 3.000000") {
		t.Fatalf("assembled OBJ missing translated origin vertex:\n%s", text)
	}
	if !strings.Contains(text, "v 3.000000 2.000000 3.000000") {
		t.Fatalf("assembled OBJ missing scaled x vertex:\n%s", text)
	}
}

func TestArtifactBuilderSkipsNonOBJAssets(t *testing.T) {
	builder := NewArtifactBuilder(t.TempDir())
	result, err := builder.Build("job-1", assembly.Manifest{
		ProjectID: "project-1",
		Placements: []assembly.ResolvedPlacement{{
			ID: "placement-1",
			Asset: assembly.ModelAssetRef{
				ID:     "asset-1",
				Format: modelasset.FormatGLB,
			},
		}},
	}, "obj")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(result.IncludedAssets) != 0 {
		t.Fatalf("included assets = %v", result.IncludedAssets)
	}
	if len(result.SkippedAssets) != 1 || result.SkippedAssets[0] != "asset-1" {
		t.Fatalf("skipped assets = %v", result.SkippedAssets)
	}
}
