package modelasset

import (
	"bytes"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"

	"taverna/backend/internal/catalog"
)

func TestUploadLimitsAndRejectedFileCleanup(t *testing.T) {
	service, err := catalog.NewService()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, filename string
		size, status   int
	}{
		{"empty", "empty.obj", 0, 400},
		{"unsupported format", "test.exe", 10, 400},
		{"multipart overhead allowed", "broken.obj", 64, 400},
		{"file limit", "large.obj", 65, 413},
		{"body limit", "huge.obj", (1 << 20) + 65, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storage := t.TempDir()
			handler := NewHandler(nil, service, storage, 64, slog.New(slog.NewTextHandler(io.Discard, nil)))
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("file", tc.filename)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := part.Write(bytes.Repeat([]byte("x"), tc.size)); err != nil {
				t.Fatal(err)
			}
			if err := writer.WriteField("slot", "gear"); err != nil {
				t.Fatal(err)
			}
			if err := writer.WriteField("attachment_point", "right_hand"); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest("POST", "/api/model-assets", &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			res := httptest.NewRecorder()
			handler.Collection(res, req)
			if res.Code != tc.status {
				t.Fatalf("status=%d, body=%s", res.Code, res.Body)
			}
			if tc.name == "multipart overhead allowed" && !bytes.Contains(res.Body.Bytes(), []byte("triangle geometry")) {
				t.Fatalf("expected geometry rejection after multipart parsing, got %s", res.Body)
			}
			entries, err := os.ReadDir(storage)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatal("rejected upload left a file in storage")
			}
		})
	}
}

func TestModelContentTypeIgnoresUntrustedHeader(t *testing.T) {
	if got := contentTypeFor("text/html", "sword.OBJ"); got != "model/obj" {
		t.Fatalf("got unsafe content type %q", got)
	}
	if got := contentTypeFor("text/html", "sword.unknown"); got != "application/octet-stream" {
		t.Fatalf("got unsafe content type %q", got)
	}
}

func TestFormatForFileName(t *testing.T) {
	tests := map[string]string{
		"hero.glb":   FormatGLB,
		"hero.GLTF":  FormatGLTF,
		"weapon.obj": FormatOBJ,
		"base.stl":   FormatSTL,
		"print.3mf":  Format3MF,
	}

	for fileName, want := range tests {
		got, ok := formatForFileName(fileName)
		if !ok {
			t.Fatalf("formatForFileName(%q) should be allowed", fileName)
		}
		if got != want {
			t.Fatalf("formatForFileName(%q) = %q, want %q", fileName, got, want)
		}
	}

	if _, ok := formatForFileName("notes.txt"); ok {
		t.Fatal("notes.txt should not be accepted as a model asset")
	}
}

func TestDecodeAssemblyPointsMarksMissingPayload(t *testing.T) {
	points, provided, err := decodeAssemblyPoints("")
	if err != nil {
		t.Fatalf("decodeAssemblyPoints() error = %v", err)
	}
	if provided {
		t.Fatal("empty assembly_points should not be marked as provided")
	}
	if len(points) != 0 {
		t.Fatalf("expected no custom points, got %d", len(points))
	}
}

func TestSuggestedAssemblyPointsUsesParsedBounds(t *testing.T) {
	points := suggestedAssemblyPoints(GeometryMetadata{
		Status: "parsed",
		Bounds: &GeometryBounds{
			Min:    Vector3{-0.2, -0.6, -0.1},
			Max:    Vector3{0.2, 1.1, 0.1},
			Center: Vector3{0, 0.25, 0},
		},
	}, "gear", "right_hand")

	if len(points) < 4 {
		t.Fatalf("expected bounds-derived anchor presets, got %d", len(points))
	}
	if points[0].ID != "grip" {
		t.Fatalf("first suggested point id = %q", points[0].ID)
	}
	if points[0].Position != (Vector3{0, -0.6, 0}) {
		t.Fatalf("grip point position = %v", points[0].Position)
	}
	if points[1].ID != "origin" {
		t.Fatalf("second suggested point id = %q", points[1].ID)
	}
}

func TestSuggestedAssemblyPointsFallsBackWithoutBounds(t *testing.T) {
	points := suggestedAssemblyPoints(GeometryMetadata{Status: "stored"}, "gear", "right_hand")
	if len(points) != 1 {
		t.Fatalf("expected one fallback point, got %d", len(points))
	}
	if points[0].ID != "right_hand" {
		t.Fatalf("fallback point id = %q", points[0].ID)
	}
}

