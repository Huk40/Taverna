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
