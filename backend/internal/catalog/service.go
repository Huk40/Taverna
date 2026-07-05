package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	_ "embed"
)

const CurrentSchemaVersion = 1

var (
	ErrInvalidConfig = errors.New("invalid character config")
	hexColorPattern  = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

//go:embed seed/catalog.json
var seedCatalogJSON []byte

type Service struct {
	catalog      Catalog
	partsBySlot  map[string]map[string]Part
	colorZoneIDs map[string]struct{}
	slotIDs      map[string]struct{}
}

func NewService() (*Service, error) {
	var seed Catalog
	if err := json.Unmarshal(seedCatalogJSON, &seed); err != nil {
		return nil, fmt.Errorf("load catalog seed: %w", err)
	}

	service := newService(seed)
	if err := service.ValidateSeed(); err != nil {
		return nil, err
	}

	return service, nil
}

func newService(catalog Catalog) *Service {
	partsBySlot := make(map[string]map[string]Part)
	for _, part := range catalog.Parts {
		if _, ok := partsBySlot[part.Slot]; !ok {
			partsBySlot[part.Slot] = make(map[string]Part)
		}
		partsBySlot[part.Slot][part.ID] = part
	}

	colorZoneIDs := make(map[string]struct{})
	for _, zone := range catalog.ColorZones {
		colorZoneIDs[zone.ID] = struct{}{}
	}

	slotIDs := make(map[string]struct{})
	for _, slot := range catalog.Slots {
		slotIDs[slot.ID] = struct{}{}
	}

	return &Service{
		catalog:      catalog,
		partsBySlot:  partsBySlot,
		colorZoneIDs: colorZoneIDs,
		slotIDs:      slotIDs,
	}
}

func (s *Service) ValidateSeed() error {
	if s.catalog.Version == "" {
		return errors.New("catalog seed version is required")
	}
	if len(s.catalog.Slots) == 0 {
		return errors.New("catalog seed must include slots")
	}
	if len(s.catalog.Parts) == 0 {
		return errors.New("catalog seed must include parts")
	}
	if len(s.catalog.ColorZones) == 0 {
		return errors.New("catalog seed must include color zones")
	}

	seenSlots := make(map[string]struct{})
	for _, slot := range s.catalog.Slots {
		if slot.ID == "" {
			return errors.New("catalog seed contains a slot without id")
		}
		if _, ok := seenSlots[slot.ID]; ok {
			return fmt.Errorf("catalog seed contains duplicate slot %q", slot.ID)
		}
		seenSlots[slot.ID] = struct{}{}
	}

	seenParts := make(map[string]struct{})
	for _, part := range s.catalog.Parts {
		if part.ID == "" || part.Slot == "" {
			return errors.New("catalog seed contains a part without id or slot")
		}
		if _, ok := seenSlots[part.Slot]; !ok {
			return fmt.Errorf("catalog seed part %q references unknown slot %q", part.ID, part.Slot)
		}
		if _, ok := seenParts[part.ID]; ok {
			return fmt.Errorf("catalog seed contains duplicate part %q", part.ID)
		}
		seenParts[part.ID] = struct{}{}
		for _, zone := range part.ColorZones {
			if _, ok := s.colorZoneIDs[zone]; !ok {
				return fmt.Errorf("catalog seed part %q references unknown color zone %q", part.ID, zone)
			}
		}
	}

	seenZones := make(map[string]struct{})
	for _, zone := range s.catalog.ColorZones {
		if zone.ID == "" {
			return errors.New("catalog seed contains a color zone without id")
		}
		if _, ok := seenZones[zone.ID]; ok {
			return fmt.Errorf("catalog seed contains duplicate color zone %q", zone.ID)
		}
		if !hexColorPattern.MatchString(zone.Default) {
			return fmt.Errorf("catalog seed color zone %q has invalid default color", zone.ID)
		}
		seenZones[zone.ID] = struct{}{}
	}

	if err := s.ValidateConfig(s.catalog.Defaults); err != nil {
		return fmt.Errorf("catalog seed defaults are invalid: %w", err)
	}

	for _, rule := range s.catalog.Rules {
		if rule.ID == "" {
			return errors.New("catalog seed contains a compatibility rule without id")
		}
		if err := s.validateSelection(rule.When); err != nil {
			return fmt.Errorf("catalog seed rule %q has invalid condition: %w", rule.ID, err)
		}
		if len(rule.Disables) == 0 {
			return fmt.Errorf("catalog seed rule %q must disable at least one selection", rule.ID)
		}
		for _, disabled := range rule.Disables {
			if err := s.validateSelection(disabled); err != nil {
				return fmt.Errorf("catalog seed rule %q disables an invalid selection: %w", rule.ID, err)
			}
		}
	}

	return nil
}

func (s *Service) Catalog() Catalog {
	return s.catalog
}

func (s *Service) Slots() []Slot {
	return s.catalog.Slots
}

func (s *Service) Rules() []CompatibilityRule {
	return s.catalog.Rules
}

func (s *Service) Parts(slot string) []Part {
	if slot == "" {
		return s.catalog.Parts
	}

	parts := make([]Part, 0)
	for _, part := range s.catalog.Parts {
		if part.Slot == slot {
			parts = append(parts, part)
		}
	}
	return parts
}

func (s *Service) ValidateRawConfig(raw json.RawMessage) error {
	if len(raw) == 0 || !json.Valid(raw) {
		return fmt.Errorf("%w: config must be valid JSON", ErrInvalidConfig)
	}

	var config CharacterConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	return s.ValidateConfig(config)
}

func (s *Service) ValidateConfig(config CharacterConfig) error {
	if config.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("%w: schema_version must be %d", ErrInvalidConfig, CurrentSchemaVersion)
	}

	selections := selectionMap(config)
	for slot, partID := range selections {
		if partID == "" {
			return fmt.Errorf("%w: %s is required", ErrInvalidConfig, slot)
		}
		if _, ok := s.partsBySlot[slot][partID]; !ok {
			return fmt.Errorf("%w: unknown %s part %q", ErrInvalidConfig, slot, partID)
		}
	}

	if config.Scale.MM < 24 || config.Scale.MM > 75 {
		return fmt.Errorf("%w: scale.mm must be between 24 and 75", ErrInvalidConfig)
	}
	if err := s.validatePrintConstraints(config, selections); err != nil {
		return err
	}

	if len(config.Colors) == 0 {
		return fmt.Errorf("%w: colors must include material zones", ErrInvalidConfig)
	}

	for _, requiredZone := range []string{"skin", "clothing_primary", "gear_metal", "stage"} {
		value, ok := config.Colors[requiredZone]
		if !ok || value == "" {
			return fmt.Errorf("%w: missing color zone %q", ErrInvalidConfig, requiredZone)
		}
	}

	for zone, value := range config.Colors {
		if _, ok := s.colorZoneIDs[zone]; !ok {
			return fmt.Errorf("%w: unknown color zone %q", ErrInvalidConfig, zone)
		}
		if !hexColorPattern.MatchString(value) {
			return fmt.Errorf("%w: color zone %q must be a hex color", ErrInvalidConfig, zone)
		}
	}

	for _, rule := range s.catalog.Rules {
		if selections[rule.When.Slot] != rule.When.PartID {
			continue
		}
		for _, disabled := range rule.Disables {
			if selections[disabled.Slot] == disabled.PartID {
				return fmt.Errorf("%w: %s", ErrInvalidConfig, rule.Reason)
			}
		}
	}

	return nil
}

