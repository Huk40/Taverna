package project

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"taverna/backend/internal/catalog"
)

type Handler struct {
	repo    *Repository
	catalog *catalog.Service
	logger  *slog.Logger
}

func NewHandler(repo *Repository, catalogService *catalog.Service, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, catalog: catalogService, logger: logger}
}

func (h *Handler) Collection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.list(w, r)
	case http.MethodPost:
		h.create(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) Item(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	id = strings.TrimSpace(id)
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.get(w, r, id)
	case http.MethodPut:
		h.update(w, r, id)
	case http.MethodDelete:
		h.delete(w, r, id)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	payload, ok := h.decodePayload(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	project, err := h.repo.Create(ctx, payload)
	if err != nil {
		h.serverError(w, "create project", err)
		return
	}

	writeJSON(w, http.StatusCreated, project)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	projects, err := h.repo.List(ctx)
	if err != nil {
		h.serverError(w, "list projects", err)
		return
	}

	writeJSON(w, http.StatusOK, projects)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request, id string) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	project, err := h.repo.Get(ctx, id)
	if err != nil {
		h.handleRepositoryError(w, "get project", err)
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request, id string) {
	payload, ok := h.decodePayload(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	project, err := h.repo.Update(ctx, id, payload)
	if err != nil {
		h.handleRepositoryError(w, "update project", err)
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request, id string) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.repo.Delete(ctx, id); err != nil {
		h.handleRepositoryError(w, "delete project", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) decodePayload(w http.ResponseWriter, r *http.Request) (ProjectPayload, bool) {
	defer r.Body.Close()

	var payload ProjectPayload
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project payload"})
		return ProjectPayload{}, false
	}

	if len(payload.Config) == 0 || !json.Valid(payload.Config) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "config must be valid JSON"})
		return ProjectPayload{}, false
	}

	if err := h.catalog.ValidateRawConfig(payload.Config); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return ProjectPayload{}, false
	}

	return payload, true
}

func (h *Handler) handleRepositoryError(w http.ResponseWriter, operation string, err error) {
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
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
