package modelasset

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

const scannerMaxTokenBytes = 1024 * 1024

func InspectGeometry(path string, format string) GeometryMetadata {
	format = strings.TrimSpace(strings.ToLower(format))
	var (
		metadata GeometryMetadata
		err      error
	)

	switch format {
	case FormatOBJ:
		metadata, err = inspectOBJGeometry(path)
	case FormatSTL:
		metadata, err = inspectSTLGeometry(path)
	case FormatGLB:
		metadata, err = inspectGLBGeometry(path)
	case Format3MF:
		metadata, err = inspect3MFGeometry(path)
	case FormatGLTF:
		return normalizeGeometryMetadata(GeometryMetadata{
			Status:   "stored",
			Source:   format,
			Warnings: []string{fmt.Sprintf("server geometry metadata is not parsed for %s assets yet", strings.ToUpper(format))},
		})
	default:
		return normalizeGeometryMetadata(GeometryMetadata{
			Status:   "unknown",
			Source:   format,
			Warnings: []string{"asset format is not recognized by geometry inspector"},
		})
	}

	if err != nil {
		return normalizeGeometryMetadata(GeometryMetadata{
			Status:   "error",
			Source:   format,
			Warnings: []string{err.Error()},
		})
	}
	return normalizeGeometryMetadata(metadata)
}

func inspectOBJGeometry(path string) (GeometryMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return GeometryMetadata{}, fmt.Errorf("read OBJ geometry: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), scannerMaxTokenBytes)

	bounds := geometryBoundsAccumulator{}
	warnings := []string{}
	vertexCount := 0
	faceCount := 0
	triangleCount := 0

	for scanner.Scan() {
		fields := strings.Fields(strings.TrimSpace(scanner.Text()))
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}

		switch fields[0] {
		case "v":
			if len(fields) < 4 {
				warnings = appendGeometryWarning(warnings, "OBJ vertex line has fewer than three coordinates")
				continue
			}
			vertex, err := parseGeometryVector(fields[1:4])
			if err != nil {
				warnings = appendGeometryWarning(warnings, fmt.Sprintf("OBJ vertex line is invalid: %v", err))
				continue
			}
			bounds.Add(vertex)
			vertexCount++
		case "f":
			if len(fields) < 4 {
				warnings = appendGeometryWarning(warnings, "OBJ face line has fewer than three vertices")
				continue
			}
			faceCount++
			triangleCount += len(fields) - 3
		}
	}
	if err := scanner.Err(); err != nil {
		return GeometryMetadata{}, fmt.Errorf("scan OBJ geometry: %w", err)
	}

	return finishGeometryMetadata("obj", vertexCount, faceCount, triangleCount, bounds, warnings), nil
}

func inspectSTLGeometry(path string) (GeometryMetadata, error) {
	triangleCount, isBinary, err := binarySTLTriangleCount(path)
	if err != nil {
		return GeometryMetadata{}, fmt.Errorf("inspect STL geometry: %w", err)
	}
	if isBinary {
		return inspectBinarySTLGeometry(path, triangleCount)
	}
	return inspectASCIISTLGeometry(path)
}

func inspectBinarySTLGeometry(path string, triangleTotal uint32) (GeometryMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return GeometryMetadata{}, fmt.Errorf("read binary STL geometry: %w", err)
	}
	defer file.Close()

	if _, err := file.Seek(84, io.SeekStart); err != nil {
		return GeometryMetadata{}, fmt.Errorf("seek binary STL geometry: %w", err)
	}

	bounds := geometryBoundsAccumulator{}
	for index := uint32(0); index < triangleTotal; index++ {
		var values [12]float32
		if err := binary.Read(file, binary.LittleEndian, &values); err != nil {
			return GeometryMetadata{}, fmt.Errorf("read binary STL triangle %d: %w", index, err)
		}
		bounds.Add(float32GeometryVector(values[3], values[4], values[5]))
		bounds.Add(float32GeometryVector(values[6], values[7], values[8]))
		bounds.Add(float32GeometryVector(values[9], values[10], values[11]))

		var attribute uint16
		if err := binary.Read(file, binary.LittleEndian, &attribute); err != nil {
			return GeometryMetadata{}, fmt.Errorf("read binary STL attribute %d: %w", index, err)
		}
	}

	triangles := int(triangleTotal)
	return finishGeometryMetadata("binary_stl", triangles*3, triangles, triangles, bounds, []string{}), nil
}

