package userprofile

import (
	"errors"
	"log/slog"
	"net/http"

	"example.com/testapp/internal/shared/httpx"
)

// Handler exposes the user-profile feature over HTTP.
type Handler struct {
	service *Service
	logger  *slog.Logger
}

// NewHandler returns a Handler for the user-profile feature.
//
// A nil logger falls back to [slog.Default] so the feature stays usable in
// tests without extra setup.
func NewHandler(service *Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger}
}

// RegisterRoutes mounts the feature's routes on mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /user-profile", h.list)
	mux.HandleFunc("POST /user-profile", h.create)
}

// list handles GET /user-profile.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		h.logger.Error("list user-profile", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "could not list user-profile")
		return
	}

	responses := make([]UserProfileResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.ToResponse())
	}
	httpx.JSON(w, http.StatusOK, responses)
}

// create handles POST /user-profile.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateUserProfileRequest
	if err := httpx.DecodeJSON(r, 0, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	item, err := h.service.Create(r.Context(), req)
	if err != nil {
		var invalid *ValidationError
		if errors.As(err, &invalid) {
			httpx.Error(w, http.StatusUnprocessableEntity, invalid.Error())
			return
		}

		h.logger.Error("create user-profile", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "could not create user-profile")
		return
	}

	httpx.JSON(w, http.StatusCreated, item.ToResponse())
}
