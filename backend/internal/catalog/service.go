package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
)

const CurrentSchemaVersion = 1

var (
	ErrInvalidConfig = errors.New("invalid character config")
	hexColorPattern  = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

type Service struct {
	catalog      Catalog
	partsBySlot  map[string]map[string]Part
	colorZoneIDs map[string]struct{}
}

func NewService() *Service {
	catalog := seedCatalog()
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

	return &Service{
		catalog:      catalog,
		partsBySlot:  partsBySlot,
		colorZoneIDs: colorZoneIDs,
	}
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

	selections := map[string]string{
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

func seedCatalog() Catalog {
	colorZones := []ColorZone{
		{ID: "skin", Label: "Skin", Default: "#c99672"},
		{ID: "hair", Label: "Hair", Default: "#3b2416"},
		{ID: "clothing_primary", Label: "Clothing primary", Default: "#444444"},
		{ID: "clothing_secondary", Label: "Clothing secondary", Default: "#8B0000"},
		{ID: "gear_metal", Label: "Gear metal", Default: "#d8dee9"},
		{ID: "stage", Label: "Stage", Default: "#29313d"},
	}

	return Catalog{
		Version: "2026.07.seed",
		Slots: []Slot{
			{ID: "species", Label: "Species", Description: "Playable ancestry or creature archetype."},
			{ID: "body", Label: "Body", Description: "Body proportions used by the preview and print target."},
			{ID: "head", Label: "Head", Description: "Head shape and facial silhouette."},
			{ID: "clothing", Label: "Clothing", Description: "Main outfit or armor layer."},
			{ID: "gear", Label: "Gear", Description: "Held weapon or prop."},
			{ID: "pose", Label: "Pose", Description: "Full-body stance preset."},
			{ID: "stage", Label: "Stage", Description: "Display scene or terrain accent."},
			{ID: "base", Label: "Base", Description: "Physical print base size."},
			{ID: "product_target", Label: "Product Target", Description: "Intended downstream output."},
		},
		Parts: []Part{
			part("human_01", "species", "Human", []string{"humanoid"}, []string{"skin"}),
			part("elf_01", "species", "Elf", []string{"humanoid", "slender"}, []string{"skin"}),
			part("dwarf_01", "species", "Dwarf", []string{"humanoid", "compact"}, []string{"skin"}),
			part("orc_01", "species", "Orc", []string{"humanoid", "large"}, []string{"skin"}),

			part("balanced_body_01", "body", "Balanced body", []string{"medium"}, []string{"skin"}),
			part("compact_body_01", "body", "Compact body", []string{"compact"}, []string{"skin"}),
			part("heroic_body_01", "body", "Heroic body", []string{"large"}, []string{"skin"}),

			part("default_head_01", "head", "Default head", []string{"neutral"}, []string{"skin", "hair"}),
			part("square_head_01", "head", "Square head", []string{"strong"}, []string{"skin", "hair"}),
			part("noble_head_01", "head", "Noble head", []string{"slender"}, []string{"skin", "hair"}),

			part("basic_armor_01", "clothing", "Basic armor", []string{"armor"}, []string{"clothing_primary", "clothing_secondary"}),
			part("travel_cloak_01", "clothing", "Travel cloak", []string{"cloth"}, []string{"clothing_primary", "clothing_secondary"}),
			part("heavy_plate_01", "clothing", "Heavy plate", []string{"armor", "heavy"}, []string{"clothing_primary", "gear_metal"}),
			part("mage_robe_01", "clothing", "Mage robe", []string{"cloth", "robe"}, []string{"clothing_primary", "clothing_secondary"}),

			part("sword_01", "gear", "Sword", []string{"weapon", "one_hand"}, []string{"gear_metal"}),
			part("axe_01", "gear", "Axe", []string{"weapon", "one_hand"}, []string{"gear_metal"}),
			part("staff_01", "gear", "Staff", []string{"weapon", "two_hand"}, []string{"gear_metal", "clothing_secondary"}),
			part("longbow_01", "gear", "Longbow", []string{"weapon", "two_hand"}, []string{"gear_metal"}),
			part("tower_shield_01", "gear", "Tower shield", []string{"shield", "large"}, []string{"gear_metal", "clothing_primary"}),
			part("none_01", "gear", "No gear", []string{"empty"}, []string{}),

			part("idle_01", "pose", "Idle", []string{"stable"}, []string{}),
			part("guard_01", "pose", "Guard", []string{"combat"}, []string{}),
			part("victory_01", "pose", "Victory", []string{"expressive"}, []string{}),

			part("plain_stage_01", "stage", "Plain stage", []string{"simple"}, []string{"stage"}),
			part("dungeon_stage_01", "stage", "Dungeon stones", []string{"terrain"}, []string{"stage"}),
			part("ruins_stage_01", "stage", "Ancient ruins", []string{"terrain"}, []string{"stage"}),

			part("round_base_32mm", "base", "Round base 32 mm", []string{"print_base"}, []string{"stage"}),
			part("round_base_40mm", "base", "Round base 40 mm", []string{"print_base"}, []string{"stage"}),

			part("stl_print", "product_target", "STL print", []string{"digital", "print"}, []string{}),
			part("glb_preview", "product_target", "GLB preview", []string{"digital", "preview"}, []string{}),
			part("physical_print", "product_target", "Physical print", []string{"print", "fulfillment"}, []string{}),
		},
		Rules: []CompatibilityRule{
			{
				ID:          "compact-body-no-longbow",
				Description: "Compact bodies use shorter gear in the MVP catalog.",
				When:        Selection{Slot: "body", PartID: "compact_body_01"},
				Disables:    []Selection{{Slot: "gear", PartID: "longbow_01"}},
				Reason:      "compact_body_01 is not compatible with longbow_01 in the MVP catalog",
			},
			{
				ID:          "robe-no-tower-shield",
				Description: "Robes avoid the large shield silhouette until kitbashing exists.",
				When:        Selection{Slot: "clothing", PartID: "mage_robe_01"},
				Disables:    []Selection{{Slot: "gear", PartID: "tower_shield_01"}},
				Reason:      "mage_robe_01 is not compatible with tower_shield_01 in the MVP catalog",
			},
			{
				ID:          "victory-no-tower-shield",
				Description: "The victory pose keeps arms open and blocks large shields.",
				When:        Selection{Slot: "pose", PartID: "victory_01"},
				Disables:    []Selection{{Slot: "gear", PartID: "tower_shield_01"}},
				Reason:      "victory_01 is not compatible with tower_shield_01 in the MVP catalog",
			},
		},
		ColorZones: colorZones,
		Defaults: CharacterConfig{
			SchemaVersion: CurrentSchemaVersion,
			Species:       "human_01",
			Body:          "balanced_body_01",
			Head:          "default_head_01",
			Clothing:      "basic_armor_01",
			Gear:          "sword_01",
			Pose:          "idle_01",
			Stage:         "plain_stage_01",
			Base:          "round_base_32mm",
			Scale:         ScaleConfig{MM: 32},
			ProductTarget: "stl_print",
			Colors:        defaultColors(colorZones),
		},
	}
}

func part(id string, slot string, label string, tags []string, colorZones []string) Part {
	maxScale := 75
	if slot == "base" {
		maxScale = 54
	}
	return Part{
		ID:           id,
		Slot:         slot,
		Label:        label,
		PreviewAsset: "placeholder://" + slot + "/" + id,
		RenderAsset:  "worker://" + slot + "/" + id,
		Tags:         slices.Clone(tags),
		ColorZones:   slices.Clone(colorZones),
		PrintConstraints: PrintConstraints{
			MinThicknessMM: 0.8,
			MaxScaleMM:     maxScale,
			SupportsSTL:    slot != "product_target",
		},
	}
}

func defaultColors(zones []ColorZone) map[string]string {
	colors := make(map[string]string, len(zones))
	for _, zone := range zones {
		colors[zone.ID] = zone.Default
	}
	return colors
}
