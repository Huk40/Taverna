package modelasset

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestReadGLBTrianglesReadsUnindexedTriangle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "triangle.glb")
	writeInspectionGLB(t, path, []float32{
		0, 0, 0,
		1, 0, 0,
		0, 1, 0,
	})

	triangles, err := ReadGLBTriangles(path)
	if err != nil {
		t.Fatalf("ReadGLBTriangles() error = %v", err)
	}
	if len(triangles) != 1 {
		t.Fatalf("triangle count = %d", len(triangles))
	}
	if triangles[0][1] != (Vector3{1, 0, 0}) {
		t.Fatalf("second vertex = %v", triangles[0][1])
	}
}

func TestReadGLBTrianglesReadsIndexedTriangle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "indexed.glb")
	writeIndexedTriangleGLB(t, path)

	triangles, err := ReadGLBTriangles(path)
	if err != nil {
		t.Fatalf("ReadGLBTriangles() error = %v", err)
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

func writeIndexedTriangleGLB(t *testing.T, path string) {
	t.Helper()

	var positionBuffer bytes.Buffer
	for _, value := range []float32{
		0, 0, 0,
		1, 0, 0,
		0, 1, 0,
	} {
		if err := binary.Write(&positionBuffer, binary.LittleEndian, value); err != nil {
			t.Fatalf("write GLB position: %v", err)
		}
	}

	var indexBuffer bytes.Buffer
	for _, value := range []uint16{2, 0, 1} {
		if err := binary.Write(&indexBuffer, binary.LittleEndian, value); err != nil {
			t.Fatalf("write GLB index: %v", err)
		}
	}

	positions := positionBuffer.Bytes()
	indicesOffset := len(positions)
	bin := append([]byte{}, positions...)
	bin = append(bin, indexBuffer.Bytes()...)
	unpaddedBinLength := len(bin)
	bin = padGLBChunk(bin, 0)

	jsonChunk := padGLBChunk([]byte(fmt.Sprintf(`{
		"asset":{"version":"2.0"},
		"buffers":[{"byteLength":%d}],
		"bufferViews":[
			{"buffer":0,"byteOffset":0,"byteLength":%d},
			{"buffer":0,"byteOffset":%d,"byteLength":%d}
		],
		"accessors":[
			{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"},
			{"bufferView":1,"componentType":5123,"count":3,"type":"SCALAR"}
		],
		"meshes":[{"primitives":[{"attributes":{"POSITION":0},"indices":1,"mode":4}]}]
	}`, unpaddedBinLength, len(positions), indicesOffset, indexBuffer.Len())), ' ')

	totalLength := 12 + 8 + len(jsonChunk) + 8 + len(bin)
	var buffer bytes.Buffer
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(glbMagic)); err != nil {
		t.Fatalf("write GLB magic: %v", err)
	}
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(glbVersion)); err != nil {
		t.Fatalf("write GLB version: %v", err)
	}
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(totalLength)); err != nil {
		t.Fatalf("write GLB length: %v", err)
	}
	writeGLBChunk(t, &buffer, jsonChunk, glbJSONChunk)
	writeGLBChunk(t, &buffer, bin, glbBINChunk)

	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatalf("write indexed GLB: %v", err)
	}
}
