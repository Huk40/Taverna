package catalog

type Catalog struct {
	Version    string              `json:"version"`
	Slots      []Slot              `json:"slots"`
	Parts      []Part              `json:"parts"`
	Rules      []CompatibilityRule `json:"rules"`
	ColorZones []ColorZone         `json:"color_zones"`
	Defaults   CharacterConfig     `json:"defaults"`
}

type Slot struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

type Part struct {
	ID               string           `json:"id"`
	Slot             string           `json:"slot"`
	Label            string           `json:"label"`
	PreviewAsset     string           `json:"preview_asset"`
	RenderAsset      string           `json:"render_asset"`
	Tags             []string         `json:"tags"`
	ColorZones       []string         `json:"color_zones"`
	PrintConstraints PrintConstraints `json:"print_constraints"`
}

type PrintConstraints struct {
	MinThicknessMM float64 `json:"min_thickness_mm"`
	MaxScaleMM     int     `json:"max_scale_mm"`
	SupportsSTL    bool    `json:"supports_stl"`
}

type ColorZone struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Default string `json:"default"`
}

type CompatibilityRule struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	When        Selection   `json:"when"`
	Disables    []Selection `json:"disables"`
	Reason      string      `json:"reason"`
}

type Selection struct {
	Slot   string `json:"slot"`
	PartID string `json:"part_id"`
}

type CharacterConfig struct {
	SchemaVersion int               `json:"schema_version"`
	Species       string            `json:"species"`
	Body          string            `json:"body"`
	Head          string            `json:"head"`
	Clothing      string            `json:"clothing"`
	Gear          string            `json:"gear"`
	Pose          string            `json:"pose"`
	Stage         string            `json:"stage"`
	Base          string            `json:"base"`
	Scale         ScaleConfig       `json:"scale"`
	ProductTarget string            `json:"product_target"`
	Colors        map[string]string `json:"colors"`
}

type ScaleConfig struct {
	MM int `json:"mm"`
}
