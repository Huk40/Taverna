package modelasset

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"taverna/backend/internal/catalog"
)

const defaultMaxUploadBytes int64 = 50 << 20

var allowedFormats = map[string]string{
	".glb":  FormatGLB,
	".gltf": FormatGLTF,
	".obj":  FormatOBJ,
	".stl":  FormatSTL,
	".3mf":  Format3MF,
}

type Handler struct {
	repo           *Repository
	catalog        *catalog.Service
	storageDir     string
	maxUploadBytes int64
	logger         *slog.Logger
}

func NewHandler(repo *Repository, catalogService *catalog.Service, storageDir string, maxUploadBytes int64, logger *slog.Logger) *Handler {
	if maxUploadBytes <= 0 {
		maxUploadBytes = defaultMaxUploadBytes
	}
	return &Handler{
		repo:           repo,
		catalog:        catalogService,
		storageDir:     storageDir,
		maxUploadBytes: maxUploadBytes,
		logger:         logger,
	}
}

func (h *Handler) Collection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.list(w, r)
	case http.MethodPost:
		h.upload(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) AssemblyPoints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, h.catalog.AssemblyPointTargets())
}

func (h *Handler) Item(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/model-assets/")
	path = strings.Trim(path, "/")
	if path == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "model asset not found"})
		return
	}

	if strings.HasSuffix(path, "/file") {
		id := strings.TrimSuffix(path, "/file")
		id = strings.Trim(id, "/")
		h.file(w, r, id)
		return
	}
	if strings.Contains(path, "/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "model asset not found"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.get(w, r, path)
	case http.MethodPut:
		h.update(w, r, path)
	case http.MethodDelete:
		h.delete(w, r, path)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	slot := strings.TrimSpace(r.URL.Query().Get("slot"))
	if slot != "" && !h.catalog.HasSlot(slot) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown slot"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	assets, err := h.repo.List(ctx, slot)
	if err != nil {
		h.serverError(w, "list model assets", err)
		return
	}

	writeJSON(w, http.StatusOK, assets)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request, id string) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	asset, err := h.repo.Get(ctx, id)
	if err != nil {
		h.handleRepositoryError(w, "get model asset", err)
		return
	}

	writeJSON(w, http.StatusOK, asset)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request, id string) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var payload struct {
		Label          *string          `json:"label"`
		AssemblyPoints *[]AssemblyPoint `json:"assembly_points"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid model asset update payload"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	current, err := h.repo.Get(ctx, id)
	if err != nil {
		h.handleRepositoryError(w, "get model asset before update", err)
		return
	}

	label := current.Label
	if payload.Label != nil {
		label = strings.TrimSpace(*payload.Label)
		if label == "" {
			label = current.FileName
		}
	}

	points := current.AssemblyPoints
	if payload.AssemblyPoints != nil {
		points, err = normalizeAssemblyPoints(*payload.AssemblyPoints)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		references, err := h.repo.MissingAnchorReferences(ctx, id, assemblyPointIDs(points))
		if err != nil {
			h.serverError(w, "check model asset anchor project references", err)
			return
		}
		if len(references) > 0 {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":      "model asset anchors are used by saved projects",
				"references": references,
			})
			return
		}
	}

	asset, err := h.repo.Update(ctx, id, UpdateRequest{
		Label:          label,
		AssemblyPoints: points,
	})
	if err != nil {
		h.handleRepositoryError(w, "update model asset", err)
		return
	}

	writeJSON(w, http.StatusOK, asset)
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	// Reserve space for multipart fields/boundaries; the limit applies to the file.
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes+(1<<20))
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "Размер загрузки превышает лимит сервера"})
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "multipart payload is required"})
		}
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file field is required"})
		return
	}
	defer file.Close()

	originalName := filepath.Base(header.Filename)
	format, ok := formatForFileName(originalName)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file must be one of: .glb, .gltf, .obj, .stl, .3mf"})
		return
	}
	if header.Size > h.maxUploadBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "Размер файла превышает лимит сервера"})
		return
	}
	if header.Size <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file size is outside allowed range"})
		return
	}

	slot := strings.TrimSpace(r.FormValue("slot"))
	attachmentPoint := strings.TrimSpace(r.FormValue("attachment_point"))
	if !catalog.IsAssemblyPointTarget(slot, attachmentPoint) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slot and attachment_point are not a supported assembly target"})
		return
	}

	customPoints, hasCustomPoints, err := decodeAssemblyPoints(r.FormValue("assembly_points"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	id, err := randomUUIDLikeID()
	if err != nil {
		h.serverError(w, "generate model asset id", err)
		return
	}

	if err := os.MkdirAll(h.storageDir, 0o755); err != nil {
		h.serverError(w, "create model asset storage", err)
		return
	}

	storagePath := filepath.Join(h.storageDir, id+"."+format)
	destination, err := os.OpenFile(storagePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		h.serverError(w, "create model asset file", err)
		return
	}

	written, copyErr := io.Copy(destination, file)
	closeErr := destination.Close()
	if copyErr != nil {
		_ = os.Remove(storagePath)
		h.serverError(w, "write model asset file", copyErr)
		return
	}
	if closeErr != nil {
		_ = os.Remove(storagePath)
		h.serverError(w, "close model asset file", closeErr)
		return
	}
	if written <= 0 || written > h.maxUploadBytes {
		_ = os.Remove(storagePath)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file size is outside allowed range"})
		return
	}
	metadata := InspectGeometry(storagePath, format)
	if metadata.Status == "error" {
		h.logger.Warn("model asset geometry inspection failed", "file_name", originalName, "format", format, "warnings", metadata.Warnings)
	}
	if err := validateUploadGeometry(format, metadata); err != nil {
		_ = os.Remove(storagePath)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	points := customPoints
	if !hasCustomPoints {
		points = suggestedAssemblyPoints(metadata, slot, attachmentPoint)
	}

	request := CreateRequest{
		ID:              id,
		Label:           r.FormValue("label"),
		Slot:            slot,
		AttachmentPoint: attachmentPoint,
		FileName:        originalName,
		ContentType:     contentTypeFor(header.Header.Get("Content-Type"), originalName),
		Format:          format,
		SizeBytes:       written,
		StoragePath:     storagePath,
		AssemblyPoints:  points,
		Metadata:        metadata,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	asset, err := h.repo.Create(ctx, request)
	if err != nil {
		_ = os.Remove(storagePath)
		h.serverError(w, "create model asset", err)
		return
	}

	writeJSON(w, http.StatusCreated, asset)
}

func (h *Handler) file(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	asset, err := h.repo.Get(ctx, id)
	if err != nil {
		h.handleRepositoryError(w, "get model asset file", err)
		return
	}

	storagePath := filepath.Clean(assetStoragePath(asset))
	storageRoot := filepath.Clean(h.storageDir)
	rel, err := filepath.Rel(storageRoot, storagePath)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		h.serverError(w, "resolve model asset file", errors.New("asset path escapes storage dir"))
		return
	}

	w.Header().Set("Content-Type", contentTypeFor("", asset.FileName))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": asset.FileName}))
	http.ServeFile(w, r, storagePath)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request, id string) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	asset, err := h.repo.Get(ctx, id)
	if err != nil {
		h.handleRepositoryError(w, "get model asset before delete", err)
		return
	}

	references, err := h.repo.ReferencingProjects(ctx, id)
	if err != nil {
		h.serverError(w, "check model asset project references", err)
		return
	}
	if len(references) > 0 {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":    "model asset is used by saved projects",
			"projects": references,
		})
		return
	}

	if err := h.repo.Delete(ctx, id); err != nil {
		h.handleRepositoryError(w, "delete model asset", err)
		return
	}

	if err := os.Remove(assetStoragePath(asset)); err != nil && !errors.Is(err, os.ErrNotExist) {
		h.logger.Warn("model asset metadata deleted but file removal failed", "asset_id", id, "error", err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func assetStoragePath(asset ModelAsset) string {
	return asset.StoragePath
}

func decodeAssemblyPoints(raw string) ([]AssemblyPoint, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false, nil
	}

	var points []AssemblyPoint
	if err := json.Unmarshal([]byte(raw), &points); err != nil {
		return nil, true, errors.New("assembly_points must be valid JSON")
	}
	normalized, err := normalizeAssemblyPoints(points)
	return normalized, true, err
}

func validateUploadGeometry(format string, metadata GeometryMetadata) error {
	switch strings.TrimSpace(strings.ToLower(format)) {
	case FormatOBJ, FormatSTL, FormatGLB, FormatGLTF, Format3MF:
	default:
		return nil
	}

	if metadata.Status != "parsed" {
		return fmt.Errorf("file does not contain readable %s triangle geometry: %s", strings.ToUpper(format), firstGeometryWarning(metadata))
	}
	if metadata.Bounds == nil {
		return fmt.Errorf("file does not contain readable %s bounds", strings.ToUpper(format))
	}
	if metadata.TriangleCount <= 0 {
		return fmt.Errorf("file does not contain %s triangle faces", strings.ToUpper(format))
	}
	return nil
}

func firstGeometryWarning(metadata GeometryMetadata) string {
	for _, warning := range metadata.Warnings {
		warning = strings.TrimSpace(warning)
		if warning != "" {
			return warning
		}
	}
	if metadata.Status == "" {
		return "geometry metadata is unavailable"
	}
	return "geometry status is " + metadata.Status
}

func suggestedAssemblyPoints(metadata GeometryMetadata, slot string, fallbackPoint string) []AssemblyPoint {
	bounds := metadata.Bounds
	if metadata.Status != "parsed" || bounds == nil || !boundsFitsAssemblyPointRange(*bounds) {
		return fallbackAssemblyPoints(fallbackPoint)
	}

	min := bounds.Min
	max := bounds.Max
	center := bounds.Center
	points := make([]AssemblyPoint, 0, 8)
	addPoint := func(id string, label string, position Vector3) {
		id = strings.TrimSpace(id)
		if id == "" || len(points) >= 8 {
			return
		}
		for _, point := range points {
			if point.ID == id {
				return
			}
		}
		points = append(points, AssemblyPoint{
			ID:       id,
			Label:    label,
			Position: roundGeometryVector(position),
			Rotation: Vector3{0, 0, 0},
		})
	}

	switch slot {
	case "gear":
		addPoint("grip", "Grip", Vector3{center[0], min[1], center[2]})
	case "head":
		addPoint("neck", "Neck", Vector3{center[0], min[1], center[2]})
	case "base", "stage":
		addPoint("contact", "Contact", Vector3{center[0], min[1], center[2]})
	case "body", "clothing":
		addPoint("mount", "Mount", center)
	default:
		addPoint("origin", "Origin", Vector3{0, 0, 0})
	}

	addPoint("origin", "Origin", Vector3{0, 0, 0})
	addPoint("center", "Center", center)
	addPoint("bottom", "Bottom", Vector3{center[0], min[1], center[2]})
	addPoint("top", "Top", Vector3{center[0], max[1], center[2]})
	addPoint("left", "Left", Vector3{min[0], center[1], center[2]})
	addPoint("right", "Right", Vector3{max[0], center[1], center[2]})
	addPoint("front", "Front", Vector3{center[0], center[1], max[2]})
	addPoint("back", "Back", Vector3{center[0], center[1], min[2]})

	if len(points) == 0 {
		return fallbackAssemblyPoints(fallbackPoint)
	}
	return points
}

func fallbackAssemblyPoints(fallbackPoint string) []AssemblyPoint {
	fallbackPoint = strings.TrimSpace(fallbackPoint)
	if fallbackPoint == "" {
		fallbackPoint = "origin"
	}
	return []AssemblyPoint{{
		ID:       fallbackPoint,
		Label:    fallbackPoint,
		Position: Vector3{0, 0, 0},
		Rotation: Vector3{0, 0, 0},
	}}
}

func boundsFitsAssemblyPointRange(bounds GeometryBounds) bool {
	for _, vector := range []Vector3{bounds.Min, bounds.Max, bounds.Center} {
		if err := validateAssemblyPointVector(vector, -20, 20); err != nil {
			return false
		}
	}
	return true
}

func normalizeAssemblyPoints(points []AssemblyPoint) ([]AssemblyPoint, error) {
	if len(points) == 0 {
		return nil, errors.New("assembly_points must include at least one point")
	}
	if len(points) > 16 {
		return nil, errors.New("assembly_points cannot exceed 16 items")
	}
	seen := make(map[string]struct{}, len(points))
	for index, point := range points {
		point.ID = strings.TrimSpace(point.ID)
		point.Label = strings.TrimSpace(point.Label)
		if point.ID == "" {
			return nil, fmt.Errorf("assembly_points[%d].id is required", index)
		}
		if len(point.ID) > 80 {
			return nil, fmt.Errorf("assembly_points[%d].id is too long", index)
		}
		if _, ok := seen[point.ID]; ok {
			return nil, fmt.Errorf("duplicate assembly point %q", point.ID)
		}
		seen[point.ID] = struct{}{}
		if point.Label == "" {
			points[index].Label = point.ID
		} else {
			points[index].Label = point.Label
		}
		if err := validateAssemblyPointVector(point.Position, -20, 20); err != nil {
			return nil, fmt.Errorf("assembly_points[%d].position %v", index, err)
		}
		if err := validateAssemblyPointVector(point.Rotation, -360, 360); err != nil {
			return nil, fmt.Errorf("assembly_points[%d].rotation %v", index, err)
		}
		points[index].ID = point.ID
	}
	return points, nil
}

func validateAssemblyPointVector(vector Vector3, min float64, max float64) error {
	for _, value := range vector {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < min || value > max {
			return fmt.Errorf("values must be finite and between %.0f and %.0f", min, max)
		}
	}
	return nil
}

func assemblyPointIDs(points []AssemblyPoint) []string {
	ids := make([]string, 0, len(points))
	for _, point := range points {
		ids = append(ids, point.ID)
	}
	return ids
}

func formatForFileName(fileName string) (string, bool) {
	extension := strings.ToLower(filepath.Ext(fileName))
	format, ok := allowedFormats[extension]
	return format, ok
}

func contentTypeFor(_ string, fileName string) string {
	// Do not let a multipart Content-Type turn an uploaded model into HTML.
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".glb":
		return "model/gltf-binary"
	case ".gltf":
		return "model/gltf+json"
	case ".obj":
		return "model/obj"
	case ".stl":
		return "model/stl"
	case ".3mf":
		return "model/3mf"
	default:
		return "application/octet-stream"
	}
}

func randomUUIDLikeID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(b[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}

func (h *Handler) handleRepositoryError(w http.ResponseWriter, operation string, err error) {
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "model asset not found"})
		return
	}
	h.serverError(w, operation, err)
}

func (h *Handler) serverError(w http.ResponseWriter, operation string, err error) {
	h.logger.Error(operation+" failed", "error", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
