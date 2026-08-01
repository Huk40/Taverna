package exportjob

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"taverna/backend/internal/assembly"
	"taverna/backend/internal/modelasset"
)

type ArtifactBuilder struct {
	storageDir string
}

type ArtifactResult struct {
	Kind           string   `json:"kind"`
	FileName       string   `json:"file_name"`
	Format         string   `json:"format"`
	ContentType    string   `json:"content_type"`
	SizeBytes      int64    `json:"size_bytes"`
	StoragePath    string   `json:"storage_path"`
	URL            string   `json:"url"`
	IncludedAssets []string `json:"included_assets"`
	SkippedAssets  []string `json:"skipped_assets"`
	Warnings       []string `json:"warnings"`
}

func NewArtifactBuilder(storageDir string) *ArtifactBuilder {
	return &ArtifactBuilder{storageDir: storageDir}
}

func (b *ArtifactBuilder) Build(jobID string, manifest assembly.Manifest, format string) (ArtifactResult, error) {
	switch format {
	case "obj":
		return b.buildOBJ(jobID, manifest)
	case "stl":
		return b.buildSTL(jobID, manifest)
	default:
		return ArtifactResult{
			Kind:        "manifest_only",
			Format:      format,
			ContentType: "application/json",
			URL:         "",
			Warnings:    []string{"mesh artifact generation currently supports OBJ and STL export only"},
		}, nil
	}
}

func (b *ArtifactBuilder) buildOBJ(jobID string, manifest assembly.Manifest) (ArtifactResult, error) {
	if err := os.MkdirAll(b.storageDir, 0o755); err != nil {
		return ArtifactResult{}, fmt.Errorf("create export artifact storage: %w", err)
	}

	fileName := jobID + ".obj"
	storagePath := filepath.Join(b.storageDir, fileName)
	file, err := os.OpenFile(storagePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return ArtifactResult{}, fmt.Errorf("create OBJ artifact: %w", err)
	}

	writer := bufio.NewWriter(file)
	result := ArtifactResult{
		Kind:           "assembled_obj",
		FileName:       fileName,
		Format:         "obj",
		ContentType:    "text/plain; charset=utf-8",
		StoragePath:    storagePath,
		URL:            "/api/exports/" + jobID + "/artifact",
		IncludedAssets: []string{},
		SkippedAssets:  []string{},
		Warnings:       []string{},
	}
	state := objWriteState{}

	if _, err := fmt.Fprintf(writer, "# Taverna assembled OBJ artifact\n# project: %s\n# placements: %d\n\n", manifest.ProjectID, len(manifest.Placements)); err != nil {
		_ = file.Close()
		return ArtifactResult{}, fmt.Errorf("write OBJ header: %w", err)
	}

	for _, placement := range manifest.Placements {
		if placement.Asset.Format != modelasset.FormatOBJ {
			result.SkippedAssets = append(result.SkippedAssets, placement.Asset.ID)
			result.Warnings = append(result.Warnings, fmt.Sprintf("asset %s has format %s and was skipped by OBJ assembler", placement.Asset.ID, placement.Asset.Format))
			continue
		}
		if strings.TrimSpace(placement.Asset.StoragePath) == "" {
			result.SkippedAssets = append(result.SkippedAssets, placement.Asset.ID)
			result.Warnings = append(result.Warnings, fmt.Sprintf("asset %s has no storage path", placement.Asset.ID))
			continue
		}
		if err := appendOBJPlacement(writer, &state, placement); err != nil {
			result.SkippedAssets = append(result.SkippedAssets, placement.Asset.ID)
			result.Warnings = append(result.Warnings, err.Error())
			continue
		}
		result.IncludedAssets = append(result.IncludedAssets, placement.Asset.ID)
	}

	if len(result.IncludedAssets) == 0 {
		result.Warnings = append(result.Warnings, "no OBJ assets were included in the assembled artifact")
	}

	if err := writer.Flush(); err != nil {
		_ = file.Close()
		return ArtifactResult{}, fmt.Errorf("flush OBJ artifact: %w", err)
	}
	if err := file.Close(); err != nil {
		return ArtifactResult{}, fmt.Errorf("close OBJ artifact: %w", err)
	}

	info, err := os.Stat(storagePath)
	if err != nil {
		return ArtifactResult{}, fmt.Errorf("stat OBJ artifact: %w", err)
	}
	result.SizeBytes = info.Size()

	return result, nil
}

