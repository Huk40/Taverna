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
	ID              string           `json:"id"`
	Label           string           `json:"label"`
	Slot            string           `json:"slot"`
	AttachmentPoint string           `json:"attachment_point"`
	FileName        string           `json:"file_name"`
	ContentType     string           `json:"content_type"`
	Format          string           `json:"format"`
	SizeBytes       int64            `json:"size_bytes"`
	URL             string           `json:"url"`
	AssemblyPoints  []AssemblyPoint  `json:"assembly_points"`
	Metadata        GeometryMetadata `json:"metadata"`
	StoragePath     string           `json:"-"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

type AssemblyPoint struct {
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	Position Vector3 `json:"position"`
	Rotation Vector3 `json:"rotation"`
}

type Vector3 [3]float64

type GeometryMetadata struct {
	Status        string          `json:"status"`
	Source        string          `json:"source,omitempty"`
	Bounds        *GeometryBounds `json:"bounds,omitempty"`
	VertexCount   int             `json:"vertex_count,omitempty"`
	FaceCount     int             `json:"face_count,omitempty"`
	TriangleCount int             `json:"triangle_count,omitempty"`
	Warnings      []string        `json:"warnings"`
}

type GeometryBounds struct {
	Min    Vector3 `json:"min"`
	Max    Vector3 `json:"max"`
	Size   Vector3 `json:"size"`
	Center Vector3 `json:"center"`
}

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
	Metadata        GeometryMetadata
}

type UpdateRequest struct {
	Label          string
	AssemblyPoints []AssemblyPoint
}
