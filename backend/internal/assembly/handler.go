package assembly

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"taverna/backend/internal/httpx"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) ProjectAssembly(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	projectID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/projects/"), "/assembly")
	projectID = strings.Trim(projectID, "/")
	if !httpx.UUID(projectID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	manifest, err := h.service.Manifest(ctx, projectID)
	if err != nil {
		switch {
		case errors.Is(err, ErrProjectNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		case errors.Is(err, ErrInvalidAssembly):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			h.serverError(w, "build assembly manifest", err)
		}
		return
	}

	writeJSON(w, http.StatusOK, manifest)
}

func (h *Handler) PreviewAssembly(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var payload struct {
		Config json.RawMessage `json:"config"`
	}
	if !httpx.Decode(w, r, &payload, 1<<20, false) {
		return
	}
	if len(payload.Config) == 0 || !json.Valid(payload.Config) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "config must be valid JSON"})
		return
	}

	config, err := h.service.catalog.DecodeRawConfig(payload.Config)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	manifest, err := h.service.ManifestForConfig(ctx, "preview", config)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidAssembly):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			h.serverError(w, "build assembly preview manifest", err)
		}
		return
	}

	writeJSON(w, http.StatusOK, manifest)
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
