package assembly

import (
	"context"
	"errors"
	"testing"

	"taverna/backend/internal/catalog"
	"taverna/backend/internal/modelasset"
)

func TestSocketTransformUsesBodyHeight(t *testing.T) {
	socket := socketTransform("head_mount", bodyHeightFor("heroic_body_01"), "idle_01")
	if socket.Position[1] != 2.8 {
		t.Fatalf("head_mount y = %v, want 2.8", socket.Position[1])
	}
}

func TestComposeWorldTransformAppliesCharacterScale(t *testing.T) {
	socket := Transform{
		Position: [3]float64{1, 2, 3},
		Rotation: [3]float64{10, 20, 30},
		Scale:    1,
	}
	user := Transform{
		Position: [3]float64{0.5, 0, -0.5},
		Rotation: [3]float64{5, -10, 15},
		Scale:    2,
	}

	world := composeWorldTransform(socket, user, modelasset.AssemblyPoint{}, 1.25)
	if world.Position != [3]float64{1.875, 2.5, 3.125} {
		t.Fatalf("world position = %v", world.Position)
	}
	if world.Rotation != [3]float64{15, 10, 45} {
		t.Fatalf("world rotation = %v", world.Rotation)
	}
	if world.Scale != 2.5 {
		t.Fatalf("world scale = %v", world.Scale)
	}
}

func TestComposeWorldTransformAlignsAssetPoint(t *testing.T) {
	socket := Transform{
		Position: [3]float64{1, 2, 3},
		Rotation: [3]float64{0, 0, 0},
		Scale:    1,
	}
	user := Transform{
		Position: [3]float64{0.5, 0, -0.5},
		Rotation: [3]float64{0, 0, 0},
		Scale:    2,
	}
	assetPoint := modelasset.AssemblyPoint{
		ID:       "grip",
		Position: modelasset.Vector3{0.25, 0, 0},
		Rotation: modelasset.Vector3{0, 0, 0},
	}

	world := composeWorldTransform(socket, user, assetPoint, 1)
	if world.Position != [3]float64{1, 2, 2.5} {
		t.Fatalf("world position = %v", world.Position)
	}
	if world.Scale != 2 {
		t.Fatalf("world scale = %v", world.Scale)
	}
}

func TestTransformGeometryBoundsAppliesScaleAndTranslation(t *testing.T) {
	bounds := modelasset.GeometryBounds{
		Min: modelasset.Vector3{-1, 0, 0},
		Max: modelasset.Vector3{1, 2, 3},
	}
	worldBounds := transformGeometryBounds(&bounds, Transform{
		Position: [3]float64{10, 0, -1},
		Rotation: [3]float64{0, 0, 0},
		Scale:    2,
	})
	if worldBounds == nil {
		t.Fatal("expected world bounds")
	}
	if worldBounds.Min != (modelasset.Vector3{8, 0, -1}) {
		t.Fatalf("world min = %v", worldBounds.Min)
	}
	if worldBounds.Max != (modelasset.Vector3{12, 4, 5}) {
		t.Fatalf("world max = %v", worldBounds.Max)
	}
	if worldBounds.Size != (modelasset.Vector3{4, 4, 6}) {
		t.Fatalf("world size = %v", worldBounds.Size)
	}
	if worldBounds.Center != (modelasset.Vector3{10, 2, 2}) {
		t.Fatalf("world center = %v", worldBounds.Center)
	}
}

func TestTransformGeometryBoundsAppliesRotation(t *testing.T) {
	bounds := modelasset.GeometryBounds{
		Min: modelasset.Vector3{0, 0, 0},
		Max: modelasset.Vector3{1, 2, 0},
	}
	worldBounds := transformGeometryBounds(&bounds, Transform{
		Position: [3]float64{0, 0, 0},
		Rotation: [3]float64{0, 0, 90},
		Scale:    1,
	})
	if worldBounds == nil {
		t.Fatal("expected world bounds")
	}
	if worldBounds.Min != (modelasset.Vector3{-2, 0, 0}) {
		t.Fatalf("world min = %v", worldBounds.Min)
	}
	if worldBounds.Max != (modelasset.Vector3{0, 1, 0}) {
		t.Fatalf("world max = %v", worldBounds.Max)
	}
}

func TestAssemblyBoundsAccumulatorCombinesPlacements(t *testing.T) {
	accumulator := assemblyBoundsAccumulator{}
	accumulator.AddBounds(modelasset.GeometryBounds{
		Min: modelasset.Vector3{-1, 0, 0},
		Max: modelasset.Vector3{1, 1, 1},
	})
	accumulator.AddBounds(modelasset.GeometryBounds{
		Min: modelasset.Vector3{2, -2, 0},
		Max: modelasset.Vector3{3, 1, 4},
	})

	bounds := accumulator.Bounds()
	if bounds == nil {
		t.Fatal("expected combined bounds")
	}
	if bounds.Min != (modelasset.Vector3{-1, -2, 0}) {
		t.Fatalf("combined min = %v", bounds.Min)
	}
	if bounds.Max != (modelasset.Vector3{3, 1, 4}) {
		t.Fatalf("combined max = %v", bounds.Max)
	}
	if bounds.Size != (modelasset.Vector3{4, 3, 4}) {
		t.Fatalf("combined size = %v", bounds.Size)
	}
}

func TestResolveAssetPointUsesRequestedPoint(t *testing.T) {
	asset := modelasset.ModelAsset{
		ID:              "asset-1",
		AttachmentPoint: "right_hand",
		AssemblyPoints: []modelasset.AssemblyPoint{
			{ID: "origin", Label: "Origin"},
			{ID: "grip", Label: "Grip", Position: modelasset.Vector3{0.1, 0, 0}},
		},
	}

	point, err := resolveAssetPoint(asset, "grip")
	if err != nil {
		t.Fatalf("resolveAssetPoint() error = %v", err)
	}
	if point.ID != "grip" {
		t.Fatalf("point id = %q", point.ID)
	}
}

func TestResolveAssetPointRejectsUnknownRequestedPoint(t *testing.T) {
	asset := modelasset.ModelAsset{
		ID:              "asset-1",
		AttachmentPoint: "right_hand",
		AssemblyPoints:  []modelasset.AssemblyPoint{{ID: "origin", Label: "Origin"}},
	}

	_, err := resolveAssetPoint(asset, "missing")
	if !errors.Is(err, ErrInvalidAssembly) {
		t.Fatalf("expected ErrInvalidAssembly, got %v", err)
	}
}

func TestManifestForConfigBuildsPreviewWithoutSavedProject(t *testing.T) {
	catalogService, err := catalog.NewService()
	if err != nil {
		t.Fatalf("catalog.NewService() error = %v", err)
	}
	service := NewService(nil, nil, catalogService)

	manifest, err := service.ManifestForConfig(context.Background(), "", catalogService.Catalog().Defaults)
	if err != nil {
		t.Fatalf("ManifestForConfig() error = %v", err)
	}
	if manifest.ProjectID != "preview" {
		t.Fatalf("project id = %q", manifest.ProjectID)
	}
	if manifest.SchemaVersion != 1 {
		t.Fatalf("schema version = %d", manifest.SchemaVersion)
	}
	if len(manifest.Placements) != 0 {
		t.Fatalf("placements = %d", len(manifest.Placements))
	}
}
