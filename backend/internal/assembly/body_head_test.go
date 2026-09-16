package assembly

import (
	"math"
	"testing"

	"taverna/backend/internal/modelasset"
)

func TestHeadFollowsBodyNeckInEitherPlacementOrder(t *testing.T) {
	for _, headFirst := range []bool{true, false} {
		body := ResolvedPlacement{
			Slot: "body", AssetPoints: []modelasset.AssemblyPoint{{ID: "head_mount", Position: modelasset.Vector3{0, 1.55, 0}}},
			WorldTransform: Transform{Position: [3]float64{0, 0.16, 0}, Scale: 1},
		}
		head := ResolvedPlacement{
			Slot: "head", AttachmentPoint: "head_mount", AssetPoint: modelasset.AssemblyPoint{ID: "neck"},
			UserTransform: Transform{Scale: 1},
			Asset: ModelAssetRef{Metadata: modelasset.GeometryMetadata{Bounds: &modelasset.GeometryBounds{
				Min: modelasset.Vector3{-0.2, 0, -0.2}, Max: modelasset.Vector3{0.2, 0.55, 0.2},
			}}},
		}
		placements := []ResolvedPlacement{body, head}
		headIndex := 1
		if headFirst {
			placements = []ResolvedPlacement{head, body}
			headIndex = 0
		}
		attachHeadsToBody(placements, 1)
		got := placements[headIndex]
		if math.Abs(got.WorldTransform.Position[1]-1.71) > 0.0001 {
			t.Fatalf("head must touch neck: %+v", got.WorldTransform)
		}
		if got.WorldBounds == nil || math.Abs(got.WorldBounds.Max[1]-2.26) > 0.0001 {
			t.Fatalf("head bounds=%+v", got.WorldBounds)
		}
	}
}

func TestHeadAttachmentIncludesBodyRotationAndScale(t *testing.T) {
	placements := []ResolvedPlacement{
		{Slot: "body", AssetPoints: []modelasset.AssemblyPoint{{ID: "head_mount", Position: modelasset.Vector3{0, 1, 0}}},
			WorldTransform: Transform{Position: [3]float64{3, 4, 5}, Rotation: [3]float64{0, 0, 90}, Scale: 3}},
		{Slot: "head", AttachmentPoint: "head_mount", AssetPoint: modelasset.AssemblyPoint{Position: modelasset.Vector3{0, 0.2, 0}},
			UserTransform: Transform{Scale: 0.5}},
	}
	attachHeadsToBody(placements, 1.5)
	head := placements[1]
	if head.WorldTransform.Scale != 1.5 {
		t.Fatalf("head scale=%v", head.WorldTransform.Scale)
	}
	aligned := applyTransform([3]float64(head.AssetPoint.Position), head.WorldTransform)
	if aligned != ([3]float64{0, 4, 5}) {
		t.Fatalf("neck anchors do not meet: %v", aligned)
	}
}

func TestBodyWithoutHeadMountKeepsOriginalHeadPlacement(t *testing.T) {
	original := Transform{Position: [3]float64{0, 2.64, 0}, Scale: 1}
	placements := []ResolvedPlacement{{Slot: "body"}, {Slot: "head", AttachmentPoint: "head_mount", WorldTransform: original}}
	attachHeadsToBody(placements, 1)
	if placements[1].WorldTransform != original {
		t.Fatal("legacy head transform changed")
	}
}
