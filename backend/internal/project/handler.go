package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"taverna/backend/internal/catalog"
	"taverna/backend/internal/modelasset"
)

type Handler struct {
	repo    *Repository
	catalog *catalog.Service
	assets  *modelasset.Repository
	logger  *slog.Logger
}

func NewHandler(repo *Repository, catalogService *catalog.Service, assets *modelasset.Repository, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, catalog: catalogService, assets: assets, logger: logger}
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
	payload, config, ok := h.decodePayload(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if !h.validateAssemblyAssets(w, ctx, config) {
		return
	}

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
	payload, config, ok := h.decodePayload(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if !h.validateAssemblyAssets(w, ctx, config) {
		return
	}

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

func (h *Handler) decodePayload(w http.ResponseWriter, r *http.Request) (ProjectPayload, catalog.CharacterConfig, bool) {
	defer r.Body.Close()

	var payload ProjectPayload
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project payload"})
		return ProjectPayload{}, catalog.CharacterConfig{}, false
	}

	if len(payload.Config) == 0 || !json.Valid(payload.Config) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "config must be valid JSON"})
		return ProjectPayload{}, catalog.CharacterConfig{}, false
	}

	config, err := h.catalog.DecodeRawConfig(payload.Config)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return ProjectPayload{}, catalog.CharacterConfig{}, false
	}

	if err := h.catalog.ValidateConfig(config); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return ProjectPayload{}, catalog.CharacterConfig{}, false
	}

	return payload, config, true
}

func (h *Handler) validateAssemblyAssets(w http.ResponseWriter, ctx context.Context, config catalog.CharacterConfig) bool {
	if h.assets == nil || len(config.Assembly.Placements) == 0 {
		return true
	}

	assetsByID, missing, err := h.loadAssemblyAssets(ctx, config)
	if err != nil {
		h.serverError(w, "validate assembly assets", err)
		return false
	}
	if len(missing) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "assembly references unknown model assets: " + strings.Join(missing, ", ")})
		return false
	}

	if err := validateAssemblyAssetPlacements(config, assetsByID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return false
	}

	return true
}

func (h *Handler) loadAssemblyAssets(ctx context.Context, config catalog.CharacterConfig) (map[string]modelasset.ModelAsset, []string, error) {
	assetsByID := make(map[string]modelasset.ModelAsset)
	missing := []string{}
	seen := make(map[string]struct{})

	for _, placement := range config.Assembly.Placements {
		assetID := strings.TrimSpace(placement.AssetID)
		if assetID == "" {
			continue
		}
		if _, ok := seen[assetID]; ok {
			continue
		}
		seen[assetID] = struct{}{}

		asset, err := h.assets.Get(ctx, assetID)
		if err != nil {
			if errors.Is(err, modelasset.ErrNotFound) {
				missing = append(missing, assetID)
				continue
			}
			return nil, nil, err
		}
		assetsByID[asset.ID] = asset
	}

	return assetsByID, missing, nil
}

func validateAssemblyAssetPlacements(config catalog.CharacterConfig, assetsByID map[string]modelasset.ModelAsset) error {
	for _, placement := range config.Assembly.Placements {
		assetID := strings.TrimSpace(placement.AssetID)
		asset, ok := assetsByID[assetID]
		if !ok {
			continue
		}
		if asset.Slot != placement.Slot {
			return fmt.Errorf("assembly placement %q uses asset %q from slot %q as slot %q", placement.ID, asset.ID, asset.Slot, placement.Slot)
		}
		if !assetHasAssemblyPoint(asset, placement.AssetPointID) {
			return fmt.Errorf("assembly placement %q references unknown asset_point_id %q for model asset %q", placement.ID, placement.AssetPointID, asset.ID)
		}
	}
	return nil
}

func assetHasAssemblyPoint(asset modelasset.ModelAsset, assetPointID string) bool {
	assetPointID = strings.TrimSpace(assetPointID)
	if assetPointID == "" {
		return true
	}
	for _, point := range asset.AssemblyPoints {
		if point.ID == assetPointID {
			return true
		}
	}
	return false
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
