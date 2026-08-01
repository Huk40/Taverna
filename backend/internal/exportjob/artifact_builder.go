package exportjob

import (
	"bufio"
	"fmt"
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
	if format != "obj" {
		return ArtifactResult{
			Kind:        "manifest_only",
			Format:      format,
			ContentType: "application/json",
			URL:         "",
			Warnings:    []string{"mesh artifact generation currently supports OBJ export only"},
		}, nil
	}

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