func inspectASCIISTLGeometry(path string) (GeometryMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return GeometryMetadata{}, fmt.Errorf("read ASCII STL geometry: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), scannerMaxTokenBytes)

	bounds := geometryBoundsAccumulator{}
	warnings := []string{}
	vertexCount := 0
	facetCount := 0

	for scanner.Scan() {
		fields := strings.Fields(strings.TrimSpace(scanner.Text()))
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "facet":
			facetCount++
		case "vertex":
			if len(fields) < 4 {
				warnings = appendGeometryWarning(warnings, "ASCII STL vertex line has fewer than three coordinates")
				continue
			}
			vertex, err := parseGeometryVector(fields[1:4])
			if err != nil {
				warnings = appendGeometryWarning(warnings, fmt.Sprintf("ASCII STL vertex line is invalid: %v", err))
				continue
			}
			bounds.Add(vertex)
			vertexCount++
		}
	}
	if err := scanner.Err(); err != nil {
		return GeometryMetadata{}, fmt.Errorf("scan ASCII STL geometry: %w", err)
	}

	triangleCount := vertexCount / 3
	if vertexCount%3 != 0 {
		warnings = appendGeometryWarning(warnings, "ASCII STL vertex count is not divisible by three")
	}
	if facetCount > 0 && facetCount != triangleCount {
		warnings = appendGeometryWarning(warnings, "ASCII STL facet count does not match vertex triples")
	}

	return finishGeometryMetadata("ascii_stl", vertexCount, facetCount, triangleCount, bounds, warnings), nil
}

func inspectGLBGeometry(path string) (GeometryMetadata, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return GeometryMetadata{}, fmt.Errorf("read GLB geometry: %w", err)
	}

	document, binChunk, warnings, err := parseGLB(raw)
	if err != nil {
		return GeometryMetadata{}, err
	}

	bounds := geometryBoundsAccumulator{}
	vertexCount := 0
	triangleCount := 0
	seenPositionAccessors := make(map[int]struct{})

	for _, mesh := range document.Meshes {
		for _, primitive := range mesh.Primitives {
			positionAccessorIndex, ok := primitive.Attributes["POSITION"]
			if !ok {
				warnings = appendGeometryWarning(warnings, "GLB primitive does not include POSITION attribute")
				continue
			}

			if _, seen := seenPositionAccessors[positionAccessorIndex]; !seen {
				count, err := addGLBPositionAccessorBounds(document, binChunk, positionAccessorIndex, &bounds)
				if err != nil {
					warnings = appendGeometryWarning(warnings, err.Error())
				} else {
					vertexCount += count
					seenPositionAccessors[positionAccessorIndex] = struct{}{}
				}
			}

			count, err := glbPrimitiveTriangleCount(document, primitive)
			if err != nil {
				warnings = appendGeometryWarning(warnings, err.Error())
				continue
			}
			triangleCount += count
		}
	}

	return finishGeometryMetadata("glb", vertexCount, triangleCount, triangleCount, bounds, warnings), nil
}

func finishGeometryMetadata(
	source string,
	vertexCount int,
	faceCount int,
	triangleCount int,
	bounds geometryBoundsAccumulator,
	warnings []string,
) GeometryMetadata {
	status := "parsed"
	if !bounds.HasValues() {
		status = "empty"
		warnings = appendGeometryWarning(warnings, "no vertices were found")
	}

	return GeometryMetadata{
		Status:        status,
		Source:        source,
		Bounds:        bounds.Bounds(),
		VertexCount:   vertexCount,
		FaceCount:     faceCount,
		TriangleCount: triangleCount,
		Warnings:      warnings,
	}
}

type geometryBoundsAccumulator struct {
	min Vector3
	max Vector3
	has bool
}

func (bounds *geometryBoundsAccumulator) Add(vector Vector3) {
	if !bounds.has {
		bounds.min = vector
		bounds.max = vector
		bounds.has = true
		return
	}
	for index := 0; index < 3; index++ {
		bounds.min[index] = math.Min(bounds.min[index], vector[index])
		bounds.max[index] = math.Max(bounds.max[index], vector[index])
	}
}

func (bounds geometryBoundsAccumulator) HasValues() bool {
	return bounds.has
}

func (bounds geometryBoundsAccumulator) Bounds() *GeometryBounds {
	if !bounds.has {
		return nil
	}

	size := Vector3{
		roundGeometryValue(bounds.max[0] - bounds.min[0]),
		roundGeometryValue(bounds.max[1] - bounds.min[1]),
		roundGeometryValue(bounds.max[2] - bounds.min[2]),
	}
	center := Vector3{
		roundGeometryValue((bounds.min[0] + bounds.max[0]) / 2),
		roundGeometryValue((bounds.min[1] + bounds.max[1]) / 2),
		roundGeometryValue((bounds.min[2] + bounds.max[2]) / 2),
	}

	return &GeometryBounds{
		Min:    roundGeometryVector(bounds.min),
		Max:    roundGeometryVector(bounds.max),
		Size:   size,
		Center: center,
	}
}

