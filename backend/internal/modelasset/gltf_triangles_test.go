package modelasset

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestReadGLTFTrianglesReadsEmbeddedIndexedTriangle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "indexed.gltf")
	writeEmbeddedTriangleGLTF(t, path, true)

	triangles, err := ReadGLTFTriangles(path)
	if err != nil {
		t.Fatalf("ReadGLTFTriangles() error = %v", err)
	}
	if len(triangles) != 1 {
		t.Fatalf("triangle count = %d", len(triangles))
	}
	if triangles[0][0] != (Vector3{0, 1, 0}) {
		t.Fatalf("first indexed vertex = %v", triangles[0][0])
	}
	if triangles[0][2] != (Vector3{1, 0, 0}) {
		t.Fatalf("third indexed vertex = %v", triangles[0][2])
	}
}

func TestInspectGeometryParsesEmbeddedGLTFBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "triangle.gltf")
	writeEmbeddedTriangleGLTF(t, path, false)

	metadata := InspectGeometry(path, FormatGLTF)
	if metadata.Status != "parsed" {
		t.Fatalf("status = %q, warnings = %v", metadata.Status, metadata.Warnings)
	}
	if metadata.Source != "gltf" {
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

func TestReadGLTFTrianglesRejectsExternalBuffer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "external.gltf")
	if err := os.WriteFile(path, []byte(`{
		"asset":{"version":"2.0"},
		"buffers":[{"uri":"triangle.bin","byteLength":36}],
		"bufferViews":[{"buffer":0,"byteOffset":0,"byteLength":36}],
		"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"}],
		"meshes":[{"primitives":[{"attributes":{"POSITION":0},"mode":4}]}]
	}`), 0o600); err != nil {
		t.Fatalf("write external GLTF: %v", err)
	}

	_, err := ReadGLTFTriangles(path)
	if err == nil {
		t.Fatal("expected external GLTF buffer to fail")
	}
	if err.Error() != `GLTF buffer 0 uses external URI "triangle.bin"; only embedded data URI buffers are supported` {
		t.Fatalf("error = %q", err.Error())
	}
}

func writeEmbeddedTriangleGLTF(t *testing.T, path string, indexed bool) {
	t.Helper()

	var positionBuffer bytes.Buffer
	for _, value := range []float32{
		0, 0, 0,
		1, 0, 0,
		0, 1, 0,
	} {
		if err := binary.Write(&positionBuffer, binary.LittleEndian, value); err != nil {
			t.Fatalf("write GLTF position: %v", err)
		}
	}

	bin := append([]byte{}, positionBuffer.Bytes()...)
	primitiveIndices := ""
	bufferViews := fmt.Sprintf(`{"buffer":0,"byteOffset":0,"byteLength":%d}`, positionBuffer.Len())
	accessors := `{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"}`
	if indexed {
		var indexBuffer bytes.Buffer
		for _, value := range []uint16{2, 0, 1} {
			if err := binary.Write(&indexBuffer, binary.LittleEndian, value); err != nil {
				t.Fatalf("write GLTF index: %v", err)
			}
		}
		indexOffset := len(bin)
		bin = append(bin, indexBuffer.Bytes()...)
		bufferViews = fmt.Sprintf(`%s,{"buffer":0,"byteOffset":%d,"byteLength":%d}`, bufferViews, indexOffset, indexBuffer.Len())
		accessors = fmt.Sprintf(`%s,{"bufferView":1,"componentType":5123,"count":3,"type":"SCALAR"}`, accessors)
		primitiveIndices = `,"indices":1`
	}

	encoded := base64.StdEncoding.EncodeToString(bin)
	raw := fmt.Sprintf(`{
		"asset":{"version":"2.0"},
		"buffers":[{"uri":"data:application/octet-stream;base64,%s","byteLength":%d}],
		"bufferViews":[%s],
		"accessors":[%s],
		"meshes":[{"primitives":[{"attributes":{"POSITION":0}%s,"mode":4}]}]
	}`, encoded, len(bin), bufferViews, accessors, primitiveIndices)

	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write embedded GLTF: %v", err)
	}
}
