package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Joseap1996/Chirpy/internal/auth"
	"github.com/Joseap1996/Chirpy/internal/database"
	"github.com/google/uuid"
)

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func (cfg *apiConfig) handleChirps(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		msg := "Something went wrong"
		respondWithError(w, http.StatusBadRequest, msg)
		return
	}
	// lenght check
	if len(params.Body) > 140 {
		msg := "Chirp is too long"
		respondWithError(w, http.StatusBadRequest, msg)
		return
	}
	msg := params.Body
	cleanedBody := removeBadWords(msg)

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		msg := "Error getting Bearer token"
		respondWithError(w, http.StatusUnauthorized, msg)
		return
	}

	tokenID, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		msg := "Invalid JWT"
		respondWithError(w, http.StatusUnauthorized, msg)
		return
	}

	chirp, err := cfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Body:      cleanedBody,
		UserID:    tokenID,
	})

	if err != nil {
		msg := "Error creating chirp"
		respondWithError(w, http.StatusInternalServerError, msg)
		return
	}

	respBody := Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	}

	respondWithJSON(w, http.StatusCreated, respBody)

}

func (cfg *apiConfig) handleGetChirps(w http.ResponseWriter, r *http.Request) {
	chirps, err := cfg.db.GetChirps(r.Context())
	if err != nil {
		msg := "Error getting chirps"
		respondWithError(w, http.StatusInternalServerError, msg)
		return
	}
	respBody := []Chirp{}
	for _, chirp := range chirps {
		respBody = append(respBody, Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		})
	}
	respondWithJSON(w, http.StatusOK, respBody)
}

func (cfg *apiConfig) handleGetChirp(w http.ResponseWriter, r *http.Request) {
	chirpIDString := r.PathValue("id")
	chirpID, err := uuid.Parse(chirpIDString)
	if err != nil {
		msg := "Invalid id"
		respondWithError(w, http.StatusBadRequest, msg)
		return
	}

	chirp, err := cfg.db.GetChirp(r.Context(), chirpID)
	if err != nil {
		msg := "Error getting chirp"
		respondWithError(w, http.StatusNotFound, msg)
		return
	}
	respBody := Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	}

	respondWithJSON(w, http.StatusOK, respBody)

}

func (cfg *apiConfig) handleDeleteChirp(w http.ResponseWriter, r *http.Request) {
	chirpIDString := r.PathValue("chirpID")
	chirpID, err := uuid.Parse(chirpIDString)
	if err != nil {
		msg := "Invalid id"
		respondWithError(w, http.StatusBadRequest, msg)
		return
	}
	chirp, err := cfg.db.GetChirp(r.Context(), chirpID)
	if err != nil {
		msg := "Error getting chirp"
		respondWithError(w, http.StatusNotFound, msg)
		return
	}

	bearer, err := auth.GetBearerToken(r.Header)
	if err != nil {
		msg := "error getting bearer token"
		respondWithError(w, http.StatusUnauthorized, msg)
		return
	}

	user_id, err := auth.ValidateJWT(bearer, cfg.secret)
	if err != nil {
		msg := "Invalid JWT"
		respondWithError(w, http.StatusUnauthorized, msg)
		return
	}
	if user_id != chirp.UserID {
		msg := "Wrong user"
		respondWithError(w, http.StatusForbidden, msg)
		return
	}
	err = cfg.db.DeleteChirp(r.Context(), chirpID)
	if err != nil {
		msg := "chirp not found"
		respondWithError(w, http.StatusNotFound, msg)
		return
	}
	w.WriteHeader(http.StatusNoContent)

}
