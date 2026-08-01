package modelasset

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectGeometryParsesOBJBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blade.obj")
	if err := os.WriteFile(path, []byte(`
v -1 0 2
v 2 4 -2
v 0 1 0
f 1 2 3
f 1 2 3 1
`), 0o600); err != nil {
		t.Fatalf("write OBJ: %v", err)
	}

	metadata := InspectGeometry(path, FormatOBJ)
	if metadata.Status != "parsed" {
		t.Fatalf("status = %q, warnings = %v", metadata.Status, metadata.Warnings)
	}
	if metadata.Source != "obj" {
		t.Fatalf("source = %q", metadata.Source)
	}
	if metadata.VertexCount != 3 || metadata.FaceCount != 2 || metadata.TriangleCount != 3 {
		t.Fatalf("counts = vertices %d faces %d triangles %d", metadata.VertexCount, metadata.FaceCount, metadata.TriangleCount)
	}
	if metadata.Bounds == nil {
		t.Fatal("expected bounds")
	}
	if metadata.Bounds.Min != (Vector3{-1, 0, -2}) {
		t.Fatalf("bounds min = %v", metadata.Bounds.Min)
	}
	if metadata.Bounds.Size != (Vector3{3, 4, 4}) {
		t.Fatalf("bounds size = %v", metadata.Bounds.Size)
	}
	if metadata.Bounds.Center != (Vector3{0.5, 2, 0}) {
		t.Fatalf("bounds center = %v", metadata.Bounds.Center)
	}
}

func TestInspectGeometryParsesASCIISTL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blade.stl")
	if err := os.WriteFile(path, []byte(`
solid blade
  facet normal 0 0 1
    outer loop
      vertex 0 0 0
      vertex 0 1 0
      vertex 0 0 1
    endloop
  endfacet
endsolid blade
`), 0o600); err != nil {
		t.Fatalf("write STL: %v", err)
	}

	metadata := InspectGeometry(path, FormatSTL)
	if metadata.Status != "parsed" {
		t.Fatalf("status = %q, warnings = %v", metadata.Status, metadata.Warnings)
	}
	if metadata.Source != "ascii_stl" {
		t.Fatalf("source = %q", metadata.Source)
	}
	if metadata.VertexCount != 3 || metadata.FaceCount != 1 || metadata.TriangleCount != 1 {
		t.Fatalf("counts = vertices %d faces %d triangles %d", metadata.VertexCount, metadata.FaceCount, metadata.TriangleCount)
	}
	if metadata.Bounds == nil || metadata.Bounds.Size != (Vector3{0, 1, 1}) {
		t.Fatalf("bounds = %#v", metadata.Bounds)
	}
}

func TestInspectGeometryParsesBinarySTL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blade-binary.stl")
	writeInspectionBinarySTL(t, path, [][3][3]float32{
		{
			{-1, 0, 0},
			{0, 2, 0},
			{0, 0, 3},
		},
	})

	metadata := InspectGeometry(path, FormatSTL)
	if metadata.Status != "parsed" {
		t.Fatalf("status = %q, warnings = %v", metadata.Status, metadata.Warnings)
	}
	if metadata.Source != "binary_stl" {
		t.Fatalf("source = %q", metadata.Source)
	}
	if metadata.VertexCount != 3 || metadata.FaceCount != 1 || metadata.TriangleCount != 1 {
		t.Fatalf("counts = vertices %d faces %d triangles %d", metadata.VertexCount, metadata.FaceCount, metadata.TriangleCount)
	}
	if metadata.Bounds == nil || metadata.Bounds.Min != (Vector3{-1, 0, 0}) || metadata.Bounds.Max != (Vector3{0, 2, 3}) {
		t.Fatalf("bounds = %#v", metadata.Bounds)
	}
}

func TestInspectGeometryMarksUnsupportedFormatsStored(t *testing.T) {
	metadata := InspectGeometry("unused.glb", FormatGLB)
	if metadata.Status != "stored" {
		t.Fatalf("status = %q", metadata.Status)
	}
	if len(metadata.Warnings) == 0 {
		t.Fatal("expected warning for unsupported geometry metadata parser")
	}
}

func writeInspectionBinarySTL(t *testing.T, path string, triangles [][3][3]float32) {
	t.Helper()

	var buffer bytes.Buffer
	header := make([]byte, 80)
	copy(header, "Taverna geometry metadata binary STL test")
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
