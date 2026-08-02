package modelasset

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestRead3MFTriangles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blade.3mf")
	writeSimple3MF(t, path)

	triangles, err := Read3MFTriangles(path)
	if err != nil {
		t.Fatalf("Read3MFTriangles() error = %v", err)
	}
	if len(triangles) != 1 {
		t.Fatalf("triangle count = %d", len(triangles))
	}
	if triangles[0][0] != (Vector3{0, 0, 0}) {
		t.Fatalf("triangle first vertex = %v", triangles[0][0])
	}
	if triangles[0][1] != (Vector3{1, 0, 0}) {
		t.Fatalf("triangle second vertex = %v", triangles[0][1])
	}
	if triangles[0][2] != (Vector3{0, 1, 0}) {
		t.Fatalf("triangle third vertex = %v", triangles[0][2])
	}
}

func TestInspectGeometryParses3MFBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blade.3mf")
	writeSimple3MF(t, path)

	metadata := InspectGeometry(path, Format3MF)
	if metadata.Status != "parsed" {
		t.Fatalf("status = %q, warnings = %v", metadata.Status, metadata.Warnings)
	}
	if metadata.Source != "3mf" {
		t.Fatalf("source = %q", metadata.Source)
	}
	if metadata.VertexCount != 3 || metadata.FaceCount != 1 || metadata.TriangleCount != 1 {
		t.Fatalf("counts = vertices %d faces %d triangles %d", metadata.VertexCount, metadata.FaceCount, metadata.TriangleCount)
	}
	if metadata.Bounds == nil {
		t.Fatal("expected bounds")
	}
	if metadata.Bounds.Min != (Vector3{0, 0, 0}) {
		t.Fatalf("bounds min = %v", metadata.Bounds.Min)
	}
	if metadata.Bounds.Size != (Vector3{1, 1, 0}) {
		t.Fatalf("bounds size = %v", metadata.Bounds.Size)
	}
	if metadata.Bounds.Center != (Vector3{0.5, 0.5, 0}) {
		t.Fatalf("bounds center = %v", metadata.Bounds.Center)
	}
}

func writeSimple3MF(t *testing.T, path string) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create 3MF: %v", err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("3D/3dmodel.model")
	if err != nil {
		t.Fatalf("create 3MF model entry: %v", err)
	}
	if _, err := entry.Write([]byte(simple3MFModelXML)); err != nil {
		t.Fatalf("write 3MF model XML: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close 3MF archive: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close 3MF file: %v", err)
	}
}

const simple3MFModelXML = `<?xml version="1.0" encoding="UTF-8"?>
<model unit="millimeter" xml:lang="en-US" xmlns="http://schemas.microsoft.com/3dmanufacturing/core/2015/02">
  <resources>
    <object id="1" type="model">
      <mesh>
        <vertices>
          <vertex x="0" y="0" z="0"/>
          <vertex x="1" y="0" z="0"/>
          <vertex x="0" y="1" z="0"/>
        </vertices>
        <triangles>
          <triangle v1="0" v2="1" v3="2"/>
        </triangles>
      </mesh>
    </object>
  </resources>
  <build>
    <item objectid="1"/>
  </build>
</model>`
