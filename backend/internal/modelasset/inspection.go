package modelasset

import (
	"bufio"
	"encoding/binary"
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
	case FormatGLB, FormatGLTF, Format3MF:
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
