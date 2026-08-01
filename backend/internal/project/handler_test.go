package project

import (
	"strings"
	"testing"

	"taverna/backend/internal/catalog"
	"taverna/backend/internal/modelasset"
)

func TestValidateAssemblyAssetPlacementsAcceptsMatchingAssetPoint(t *testing.T) {
	config := configWithPlacement(catalog.AssemblyPlacement{
		ID:              "placement-1",
		Slot:            "gear",
		AttachmentPoint: "right_hand",
		AssetPointID:    "grip",
		AssetID:         "asset-1",
		Transform:       catalog.AssemblyTransform{Scale: 1},
	})
	assets := map[string]modelasset.ModelAsset{
		"asset-1": {
			ID:   "asset-1",
			Slot: "gear",
			AssemblyPoints: []modelasset.AssemblyPoint{{
				ID:       "grip",
				Label:    "Grip",
				Position: modelasset.Vector3{0, 0, 0},
				Rotation: modelasset.Vector3{0, 0, 0},
			}},
		},
	}

	if err := validateAssemblyAssetPlacements(config, assets); err != nil {
		t.Fatalf("validateAssemblyAssetPlacements() error = %v", err)
	}
}

func TestValidateAssemblyAssetPlacementsRejectsSlotMismatch(t *testing.T) {
	config := configWithPlacement(catalog.AssemblyPlacement{
		ID:              "placement-1",
		Slot:            "gear",
		AttachmentPoint: "right_hand",
		AssetPointID:    "origin",
		AssetID:         "asset-1",
		Transform:       catalog.AssemblyTransform{Scale: 1},
	})
	assets := map[string]modelasset.ModelAsset{
		"asset-1": {
			ID:             "asset-1",
			Slot:           "head",
			AssemblyPoints: []modelasset.AssemblyPoint{{ID: "origin", Label: "Origin"}},
		},
	}

	err := validateAssemblyAssetPlacements(config, assets)
	if err == nil {
		t.Fatal("expected slot mismatch to fail")
	}
	if !strings.Contains(err.Error(), `from slot "head" as slot "gear"`) {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestValidateAssemblyAssetPlacementsRejectsUnknownAssetPoint(t *testing.T) {
	config := configWithPlacement(catalog.AssemblyPlacement{
		ID:              "placement-1",
		Slot:            "gear",
		AttachmentPoint: "right_hand",
		AssetPointID:    "blade_tip",
		AssetID:         "asset-1",
		Transform:       catalog.AssemblyTransform{Scale: 1},
	})
	assets := map[string]modelasset.ModelAsset{
		"asset-1": {
			ID:             "asset-1",
			Slot:           "gear",
			AssemblyPoints: []modelasset.AssemblyPoint{{ID: "grip", Label: "Grip"}},
		},
	}

	err := validateAssemblyAssetPlacements(config, assets)
	if err == nil {
		t.Fatal("expected unknown asset_point_id to fail")
	}
	if !strings.Contains(err.Error(), `unknown asset_point_id "blade_tip"`) {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestValidateAssemblyAssetPlacementsAllowsEmptyAssetPoint(t *testing.T) {
	config := configWithPlacement(catalog.AssemblyPlacement{
		ID:              "placement-1",
		Slot:            "gear",
		AttachmentPoint: "right_hand",
		AssetID:         "asset-1",
		Transform:       catalog.AssemblyTransform{Scale: 1},
	})
	assets := map[string]modelasset.ModelAsset{
		"asset-1": {
			ID:             "asset-1",
			Slot:           "gear",
			AssemblyPoints: []modelasset.AssemblyPoint{{ID: "grip", Label: "Grip"}},
		},
	}

	if err := validateAssemblyAssetPlacements(config, assets); err != nil {
		t.Fatalf("validateAssemblyAssetPlacements() error = %v", err)
	}
}

func configWithPlacement(placement catalog.AssemblyPlacement) catalog.CharacterConfig {
	return catalog.CharacterConfig{
		SchemaVersion: 1,
		Assembly: catalog.AssemblyConfig{
			Placements: []catalog.AssemblyPlacement{placement},
		},
	}
}
