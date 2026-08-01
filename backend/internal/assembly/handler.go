package assembly

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
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
	if projectID == "" || strings.Contains(projectID, "/") {
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

func (h *Handler) serverError(w http.ResponseWriter, operation string, err error) {
	h.logger.Error(operation+" failed", "error", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
