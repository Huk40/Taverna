package exportjob

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taverna/backend/internal/assembly"
	"taverna/backend/internal/modelasset"
)

func printFixture(t *testing.T, source string) assembly.Manifest {
	t.Helper()
	path := filepath.Join(t.TempDir(), "part.obj")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	return assembly.Manifest{ScaleMM: 32, CharacterScale: 1, Placements: []assembly.ResolvedPlacement{{
		Slot: "body", Asset: assembly.ModelAssetRef{ID: "part", Format: modelasset.FormatOBJ, StoragePath: path},
		WorldTransform: assembly.Transform{Position: [3]float64{0, 1, 0}, Scale: 1},
	}}}
}

func TestPrintSTLUsesMillimetresZUpAndIncludesBase(t *testing.T) {
	manifest := printFixture(t, "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")
	for _, height := range []int{32, 40} {
		manifest.ScaleMM = height
		data, err := buildPrintSTL(context.Background(), manifest)
		if err != nil {
			t.Fatal(err)
		}
		count := binary.LittleEndian.Uint32(data[80:84])
		if count != 257 || len(data) != 84+int(count)*50 {
			t.Fatalf("invalid STL length/count: %d / %d", len(data), count)
		}
		minZ, maxZ := math.Inf(1), math.Inf(-1)
		for i := 0; i < int(count); i++ {
			for v := 0; v < 3; v++ {
				offset := 84 + i*50 + 12 + v*12 + 8
				z := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[offset : offset+4])))
				minZ = math.Min(minZ, z)
				maxZ = math.Max(maxZ, z)
			}
		}
		if minZ != 0 || math.Abs(maxZ-float64(height)) > 1e-5 {
			t.Fatalf("print height=%f..%f, want 0..%d", minZ, maxZ, height)
		}
		// The fixture's normal points +Z in the viewer and -Y on the print bed.
		ny := math.Float32frombits(binary.LittleEndian.Uint32(data[88:92]))
		if math.Abs(float64(ny+1)) > 1e-6 {
			t.Fatalf("incorrect axis conversion, normal Y=%f", ny)
		}
	}
}

func TestPrintSTLDoesNotDuplicateCustomBase(t *testing.T) {
	manifest := printFixture(t, "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")
	manifest.Placements[0].Slot = "base"
	data, err := buildPrintSTL(context.Background(), manifest)
	if err != nil {
		t.Fatal(err)
	}
	if count := binary.LittleEndian.Uint32(data[80:84]); count != 1 {
		t.Fatalf("generated duplicate base, triangles=%d", count)
	}
}

func TestPrintSTLRejectsPartialAndInvalidGeometry(t *testing.T) {
	for _, source := range []string{"v 0 0 0\n", "v NaN 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n", "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\nf 1 2 99\n"} {
		data, err := buildPrintSTL(context.Background(), printFixture(t, source))
		if !errors.Is(err, ErrPrintGeometry) || data != nil {
			t.Fatalf("invalid source produced STL: %v", err)
		}
	}
	manifest := printFixture(t, "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")
	missing := manifest.Placements[0]
	missing.Asset.StoragePath += ".missing"
	manifest.Placements = append(manifest.Placements, missing)
	if data, err := buildPrintSTL(context.Background(), manifest); !errors.Is(err, ErrPrintGeometry) || data != nil {
		t.Fatal("missing second asset must fail the whole download")
	}
}

func TestPrintSTLRejectsEmptySelection(t *testing.T) {
	if data, err := buildPrintSTL(context.Background(), assembly.Manifest{ScaleMM: 32}); !errors.Is(err, ErrPrintGeometry) || data != nil {
		t.Fatal("empty assembly produced a file")
	}
}

func TestPrintSTLHTTPRejectsInvalidRequests(t *testing.T) {
	handler := &Handler{}
	for _, tc := range []struct {
		method, body string
		status       int
	}{
		{http.MethodGet, "", 405}, {http.MethodPost, "not json", 400}, {http.MethodPost, `{"config":{}}`, 400},
		{http.MethodPost, `{"config":{}} {"extra":true}`, 400},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(tc.method, "/api/exports/stl", strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		handler.PrintSTL(recorder, request)
		if recorder.Code != tc.status || recorder.Header().Get("Content-Disposition") != "" {
			t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
		}
	}
}