func (b *ArtifactBuilder) buildSTL(jobID string, manifest assembly.Manifest) (ArtifactResult, error) {
	if err := os.MkdirAll(b.storageDir, 0o755); err != nil {
		return ArtifactResult{}, fmt.Errorf("create export artifact storage: %w", err)
	}

	fileName := jobID + ".stl"
	storagePath := filepath.Join(b.storageDir, fileName)
	file, err := os.OpenFile(storagePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return ArtifactResult{}, fmt.Errorf("create STL artifact: %w", err)
	}

	writer := bufio.NewWriter(file)
	result := ArtifactResult{
		Kind:           "assembled_stl",
		FileName:       fileName,
		Format:         "stl",
		ContentType:    "model/stl",
		StoragePath:    storagePath,
		URL:            "/api/exports/" + jobID + "/artifact",
		IncludedAssets: []string{},
		SkippedAssets:  []string{},
		Warnings:       []string{},
	}
	solidName := sanitizeOBJName("taverna_" + jobID)

	if _, err := fmt.Fprintf(writer, "solid %s\n", solidName); err != nil {
		_ = file.Close()
		return ArtifactResult{}, fmt.Errorf("write STL header: %w", err)
	}

	for _, placement := range manifest.Placements {
		if placement.Asset.Format != modelasset.FormatOBJ && placement.Asset.Format != modelasset.FormatSTL {
			result.SkippedAssets = append(result.SkippedAssets, placement.Asset.ID)
			result.Warnings = append(result.Warnings, fmt.Sprintf("asset %s has format %s and was skipped by STL assembler", placement.Asset.ID, placement.Asset.Format))
			continue
		}
		if strings.TrimSpace(placement.Asset.StoragePath) == "" {
			result.SkippedAssets = append(result.SkippedAssets, placement.Asset.ID)
			result.Warnings = append(result.Warnings, fmt.Sprintf("asset %s has no storage path", placement.Asset.ID))
			continue
		}
		triangleCount, err := appendSTLPlacement(writer, placement)
		if err != nil {
			result.SkippedAssets = append(result.SkippedAssets, placement.Asset.ID)
			result.Warnings = append(result.Warnings, err.Error())
			continue
		}
		if triangleCount == 0 {
			result.SkippedAssets = append(result.SkippedAssets, placement.Asset.ID)
			result.Warnings = append(result.Warnings, fmt.Sprintf("asset %s did not contain triangles", placement.Asset.ID))
			continue
		}
		result.IncludedAssets = append(result.IncludedAssets, placement.Asset.ID)
	}

	if len(result.IncludedAssets) == 0 {
		result.Warnings = append(result.Warnings, "no OBJ or ASCII STL assets were included in the assembled artifact")
	}

	if _, err := fmt.Fprintf(writer, "endsolid %s\n", solidName); err != nil {
		_ = file.Close()
		return ArtifactResult{}, fmt.Errorf("write STL footer: %w", err)
	}
	if err := writer.Flush(); err != nil {
		_ = file.Close()
		return ArtifactResult{}, fmt.Errorf("flush STL artifact: %w", err)
	}
	if err := file.Close(); err != nil {
		return ArtifactResult{}, fmt.Errorf("close STL artifact: %w", err)
	}

	info, err := os.Stat(storagePath)
	if err != nil {
		return ArtifactResult{}, fmt.Errorf("stat STL artifact: %w", err)
	}
	result.SizeBytes = info.Size()

	return result, nil
}

type objWriteState struct {
	vertexOffset  int
	textureOffset int
	normalOffset  int
}

