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

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes)
	if err := r.ParseMultipartForm(h.maxUploadBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "multipart payload is required and must fit upload limit"})
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

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
	if header.Size <= 0 || header.Size > h.maxUploadBytes {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file size is outside allowed range"})
		return
	}

	slot := strings.TrimSpace(r.FormValue("slot"))
	attachmentPoint := strings.TrimSpace(r.FormValue("attachment_point"))
	if !catalog.IsAssemblyPointTarget(slot, attachmentPoint) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slot and attachment_point are not a supported assembly target"})
		return
	}

	points, err := decodeAssemblyPoints(r.FormValue("assembly_points"), attachmentPoint)
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

	w.Header().Set("Content-Type", asset.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", asset.FileName))
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

func decodeAssemblyPoints(raw string, fallbackPoint string) ([]AssemblyPoint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []AssemblyPoint{{
			ID:       fallbackPoint,
			Label:    fallbackPoint,
			Position: Vector3{0, 0, 0},
			Rotation: Vector3{0, 0, 0},
		}}, nil
	}

	var points []AssemblyPoint
	if err := json.Unmarshal([]byte(raw), &points); err != nil {
		return nil, errors.New("assembly_points must be valid JSON")
	}
	if len(points) == 0 {
		return nil, errors.New("assembly_points must include at least one point")
	}
	if len(points) > 16 {
		return nil, errors.New("assembly_points cannot exceed 16 items")
	}
	for index, point := range points {
		if strings.TrimSpace(point.ID) == "" {
			return nil, fmt.Errorf("assembly_points[%d].id is required", index)
		}
		if strings.TrimSpace(point.Label) == "" {
			points[index].Label = point.ID
		}
	}
	return points, nil
}

func formatForFileName(fileName string) (string, bool) {
	extension := strings.ToLower(filepath.Ext(fileName))
	format, ok := allowedFormats[extension]
	return format, ok
}

func contentTypeFor(value string, fileName string) string {
	value = strings.TrimSpace(value)
	if value != "" && value != "application/octet-stream" {
		return value
	}
	if detected := mime.TypeByExtension(filepath.Ext(fileName)); detected != "" {
		return detected
	}
	return "application/octet-stream"
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
