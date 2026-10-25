package modelasset

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccessorRejectsCountsBeforeAllocation(t *testing.T) {
	for _, count := range []int{math.MaxInt, -1, 0, 100} {
		if err := validateAccessor(count, 12, 12, 0, 36); err == nil {
			t.Fatalf("accepted count %d", count)
		}
	}
	if err := validateAccessor(3, 12, 12, 0, 36); err != nil {
		t.Fatal(err)
	}
}

func TestBoundedGeometryAndCancellation(t *testing.T) {
	if _, err := readBoundedGeometry(strings.NewReader(strings.Repeat("x", 33)), 32); err == nil {
		t.Fatal("unbounded geometry")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, read := range []func(context.Context, string) ([]Triangle, error){ReadGLBTrianglesContext, ReadGLTFTrianglesContext, Read3MFTrianglesContext} {
		if _, err := read(ctx, "nonexistent"); !errors.Is(err, context.Canceled) {
			t.Fatalf("ignored cancellation: %v", err)
		}
	}
}

func FuzzGLBContainer(f *testing.F) {
	data := []byte(`{"asset":{"version":"2.0"},"meshes":[]}`)
	for len(data)%4 != 0 {
		data = append(data, ' ')
	}
	raw := make([]byte, 20+len(data))
	binary.LittleEndian.PutUint32(raw, glbMagic)
	binary.LittleEndian.PutUint32(raw[4:], glbVersion)
	binary.LittleEndian.PutUint32(raw[8:], uint32(len(raw)))
	binary.LittleEndian.PutUint32(raw[12:], uint32(len(data)))
	binary.LittleEndian.PutUint32(raw[16:], glbJSONChunk)
	copy(raw[20:], data)
	f.Add(raw)
	f.Add([]byte("glTF"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<20 {
			return
		}
		_, _, _, _ = parseGLB(raw)
	})
}

func Fuzz3MFModel(f *testing.F) {
	f.Add(`<model><vertices><vertex x="0" y="0" z="0"/><vertex x="1" y="0" z="0"/><vertex x="0" y="1" z="0"/></vertices><triangles><triangle v1="0" v2="1" v3="2"/></triangles></model>`)
	f.Add("<model><resources/></model>")
	f.Add("<model><vertices><vertex x=\"nan\"/></vertices></model>")
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 1<<16 {
			return
		}
		_, _ = parse3MFModelTriangles(strings.NewReader(raw))
	})
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func Test3MFRejectsCompressedOversizedModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.3mf")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("3D/3dmodel.model")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(entry, zeros{}, maxGeometryBytes+1); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Read3MFTriangles(path); err == nil {
		t.Fatal("accepted an oversized compressed model")
	}
}