func appendOBJPlacement(writer *bufio.Writer, state *objWriteState, placement assembly.ResolvedPlacement) error {
	source, err := os.Open(placement.Asset.StoragePath)
	if err != nil {
		return fmt.Errorf("read asset %s: %w", placement.Asset.ID, err)
	}
	defer source.Close()

	localVertices := 0
	localTextures := 0
	localNormals := 0

	if _, err := fmt.Fprintf(writer, "o %s_%s\n", sanitizeOBJName(placement.ID), sanitizeOBJName(placement.Asset.ID)); err != nil {
		return err
	}

	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case "v":
			vertex, err := parseVector(fields[1:])
			if err != nil {
				return fmt.Errorf("asset %s has invalid vertex: %w", placement.Asset.ID, err)
			}
			vertex = applyTransform(vertex, placement.WorldTransform, true)
			if _, err := fmt.Fprintf(writer, "v %.6f %.6f %.6f\n", vertex[0], vertex[1], vertex[2]); err != nil {
				return err
			}
			localVertices++
		case "vt":
			if _, err := fmt.Fprintf(writer, "%s\n", line); err != nil {
				return err
			}
			localTextures++
		case "vn":
			normal, err := parseVector(fields[1:])
			if err != nil {
				return fmt.Errorf("asset %s has invalid normal: %w", placement.Asset.ID, err)
			}
			normal = normalizeVector(applyTransform(normal, placement.WorldTransform, false))
			if _, err := fmt.Fprintf(writer, "vn %.6f %.6f %.6f\n", normal[0], normal[1], normal[2]); err != nil {
				return err
			}
			localNormals++
		case "f":
			face := make([]string, 0, len(fields)-1)
			for _, token := range fields[1:] {
				shifted, err := shiftFaceToken(token, state)
				if err != nil {
					return fmt.Errorf("asset %s has unsupported face token %q: %w", placement.Asset.ID, token, err)
				}
				face = append(face, shifted)
			}
			if _, err := fmt.Fprintf(writer, "f %s\n", strings.Join(face, " ")); err != nil {
				return err
			}
		case "g", "s", "usemtl", "mtllib":
			// Material references are intentionally not copied in the MVP OBJ assembler.
			continue
		default:
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan asset %s OBJ: %w", placement.Asset.ID, err)
	}

	state.vertexOffset += localVertices
	state.textureOffset += localTextures
	state.normalOffset += localNormals
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	return nil
}

func appendSTLPlacement(writer *bufio.Writer, placement assembly.ResolvedPlacement) (int, error) {
	switch placement.Asset.Format {
	case modelasset.FormatOBJ:
		return appendOBJAsSTL(writer, placement)
	case modelasset.FormatSTL:
		return appendSTLFilePlacement(writer, placement)
	default:
		return 0, fmt.Errorf("asset %s has unsupported STL assembly format %s", placement.Asset.ID, placement.Asset.Format)
	}
}

func appendSTLFilePlacement(writer *bufio.Writer, placement assembly.ResolvedPlacement) (int, error) {
	triangleCount, isBinary, err := binarySTLTriangleCount(placement.Asset.StoragePath)
	if err != nil {
		return 0, fmt.Errorf("inspect asset %s STL: %w", placement.Asset.ID, err)
	}
	if isBinary {
		return appendBinarySTLPlacement(writer, placement, triangleCount)
	}
	return appendASCIISTLPlacement(writer, placement)
}

func appendOBJAsSTL(writer *bufio.Writer, placement assembly.ResolvedPlacement) (int, error) {
	source, err := os.Open(placement.Asset.StoragePath)
	if err != nil {
		return 0, fmt.Errorf("read asset %s: %w", placement.Asset.ID, err)
	}
	defer source.Close()

	vertices := make([][3]float64, 0)
	triangleCount := 0

	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case "v":
			vertex, err := parseVector(fields[1:])
			if err != nil {
				return triangleCount, fmt.Errorf("asset %s has invalid OBJ vertex: %w", placement.Asset.ID, err)
			}
			vertices = append(vertices, applyTransform(vertex, placement.WorldTransform, true))
		case "f":
			if len(fields) < 4 {
				return triangleCount, fmt.Errorf("asset %s has OBJ face with fewer than 3 vertices", placement.Asset.ID)
			}
			indices := make([]int, 0, len(fields)-1)
			for _, token := range fields[1:] {
				index, err := parseOBJVertexIndex(token, len(vertices))
				if err != nil {
					return triangleCount, fmt.Errorf("asset %s has unsupported OBJ face token %q: %w", placement.Asset.ID, token, err)
				}
				indices = append(indices, index)
			}
			for index := 1; index < len(indices)-1; index++ {
				if err := writeSTLFacet(writer, vertices[indices[0]], vertices[indices[index]], vertices[indices[index+1]]); err != nil {
					return triangleCount, err
				}
				triangleCount++
			}
		default:
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		return triangleCount, fmt.Errorf("scan asset %s OBJ: %w", placement.Asset.ID, err)
	}

	return triangleCount, nil
}

