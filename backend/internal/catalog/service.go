package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"

	_ "embed"
)

const CurrentSchemaVersion = 1

const maxAssemblyPlacements = 24

var (
	ErrInvalidConfig = errors.New("invalid character config")
	hexColorPattern  = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

var assemblyPointTargets = []AssemblyPointTarget{
	{ID: "body_core", Slot: "body", Label: "Body core", Description: "Centered socket for torso-scale replacement meshes."},
	{ID: "head_mount", Slot: "head", Label: "Head mount", Description: "Top neck socket for heads, helmets, horns, and hair meshes."},
	{ID: "torso_mount", Slot: "clothing", Label: "Torso mount", Description: "Chest socket for armor, robes, and clothing shells."},
	{ID: "back_mount", Slot: "clothing", Label: "Back mount", Description: "Back socket for cloaks, backpacks, wings, and banners."},
	{ID: "right_hand", Slot: "gear", Label: "Right hand", Description: "Weapon socket aligned to the procedural right hand."},
	{ID: "left_hand", Slot: "gear", Label: "Left hand", Description: "Off-hand socket for shields, tools, and paired weapons."},
	{ID: "stage_anchor", Slot: "stage", Label: "Stage anchor", Description: "Ground-level anchor for terrain and scene props."},
	{ID: "base_mount", Slot: "base", Label: "Base mount", Description: "Center of the miniature base."},
}

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
	if catalog.Defaults.Assembly.Placements == nil {
		catalog.Defaults.Assembly.Placements = []AssemblyPlacement{}
	}

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

func (s *Service) AssemblyPointTargets() []AssemblyPointTarget {
	return assemblyPointTargets
}

func (s *Service) HasSlot(slot string) bool {
	_, ok := s.slotIDs[slot]
	return ok
}

func IsAssemblyPointTarget(slot string, attachmentPoint string) bool {
	for _, target := range assemblyPointTargets {
		if target.Slot == slot && target.ID == attachmentPoint {
			return true
		}
	}
	return false
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
	config, err := s.DecodeRawConfig(raw)
	if err != nil {
		return err
	}

	return s.ValidateConfig(config)
}

func (s *Service) DecodeRawConfig(raw json.RawMessage) (CharacterConfig, error) {
	if len(raw) == 0 || !json.Valid(raw) {
		return CharacterConfig{}, fmt.Errorf("%w: config must be valid JSON", ErrInvalidConfig)
	}

	var config CharacterConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return CharacterConfig{}, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	return config, nil
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

	if err := s.validateAssembly(config.Assembly); err != nil {
		return err
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

func (s *Service) validateAssembly(assembly AssemblyConfig) error {
	if len(assembly.Placements) > maxAssemblyPlacements {
		return fmt.Errorf("%w: assembly.placements cannot exceed %d items", ErrInvalidConfig, maxAssemblyPlacements)
	}

	seen := make(map[string]struct{})
	for index, placement := range assembly.Placements {
		if placement.ID == "" {
			return fmt.Errorf("%w: assembly.placements[%d].id is required", ErrInvalidConfig, index)
		}
		if _, ok := seen[placement.ID]; ok {
			return fmt.Errorf("%w: duplicate assembly placement %q", ErrInvalidConfig, placement.ID)
		}
		seen[placement.ID] = struct{}{}

		if placement.AssetID == "" {
			return fmt.Errorf("%w: assembly placement %q asset_id is required", ErrInvalidConfig, placement.ID)
		}
		if _, ok := s.slotIDs[placement.Slot]; !ok || placement.Slot == "product_target" {
			return fmt.Errorf("%w: assembly placement %q has unknown slot %q", ErrInvalidConfig, placement.ID, placement.Slot)
		}
		if !IsAssemblyPointTarget(placement.Slot, placement.AttachmentPoint) {
			return fmt.Errorf("%w: assembly placement %q cannot use attachment point %q for slot %q", ErrInvalidConfig, placement.ID, placement.AttachmentPoint, placement.Slot)
		}
		if len(strings.TrimSpace(placement.AssetPointID)) > 80 {
			return fmt.Errorf("%w: assembly placement %q asset_point_id is too long", ErrInvalidConfig, placement.ID)
		}
		if err := validateAssemblyTransform(placement.Transform); err != nil {
			return fmt.Errorf("%w: assembly placement %q has invalid transform: %v", ErrInvalidConfig, placement.ID, err)
		}
	}

	return nil
}

func validateAssemblyTransform(transform AssemblyTransform) error {
	for _, value := range transform.Position {
		if !finiteWithin(value, -20, 20) {
			return errors.New("position values must be finite and between -20 and 20")
		}
	}
	for _, value := range transform.Rotation {
		if !finiteWithin(value, -360, 360) {
			return errors.New("rotation values must be finite and between -360 and 360")
		}
	}
	if !finiteWithin(transform.Scale, 0.05, 10) {
		return errors.New("scale must be finite and between 0.05 and 10")
	}
	return nil
}

func finiteWithin(value float64, min float64, max float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= min && value <= max
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
