package exportjob

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	repo   *Repository
	logger *slog.Logger
}

func NewHandler(repo *Repository, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, logger: logger}
}

func (h *Handler) ProjectExports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	projectID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/projects/"), "/exports")
	projectID = strings.Trim(projectID, "/")
	if projectID == "" || strings.Contains(projectID, "/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	defer r.Body.Close()
	var request CreateRequest
	if r.Body != nil {
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				request = CreateRequest{}
			} else {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid export payload"})
				return
			}
		}
	}

	if request.Format == "" && request.Target == "" {
		request = CreateRequest{}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	job, err := h.repo.Create(ctx, projectID, request)
	if err != nil {
		if errors.Is(err, ErrInvalidRequest) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid export payload"})
			return
		}
		if errors.Is(err, ErrProjectNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
			return
		}
		h.serverError(w, "create export job", err)
		return
	}

	writeJSON(w, http.StatusCreated, job)
}

func (h *Handler) Item(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/exports/")
	id = strings.TrimSpace(id)
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "export job not found"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	job, err := h.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "export job not found"})
			return
		}
		h.serverError(w, "get export job", err)
		return
	}

	writeJSON(w, http.StatusOK, job)
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
