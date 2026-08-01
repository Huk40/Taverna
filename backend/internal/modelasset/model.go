package modelasset

import "time"

const (
	FormatGLB  = "glb"
	FormatGLTF = "gltf"
	FormatOBJ  = "obj"
	FormatSTL  = "stl"
	Format3MF  = "3mf"
)

type ModelAsset struct {
	ID              string          `json:"id"`
	Label           string          `json:"label"`
	Slot            string          `json:"slot"`
	AttachmentPoint string          `json:"attachment_point"`
	FileName        string          `json:"file_name"`
	ContentType     string          `json:"content_type"`
	Format          string          `json:"format"`
	SizeBytes       int64           `json:"size_bytes"`
	URL             string          `json:"url"`
	AssemblyPoints  []AssemblyPoint `json:"assembly_points"`
	StoragePath     string          `json:"-"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type AssemblyPoint struct {
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	Position Vector3 `json:"position"`
	Rotation Vector3 `json:"rotation"`
}

type Vector3 [3]float64

type CreateRequest struct {
	ID              string
	Label           string
	Slot            string
	AttachmentPoint string
	FileName        string
	ContentType     string
	Format          string
	SizeBytes       int64
	StoragePath     string
	AssemblyPoints  []AssemblyPoint
}

type UpdateRequest struct {
	Label          string
	AssemblyPoints []AssemblyPoint
}