func appendASCIISTLPlacement(writer *bufio.Writer, placement assembly.ResolvedPlacement) (int, error) {
	source, err := os.Open(placement.Asset.StoragePath)
	if err != nil {
		return 0, fmt.Errorf("read asset %s: %w", placement.Asset.ID, err)
	}
	defer source.Close()

	triangle := make([][3]float64, 0, 3)
	triangleCount := 0

	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "vertex" {
			continue
		}

		vertex, err := parseVector(fields[1:])
		if err != nil {
			return triangleCount, fmt.Errorf("asset %s has invalid STL vertex: %w", placement.Asset.ID, err)
		}
		triangle = append(triangle, applyTransform(vertex, placement.WorldTransform, true))
		if len(triangle) == 3 {
			if err := writeSTLFacet(writer, triangle[0], triangle[1], triangle[2]); err != nil {
				return triangleCount, err
			}
			triangleCount++
			triangle = triangle[:0]
		}
	}
	if err := scanner.Err(); err != nil {
		return triangleCount, fmt.Errorf("scan asset %s STL: %w", placement.Asset.ID, err)
	}
	if len(triangle) != 0 {
		return triangleCount, fmt.Errorf("asset %s has incomplete ASCII STL triangle", placement.Asset.ID)
	}
	if triangleCount == 0 {
		return 0, fmt.Errorf("asset %s has no ASCII STL triangles", placement.Asset.ID)
	}

	return triangleCount, nil
}

func appendBinarySTLPlacement(writer *bufio.Writer, placement assembly.ResolvedPlacement, triangleTotal uint32) (int, error) {
	source, err := os.Open(placement.Asset.StoragePath)
	if err != nil {
		return 0, fmt.Errorf("read asset %s: %w", placement.Asset.ID, err)
	}
	defer source.Close()

	if _, err := source.Seek(84, io.SeekStart); err != nil {
		return 0, fmt.Errorf("seek asset %s binary STL: %w", placement.Asset.ID, err)
	}

	triangleCount := 0
	for index := uint32(0); index < triangleTotal; index++ {
		var values [12]float32
		if err := binary.Read(source, binary.LittleEndian, &values); err != nil {
			return triangleCount, fmt.Errorf("read asset %s binary STL triangle %d: %w", placement.Asset.ID, index, err)
		}

		a := applyTransform(float32Vector(values[3], values[4], values[5]), placement.WorldTransform, true)
		b := applyTransform(float32Vector(values[6], values[7], values[8]), placement.WorldTransform, true)
		c := applyTransform(float32Vector(values[9], values[10], values[11]), placement.WorldTransform, true)
		if err := writeSTLFacet(writer, a, b, c); err != nil {
			return triangleCount, err
		}

		var attribute uint16
		if err := binary.Read(source, binary.LittleEndian, &attribute); err != nil {
			return triangleCount, fmt.Errorf("read asset %s binary STL attribute %d: %w", placement.Asset.ID, index, err)
		}
		triangleCount++
	}

	return triangleCount, nil
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

func writeSTLFacet(writer *bufio.Writer, a [3]float64, b [3]float64, c [3]float64) error {
	normal := triangleNormal(a, b, c)
	if _, err := fmt.Fprintf(writer, "  facet normal %.6f %.6f %.6f\n", normal[0], normal[1], normal[2]); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, "    outer loop"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "      vertex %.6f %.6f %.6f\n", a[0], a[1], a[2]); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "      vertex %.6f %.6f %.6f\n", b[0], b[1], b[2]); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "      vertex %.6f %.6f %.6f\n", c[0], c[1], c[2]); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, "    endloop"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, "  endfacet"); err != nil {
		return err
	}
	return nil
}

