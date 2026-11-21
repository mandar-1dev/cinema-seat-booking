package catalog

import (
	"net/http"

	"github.com/sikozonpc/cinema/internal/utils"
)

type Handler struct {
	repository *MongoRepository
}

func NewHandler(repository *MongoRepository) *Handler {
	return &Handler{repository: repository}
}

func (h *Handler) ListMovies(w http.ResponseWriter, r *http.Request) {
	result, err := h.repository.ListMovies(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "could not load movies")
		return
	}
	utils.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) ListTheatres(w http.ResponseWriter, r *http.Request) {
	result, err := h.repository.ListTheatres(r.Context(), r.URL.Query().Get("city"))
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "could not load theatres")
		return
	}
	utils.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) ListShows(w http.ResponseWriter, r *http.Request) {
	result, err := h.repository.ListShows(
		r.Context(),
		r.URL.Query().Get("movie_id"),
		r.URL.Query().Get("theatre_id"),
		r.URL.Query().Get("date"),
	)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "could not load shows")
		return
	}
	utils.WriteJSON(w, http.StatusOK, result)
}