type glbDocument struct {
	Accessors   []glbAccessor   `json:"accessors"`
	BufferViews []glbBufferView `json:"bufferViews"`
	Meshes      []glbMesh       `json:"meshes"`
}

type glbAccessor struct {
	BufferView    *int      `json:"bufferView"`
	ByteOffset    int       `json:"byteOffset"`
	ComponentType int       `json:"componentType"`
	Count         int       `json:"count"`
	Type          string    `json:"type"`
	Min           []float64 `json:"min"`
	Max           []float64 `json:"max"`
}

type glbBufferView struct {
	Buffer     int `json:"buffer"`
	ByteOffset int `json:"byteOffset"`
	ByteLength int `json:"byteLength"`
	ByteStride int `json:"byteStride"`
}

type glbMesh struct {
	Primitives []glbPrimitive `json:"primitives"`
}

type glbPrimitive struct {
	Attributes map[string]int `json:"attributes"`
	Indices    *int           `json:"indices"`
	Mode       *int           `json:"mode"`
}

const (
	glbMagic     = 0x46546c67
	glbVersion   = 2
	glbJSONChunk = 0x4e4f534a
	glbBINChunk  = 0x004e4942
)

func parseGLB(raw []byte) (glbDocument, []byte, []string, error) {
	if len(raw) < 12 {
		return glbDocument{}, nil, nil, fmt.Errorf("read GLB geometry: file is shorter than header")
	}
	if binary.LittleEndian.Uint32(raw[0:4]) != glbMagic {
		return glbDocument{}, nil, nil, fmt.Errorf("read GLB geometry: invalid magic")
	}
	if version := binary.LittleEndian.Uint32(raw[4:8]); version != glbVersion {
		return glbDocument{}, nil, nil, fmt.Errorf("read GLB geometry: unsupported version %d", version)
	}

	totalLength := int(binary.LittleEndian.Uint32(raw[8:12]))
	if totalLength > len(raw) || totalLength < 12 {
		return glbDocument{}, nil, nil, fmt.Errorf("read GLB geometry: invalid declared length")
	}

	var jsonChunk []byte
	var binChunk []byte
	warnings := []string{}
	offset := 12
	for offset+8 <= totalLength {
		chunkLength := int(binary.LittleEndian.Uint32(raw[offset : offset+4]))
		chunkType := binary.LittleEndian.Uint32(raw[offset+4 : offset+8])
		offset += 8
		if chunkLength > totalLength-offset {
			return glbDocument{}, nil, nil, fmt.Errorf("read GLB geometry: invalid chunk length")
		}

		chunk := raw[offset : offset+chunkLength]
		switch chunkType {
		case glbJSONChunk:
			if jsonChunk == nil {
				jsonChunk = bytes.TrimRight(chunk, "\x00 \n\r\t")
			}
		case glbBINChunk:
			if binChunk == nil {
				binChunk = chunk
			}
		default:
			warnings = appendGeometryWarning(warnings, fmt.Sprintf("GLB contains unsupported chunk type %d", chunkType))
		}
		offset += chunkLength
	}

	if jsonChunk == nil {
		return glbDocument{}, nil, nil, fmt.Errorf("read GLB geometry: JSON chunk is missing")
	}

	var document glbDocument
	if err := json.Unmarshal(jsonChunk, &document); err != nil {
		return glbDocument{}, nil, nil, fmt.Errorf("parse GLB JSON chunk: %w", err)
	}

	return document, binChunk, warnings, nil
}

