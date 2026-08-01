package exportjob

import (
	"bytes"
	"encoding/binary"
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

func TestArtifactBuilderAssemblesSTLFromOBJPlacement(t *testing.T) {
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
	}, "stl")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if result.Kind != "assembled_stl" {
		t.Fatalf("artifact kind = %q", result.Kind)
	}
	if result.ContentType != "model/stl" {
		t.Fatalf("content type = %q", result.ContentType)
	}
	if result.URL != "/api/exports/job-1/artifact" {
		t.Fatalf("artifact url = %q", result.URL)
	}
	if len(result.IncludedAssets) != 1 || result.IncludedAssets[0] != "asset-1" {
		t.Fatalf("included assets = %v", result.IncludedAssets)
	}

	raw, err := os.ReadFile(result.StoragePath)
	if err != nil {
		t.Fatalf("read assembled STL: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "solid taverna_job-1") {
		t.Fatalf("assembled STL missing solid header:\n%s", text)
	}
	if !strings.Contains(text, "vertex 1.000000 2.000000 3.000000") {
		t.Fatalf("assembled STL missing translated origin vertex:\n%s", text)
	}
	if !strings.Contains(text, "vertex 3.000000 2.000000 3.000000") {
		t.Fatalf("assembled STL missing scaled x vertex:\n%s", text)
	}
	if !strings.Contains(text, "facet normal 0.000000 0.000000 1.000000") {
		t.Fatalf("assembled STL missing recalculated normal:\n%s", text)
	}
}

func TestArtifactBuilderAssemblesSTLPlacement(t *testing.T) {
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "blade.stl")
	source := `solid blade
facet normal 0 0 1
  outer loop
    vertex 0 0 0
    vertex 0 1 0
    vertex 0 0 1
  endloop
endfacet
endsolid blade
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatalf("write source STL: %v", err)
	}

	builder := NewArtifactBuilder(t.TempDir())
	result, err := builder.Build("job-1", assembly.Manifest{
		ProjectID: "project-1",
		Placements: []assembly.ResolvedPlacement{{
			ID: "placement-1",
			Asset: assembly.ModelAssetRef{
				ID:          "asset-1",
				Label:       "Blade",
				FileName:    "blade.stl",
				Format:      modelasset.FormatSTL,
				StoragePath: sourcePath,
			},
			WorldTransform: assembly.Transform{
				Position: [3]float64{0.5, -1, 2},
				Rotation: [3]float64{0, 0, 0},
				Scale:    2,
			},
		}},
	}, "stl")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(result.IncludedAssets) != 1 || result.IncludedAssets[0] != "asset-1" {
		t.Fatalf("included assets = %v", result.IncludedAssets)
	}

	raw, err := os.ReadFile(result.StoragePath)
	if err != nil {
		t.Fatalf("read assembled STL: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "vertex 0.500000 -1.000000 2.000000") {
		t.Fatalf("assembled STL missing transformed origin vertex:\n%s", text)
	}
	if !strings.Contains(text, "vertex 0.500000 1.000000 2.000000") {
		t.Fatalf("assembled STL missing transformed y vertex:\n%s", text)
	}
}

func TestArtifactBuilderAssemblesBinarySTLPlacement(t *testing.T) {
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "blade-binary.stl")
	writeBinarySTL(t, sourcePath, [][3][3]float32{
		{
			{0, 0, 0},
			{0, 1, 0},
			{0, 0, 1},
		},
	})

	builder := NewArtifactBuilder(t.TempDir())
	result, err := builder.Build("job-1", assembly.Manifest{
		ProjectID: "project-1",
		Placements: []assembly.ResolvedPlacement{{
			ID: "placement-1",
			Asset: assembly.ModelAssetRef{
				ID:          "asset-1",
				Label:       "Blade",
				FileName:    "blade-binary.stl",
				Format:      modelasset.FormatSTL,
				StoragePath: sourcePath,
			},
			WorldTransform: assembly.Transform{
				Position: [3]float64{0.5, -1, 2},
				Rotation: [3]float64{0, 0, 0},
				Scale:    2,
			},
		}},
	}, "stl")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(result.IncludedAssets) != 1 || result.IncludedAssets[0] != "asset-1" {
		t.Fatalf("included assets = %v", result.IncludedAssets)
	}

	raw, err := os.ReadFile(result.StoragePath)
	if err != nil {
		t.Fatalf("read assembled STL: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "vertex 0.500000 -1.000000 2.000000") {
		t.Fatalf("assembled STL missing transformed binary origin vertex:\n%s", text)
	}
	if !strings.Contains(text, "vertex 0.500000 1.000000 2.000000") {
		t.Fatalf("assembled STL missing transformed binary y vertex:\n%s", text)
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

func writeBinarySTL(t *testing.T, path string, triangles [][3][3]float32) {
	t.Helper()

	var buffer bytes.Buffer
	header := make([]byte, 80)
	copy(header, "Taverna binary STL test")
	if _, err := buffer.Write(header); err != nil {
		t.Fatalf("write binary STL header: %v", err)
	}
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(triangles))); err != nil {
		t.Fatalf("write binary STL triangle count: %v", err)
	}
	for _, triangle := range triangles {
		values := [12]float32{
			0, 0, 1,
			triangle[0][0], triangle[0][1], triangle[0][2],
			triangle[1][0], triangle[1][1], triangle[1][2],
			triangle[2][0], triangle[2][1], triangle[2][2],
		}
		if err := binary.Write(&buffer, binary.LittleEndian, values); err != nil {
			t.Fatalf("write binary STL triangle: %v", err)
		}
		if err := binary.Write(&buffer, binary.LittleEndian, uint16(0)); err != nil {
			t.Fatalf("write binary STL attribute: %v", err)
		}
	}

	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatalf("write binary STL: %v", err)
	}
}
