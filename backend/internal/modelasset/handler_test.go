package modelasset

import "testing"

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

func TestDecodeAssemblyPointsUsesFallback(t *testing.T) {
	points, err := decodeAssemblyPoints("", "right_hand")
	if err != nil {
		t.Fatalf("decodeAssemblyPoints() error = %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected one fallback point, got %d", len(points))
	}
	if points[0].ID != "right_hand" {
		t.Fatalf("fallback point id = %q", points[0].ID)
	}
}

func TestDecodeAssemblyPointsRejectsInvalidJSON(t *testing.T) {
	if _, err := decodeAssemblyPoints("{", "right_hand"); err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
}

func TestDecodeAssemblyPointsNormalizesCustomPoints(t *testing.T) {
	points, err := decodeAssemblyPoints(`[{"id":" grip ","label":"","position":[0.1,0,0],"rotation":[0,15,0]}]`, "right_hand")
	if err != nil {
		t.Fatalf("decodeAssemblyPoints() error = %v", err)
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
	if _, err := decodeAssemblyPoints(raw, "right_hand"); err == nil {
		t.Fatal("expected duplicate IDs to fail")
	}
}

func TestDecodeAssemblyPointsRejectsOutOfRangePosition(t *testing.T) {
	raw := `[{"id":"grip","label":"Grip","position":[21,0,0],"rotation":[0,0,0]}]`
	if _, err := decodeAssemblyPoints(raw, "right_hand"); err == nil {
		t.Fatal("expected out-of-range position to fail")
	}
}
