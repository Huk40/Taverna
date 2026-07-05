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
	if catalog.Version != "2026.07.seed" {
		t.Fatalf("unexpected catalog version %q", catalog.Version)
	}
	if got := len(service.Slots()); got != 9 {
		t.Fatalf("expected 9 slots, got %d", got)
	}
	if got := len(service.Parts("gear")); got != 6 {
		t.Fatalf("expected 6 gear parts, got %d", got)
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
	config := service.Catalog().Defaults
	config.Body = "compact_body_01"
	config.Gear = "longbow_01"

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