func selectionMap(config CharacterConfig) map[string]string {
	return map[string]string{
		"species":        config.Species,
		"body":           config.Body,
		"head":           config.Head,
		"clothing":       config.Clothing,
		"gear":           config.Gear,
		"pose":           config.Pose,
		"stage":          config.Stage,
		"base":           config.Base,
		"product_target": config.ProductTarget,
	}
}

func (s *Service) validatePrintConstraints(config CharacterConfig, selections map[string]string) error {
	maxScale := 75
	constrainedBy := ""

	for slot, partID := range selections {
		part := s.partsBySlot[slot][partID]
		if part.PrintConstraints.MaxScaleMM > 0 && part.PrintConstraints.MaxScaleMM < maxScale {
			maxScale = part.PrintConstraints.MaxScaleMM
			constrainedBy = part.ID
		}
	}

	if config.Scale.MM > maxScale {
		return fmt.Errorf("%w: scale.mm exceeds print constraint max %d for %s", ErrInvalidConfig, maxScale, constrainedBy)
	}

	return nil
}

func (s *Service) validateSelection(selection Selection) error {
	if selection.Slot == "" || selection.PartID == "" {
		return errors.New("selection slot and part_id are required")
	}
	if _, ok := s.slotIDs[selection.Slot]; !ok {
		return fmt.Errorf("unknown slot %q", selection.Slot)
	}
	if _, ok := s.partsBySlot[selection.Slot][selection.PartID]; !ok {
		return fmt.Errorf("unknown %s part %q", selection.Slot, selection.PartID)
	}
	return nil
}
