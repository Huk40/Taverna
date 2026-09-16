package catalog

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestSeedCatalogLoadsAndValidatesDefaults(t *testing.T) {
	service, err := NewService()
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	catalog := service.Catalog()
	if catalog.Version != "2026.09.models" {
		t.Fatalf("unexpected catalog version %q", catalog.Version)
	}
	if got := len(service.Slots()); got != 7 {
		t.Fatalf("expected 7 slots, got %d", got)
	}
	for _, slot := range []string{"species", "body", "head", "clothing", "gear", "pose", "stage"} {
		if got := len(service.Parts(slot)); got != 0 {
			t.Fatalf("retired demo parts remain in %s: %d", slot, got)
		}
	}
	if service.HasSlot("species") || service.HasSlot("pose") {
		t.Fatal("retired categories must not appear in the catalog")
	}
	if err := service.ValidateConfig(catalog.Defaults); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}
}

func TestValidateRawConfigRejectsInvalidSchemaVersion(t *testing.T) {
	service := mustService(t)
	config := service.Catalog().Defaults
	config.SchemaVersion = 2

	raw := mustJSON(t, config)
	err := service.ValidateRawConfig(raw)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestValidateConfigRejectsUnknownPart(t *testing.T) {
	service := mustService(t)
	config := service.Catalog().Defaults
	config.Gear = "missing_gear_01"

	err := service.ValidateConfig(config)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestValidateConfigRejectsCompatibilityConflict(t *testing.T) {
	service := mustService(t)
	seed := service.Catalog()
	seed.Rules = []CompatibilityRule{{
		ID: "fixture-conflict", When: Selection{Slot: "base", PartID: "round_base_32mm"},
		Disables: []Selection{{Slot: "product_target", PartID: "glb_preview"}}, Reason: "fixture conflict",
	}}
	service = newService(seed)
	config := seed.Defaults
	config.ProductTarget = "glb_preview"

	err := service.ValidateConfig(config)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestDecodeRetiredProjectPreservesUploadedModels(t *testing.T) {
	service := mustService(t)
	config := service.Catalog().Defaults
	config.Species = "elf_01"
	config.Body = "compact_body_01"
	config.Head = "default_head_01"
	config.Gear = "sword_01"
	config.Base = "round_base_40mm"
	config.Assembly.Placements = []AssemblyPlacement{{
		ID: "saved-head", Slot: "head", AttachmentPoint: "head_mount", AssetID: "saved-asset", Transform: AssemblyTransform{Scale: 1},
	}}
	decoded, err := service.DecodeRawConfig(mustJSON(t, config))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Species != "" || decoded.Body != "" || decoded.Head != "" || decoded.Gear != "" {
		t.Fatal("retired selections should be cleared when opening an old project")
	}
	if decoded.Base != service.Catalog().Defaults.Base || len(decoded.Assembly.Placements) != 1 || decoded.Assembly.Placements[0].AssetID != "saved-asset" {
		t.Fatal("project migration lost the base or uploaded model")
	}
	if err := service.ValidateConfig(decoded); err != nil {
		t.Fatal(err)
	}
}

func TestValidateConfigRejectsScaleAbovePrintConstraint(t *testing.T) {
	service := mustService(t)
	config := service.Catalog().Defaults
	config.Scale.MM = 60

	err := service.ValidateConfig(config)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestValidateConfigAllowsAssemblyPlacement(t *testing.T) {
	service := mustService(t)
	config := service.Catalog().Defaults
	config.Assembly.Placements = []AssemblyPlacement{{
		ID:              "placement-1",
		Slot:            "gear",
		AttachmentPoint: "right_hand",
		AssetID:         "11111111-1111-4111-8111-111111111111",
		Transform: AssemblyTransform{
			Position: Vector3{0.1, 0, 0},
			Rotation: Vector3{0, 15, 0},
			Scale:    1,
		},
	}}

	if err := service.ValidateConfig(config); err != nil {
		t.Fatalf("assembly placement should be valid: %v", err)
	}
}

func TestValidateConfigRejectsInvalidAssemblyPoint(t *testing.T) {
	service := mustService(t)
	config := service.Catalog().Defaults
	config.Assembly.Placements = []AssemblyPlacement{{
		ID:              "placement-1",
		Slot:            "gear",
		AttachmentPoint: "head_mount",
		AssetID:         "11111111-1111-4111-8111-111111111111",
		Transform:       AssemblyTransform{Scale: 1},
	}}

	err := service.ValidateConfig(config)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestValidateSeedRejectsRuleWithUnknownPart(t *testing.T) {
	service := mustService(t)
	seed := service.Catalog()
	seed.Rules = append(seed.Rules, CompatibilityRule{
		ID:          "bad-rule",
		Description: "Bad rule",
		When:        Selection{Slot: "gear", PartID: "missing_gear"},
		Disables:    []Selection{{Slot: "pose", PartID: "idle_01"}},
		Reason:      "bad seed",
	})

	err := newService(seed).ValidateSeed()
	if err == nil {
		t.Fatal("expected bad seed to fail validation")
	}
}

func mustService(t *testing.T) *Service {
	t.Helper()

	service, err := NewService()
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return raw
}