func addGLBPositionAccessorBounds(document glbDocument, binChunk []byte, accessorIndex int, bounds *geometryBoundsAccumulator) (int, error) {
	accessor, err := glbAccessorAt(document, accessorIndex)
	if err != nil {
		return 0, err
	}
	if accessor.ComponentType != 5126 || accessor.Type != "VEC3" {
		return 0, fmt.Errorf("GLB POSITION accessor %d must be FLOAT VEC3", accessorIndex)
	}
	if accessor.Count <= 0 {
		return 0, fmt.Errorf("GLB POSITION accessor %d has no vertices", accessorIndex)
	}

	if len(accessor.Min) >= 3 && len(accessor.Max) >= 3 {
		bounds.Add(Vector3{accessor.Min[0], accessor.Min[1], accessor.Min[2]})
		bounds.Add(Vector3{accessor.Max[0], accessor.Max[1], accessor.Max[2]})
		return accessor.Count, nil
	}

	if accessor.BufferView == nil {
		return 0, fmt.Errorf("GLB POSITION accessor %d has no bufferView and no min/max bounds", accessorIndex)
	}
	if len(binChunk) == 0 {
		return 0, fmt.Errorf("GLB POSITION accessor %d requires a BIN chunk", accessorIndex)
	}

	bufferView, err := glbBufferViewAt(document, *accessor.BufferView)
	if err != nil {
		return 0, err
	}
	if bufferView.Buffer != 0 {
		return 0, fmt.Errorf("GLB POSITION accessor %d references unsupported buffer %d", accessorIndex, bufferView.Buffer)
	}

	stride := bufferView.ByteStride
	if stride == 0 {
		stride = 12
	}
	if stride < 12 {
		return 0, fmt.Errorf("GLB POSITION accessor %d has byteStride smaller than VEC3 FLOAT", accessorIndex)
	}

	start := bufferView.ByteOffset + accessor.ByteOffset
	viewEnd := bufferView.ByteOffset + bufferView.ByteLength
	if start < 0 || viewEnd > len(binChunk) || start > viewEnd {
		return 0, fmt.Errorf("GLB POSITION accessor %d has invalid buffer range", accessorIndex)
	}

	for index := 0; index < accessor.Count; index++ {
		positionOffset := start + index*stride
		if positionOffset+12 > viewEnd || positionOffset+12 > len(binChunk) {
			return 0, fmt.Errorf("GLB POSITION accessor %d reads past bufferView", accessorIndex)
		}
		bounds.Add(Vector3{
			float64(math.Float32frombits(binary.LittleEndian.Uint32(binChunk[positionOffset : positionOffset+4]))),
			float64(math.Float32frombits(binary.LittleEndian.Uint32(binChunk[positionOffset+4 : positionOffset+8]))),
			float64(math.Float32frombits(binary.LittleEndian.Uint32(binChunk[positionOffset+8 : positionOffset+12]))),
		})
	}

	return accessor.Count, nil
}

func glbPrimitiveTriangleCount(document glbDocument, primitive glbPrimitive) (int, error) {
	vertexCount := 0
	if primitive.Indices != nil {
		accessor, err := glbAccessorAt(document, *primitive.Indices)
		if err != nil {
			return 0, err
		}
		vertexCount = accessor.Count
	} else if positionAccessorIndex, ok := primitive.Attributes["POSITION"]; ok {
		accessor, err := glbAccessorAt(document, positionAccessorIndex)
		if err != nil {
			return 0, err
		}
		vertexCount = accessor.Count
	}

	mode := 4
	if primitive.Mode != nil {
		mode = *primitive.Mode
	}

	switch mode {
	case 4:
		if vertexCount%3 != 0 {
			return vertexCount / 3, fmt.Errorf("GLB triangle primitive count is not divisible by three")
		}
		return vertexCount / 3, nil
	case 5, 6:
		return int(math.Max(0, float64(vertexCount-2))), nil
	default:
		return 0, fmt.Errorf("GLB primitive mode %d is not supported for triangle counts", mode)
	}
}

func glbAccessorAt(document glbDocument, index int) (glbAccessor, error) {
	if index < 0 || index >= len(document.Accessors) {
		return glbAccessor{}, fmt.Errorf("GLB accessor %d is out of range", index)
	}
	return document.Accessors[index], nil
}

func glbBufferViewAt(document glbDocument, index int) (glbBufferView, error) {
	if index < 0 || index >= len(document.BufferViews) {
		return glbBufferView{}, fmt.Errorf("GLB bufferView %d is out of range", index)
	}
	return document.BufferViews[index], nil
}

func parseGeometryVector(values []string) (Vector3, error) {
	if len(values) < 3 {
		return Vector3{}, fmt.Errorf("need 3 values")
	}

	x, err := strconv.ParseFloat(values[0], 64)
	if err != nil {
		return Vector3{}, err
	}
	y, err := strconv.ParseFloat(values[1], 64)
	if err != nil {
		return Vector3{}, err
	}
	z, err := strconv.ParseFloat(values[2], 64)
	if err != nil {
		return Vector3{}, err
	}

	return Vector3{x, y, z}, nil
}

func float32GeometryVector(x float32, y float32, z float32) Vector3 {
	return Vector3{float64(x), float64(y), float64(z)}
}

func roundGeometryVector(vector Vector3) Vector3 {
	return Vector3{
		roundGeometryValue(vector[0]),
		roundGeometryValue(vector[1]),
		roundGeometryValue(vector[2]),
	}
}

func roundGeometryValue(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func appendGeometryWarning(warnings []string, warning string) []string {
	if len(warnings) >= 6 {
		return warnings
	}
	return append(warnings, warning)
}

func binarySTLTriangleCount(path string) (uint32, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, false, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return 0, false, err
	}
	if info.Size() < 84 {
		return 0, false, nil
	}

	var header [84]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		return 0, false, err
	}
	triangleCount := binary.LittleEndian.Uint32(header[80:84])
	expectedSize := int64(84) + int64(triangleCount)*50
	return triangleCount, expectedSize == info.Size(), nil
}