func TestValidateUploadGeometryAllowsParsedTriangleGeometry(t *testing.T) {
	err := validateUploadGeometry(FormatOBJ, GeometryMetadata{
		Status:        "parsed",
		Bounds:        &GeometryBounds{Min: Vector3{0, 0, 0}, Max: Vector3{1, 1, 0}, Size: Vector3{1, 1, 0}, Center: Vector3{0.5, 0.5, 0}},
		TriangleCount: 1,
	})
	if err != nil {
		t.Fatalf("validateUploadGeometry() error = %v", err)
	}
}

func TestValidateUploadGeometryRejectsEmptyParsedFormats(t *testing.T) {
	err := validateUploadGeometry(FormatOBJ, GeometryMetadata{
		Status:   "empty",
		Warnings: []string{"no vertices were found"},
	})
	if err == nil {
		t.Fatal("expected empty OBJ geometry to fail")
	}
	if err.Error() != "file does not contain readable OBJ triangle geometry: no vertices were found" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestValidateUploadGeometryRejectsMissingTriangles(t *testing.T) {
	err := validateUploadGeometry(FormatGLB, GeometryMetadata{
		Status:        "parsed",
		Bounds:        &GeometryBounds{Min: Vector3{0, 0, 0}, Max: Vector3{1, 1, 0}, Size: Vector3{1, 1, 0}, Center: Vector3{0.5, 0.5, 0}},
		TriangleCount: 0,
	})
	if err == nil {
		t.Fatal("expected parsed GLB without triangles to fail")
	}
	if err.Error() != "file does not contain GLB triangle faces" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestValidateUploadGeometryIgnoresUnknownFormats(t *testing.T) {
	if err := validateUploadGeometry("fbx", GeometryMetadata{Status: "stored"}); err != nil {
		t.Fatalf("validateUploadGeometry() error = %v", err)
	}
}

func TestValidateUploadGeometryRequiresParsedGLTF(t *testing.T) {
	err := validateUploadGeometry(FormatGLTF, GeometryMetadata{Status: "stored"})
	if err == nil {
		t.Fatal("expected stored GLTF geometry to fail")
	}
	if err.Error() != "file does not contain readable GLTF triangle geometry: geometry status is stored" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestValidateUploadGeometryRequiresParsed3MF(t *testing.T) {
	err := validateUploadGeometry(Format3MF, GeometryMetadata{Status: "stored"})
	if err == nil {
		t.Fatal("expected stored 3MF geometry to fail")
	}
	if err.Error() != "file does not contain readable 3MF triangle geometry: geometry status is stored" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestDecodeAssemblyPointsRejectsInvalidJSON(t *testing.T) {
	if _, _, err := decodeAssemblyPoints("{"); err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
}

func TestDecodeAssemblyPointsNormalizesCustomPoints(t *testing.T) {
	points, provided, err := decodeAssemblyPoints(`[{"id":" grip ","label":"","position":[0.1,0,0],"rotation":[0,15,0]}]`)
	if err != nil {
		t.Fatalf("decodeAssemblyPoints() error = %v", err)
	}
	if !provided {
		t.Fatal("custom assembly_points should be marked as provided")
	}
	if len(points) != 1 {
		t.Fatalf("expected one point, got %d", len(points))
	}
	if points[0].ID != "grip" {
		t.Fatalf("point id = %q", points[0].ID)
	}
	if points[0].Label != "grip" {
		t.Fatalf("point label = %q", points[0].Label)
	}
	if points[0].Position != (Vector3{0.1, 0, 0}) {
		t.Fatalf("point position = %v", points[0].Position)
	}
}

func TestDecodeAssemblyPointsRejectsDuplicateIDs(t *testing.T) {
	raw := `[{"id":"grip","label":"Grip","position":[0,0,0],"rotation":[0,0,0]},{"id":"grip","label":"Grip 2","position":[0,0,0],"rotation":[0,0,0]}]`
	if _, _, err := decodeAssemblyPoints(raw); err == nil {
		t.Fatal("expected duplicate IDs to fail")
	}
}

func TestDecodeAssemblyPointsRejectsOutOfRangePosition(t *testing.T) {
	raw := `[{"id":"grip","label":"Grip","position":[21,0,0],"rotation":[0,0,0]}]`
	if _, _, err := decodeAssemblyPoints(raw); err == nil {
		t.Fatal("expected out-of-range position to fail")
	}
}

func TestNormalizeAssemblyPointsSupportsUpdatePayload(t *testing.T) {
	points, err := normalizeAssemblyPoints([]AssemblyPoint{{
		ID:       " tip ",
		Label:    " ",
		Position: Vector3{0, 1.25, 0},
		Rotation: Vector3{0, 0, 90},
	}})
	if err != nil {
		t.Fatalf("normalizeAssemblyPoints() error = %v", err)
	}
	if points[0].ID != "tip" {
		t.Fatalf("point id = %q", points[0].ID)
	}
	if points[0].Label != "tip" {
		t.Fatalf("point label = %q", points[0].Label)
	}
}