func float32Vector(x float32, y float32, z float32) [3]float64 {
	return [3]float64{float64(x), float64(y), float64(z)}
}

func parseVector(values []string) ([3]float64, error) {
	if len(values) < 3 {
		return [3]float64{}, fmt.Errorf("need 3 values")
	}
	var vector [3]float64
	for index := 0; index < 3; index++ {
		value, err := strconv.ParseFloat(values[index], 64)
		if err != nil {
			return [3]float64{}, err
		}
		vector[index] = value
	}
	return vector, nil
}

func applyTransform(vector [3]float64, transform assembly.Transform, translate bool) [3]float64 {
	x := vector[0] * transform.Scale
	y := vector[1] * transform.Scale
	z := vector[2] * transform.Scale

	x, y, z = rotateX(x, y, z, degreesToRadians(transform.Rotation[0]))
	x, y, z = rotateY(x, y, z, degreesToRadians(transform.Rotation[1]))
	x, y, z = rotateZ(x, y, z, degreesToRadians(transform.Rotation[2]))

	if translate {
		x += transform.Position[0]
		y += transform.Position[1]
		z += transform.Position[2]
	}

	return [3]float64{x, y, z}
}

func rotateX(x float64, y float64, z float64, angle float64) (float64, float64, float64) {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	return x, y*cos - z*sin, y*sin + z*cos
}

func rotateY(x float64, y float64, z float64, angle float64) (float64, float64, float64) {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	return x*cos + z*sin, y, -x*sin + z*cos
}

func rotateZ(x float64, y float64, z float64, angle float64) (float64, float64, float64) {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	return x*cos - y*sin, x*sin + y*cos, z
}

func degreesToRadians(value float64) float64 {
	return value * math.Pi / 180
}

func normalizeVector(vector [3]float64) [3]float64 {
	length := math.Sqrt(vector[0]*vector[0] + vector[1]*vector[1] + vector[2]*vector[2])
	if length == 0 {
		return vector
	}
	return [3]float64{vector[0] / length, vector[1] / length, vector[2] / length}
}

func triangleNormal(a [3]float64, b [3]float64, c [3]float64) [3]float64 {
	edgeAB := [3]float64{b[0] - a[0], b[1] - a[1], b[2] - a[2]}
	edgeAC := [3]float64{c[0] - a[0], c[1] - a[1], c[2] - a[2]}
	return normalizeVector([3]float64{
		edgeAB[1]*edgeAC[2] - edgeAB[2]*edgeAC[1],
		edgeAB[2]*edgeAC[0] - edgeAB[0]*edgeAC[2],
		edgeAB[0]*edgeAC[1] - edgeAB[1]*edgeAC[0],
	})
}

func parseOBJVertexIndex(token string, vertexCount int) (int, error) {
	parts := strings.Split(token, "/")
	if len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("missing vertex index")
	}
	value, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	if value == 0 {
		return 0, fmt.Errorf("OBJ indices are 1-based")
	}
	if value < 0 {
		value = vertexCount + value + 1
	}
	index := value - 1
	if index < 0 || index >= vertexCount {
		return 0, fmt.Errorf("vertex index %d is out of range", value)
	}
	return index, nil
}

func shiftFaceToken(token string, state *objWriteState) (string, error) {
	parts := strings.Split(token, "/")
	if len(parts) == 0 || len(parts) > 3 {
		return "", fmt.Errorf("expected v, v/vt, v//vn, or v/vt/vn")
	}

	offsets := []int{state.vertexOffset, state.textureOffset, state.normalOffset}
	for index, part := range parts {
		if part == "" {
			continue
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return "", err
		}
		if value < 0 {
			return "", fmt.Errorf("negative indices are not supported")
		}
		parts[index] = strconv.Itoa(value + offsets[index])
	}
	return strings.Join(parts, "/"), nil
}

func sanitizeOBJName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unnamed"
	}
	var builder strings.Builder
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			builder.WriteRune(char)
			continue
		}
		builder.WriteByte('_')
	}
	return builder.String()
}
