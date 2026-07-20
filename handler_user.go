package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Joseap1996/Chirpy/internal/auth"
	"github.com/Joseap1996/Chirpy/internal/database"
	"github.com/google/uuid"
)

type User struct {
	ID             uuid.UUID `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Email          string    `json:"email"`
	HashedPassword string    `json:"hashed_password"`
}

func (cfg *apiConfig) handleUsers(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password string `json:"password"`
		Eml      string `json:"email"`
	}
	decoder := json.NewDecoder(r.Body)
	response := parameters{}
	err := decoder.Decode(&response)
	if err != nil {
		msg := "error decoding json"
		respondWithError(w, http.StatusBadRequest, msg)
		return
	}
	hashed_password, err := auth.HashPassword(response.Password)
	if err != nil {
		msg := "Error hashing password"
		respondWithError(w, http.StatusBadRequest, msg)
		return
	}

	user, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{
		ID:             uuid.New(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Email:          response.Eml,
		HashedPassword: hashed_password,
	})

	if err != nil {
		msg := "error creating user"
		respondWithError(w, http.StatusBadRequest, msg)
		return
	}

	returnVals := User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}

	respondWithJSON(w, http.StatusCreated, returnVals)

}

func (cfg *apiConfig) handleLogin(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password string `json:"password"`
		Eml      string `json:"email"`
	}
	type responseVals struct {
		ID           uuid.UUID `json:"id"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		Email        string    `json:"email"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
	}

	expirationTime := time.Hour
	decoder := json.NewDecoder(r.Body)
	response := parameters{}
	err := decoder.Decode(&response)
	if err != nil {
		msg := "error decoding json"
		respondWithError(w, http.StatusBadRequest, msg)
		return
	}

	user, err := cfg.db.GetUser(r.Context(), response.Eml)
	if err != nil {
		msg := "Incorrect email or password"
		respondWithError(w, http.StatusUnauthorized, msg)
		return
	}

	match, err := auth.CheckPasswordHash(response.Password, user.HashedPassword)
	if err != nil || !match {
		msg := "Incorrect email or password"
		respondWithError(w, http.StatusUnauthorized, msg)
		return
	}

	token, err := auth.MakeJWT(user.ID, cfg.secret, expirationTime)
	if err != nil {
		msg := "Error creating token"
		respondWithError(w, http.StatusInternalServerError, msg)
		return
	}
	refreshToken := auth.MakeRefreshToken()
	rtExpiration := time.Now().Add(time.Hour * 1440) // refresh token expiration time is 60 days
	_, err = cfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:     refreshToken,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		ExpiresAt: rtExpiration,
		RevokedAt: sql.NullTime{},
	})

	if err != nil {
		msg := "Errore adding refresh token to the database"
		respondWithError(w, http.StatusInternalServerError, msg)
		return
	}

	returnVals := responseVals{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        token,
		RefreshToken: refreshToken,
	}

	respondWithJSON(w, http.StatusOK, returnVals)
}

func (cfg *apiConfig) handleUpdate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password string `json:"password"`
		Eml      string `json:"email"`
	}
	decoder := json.NewDecoder(r.Body)
	response := parameters{}
	err := decoder.Decode(&response)
	if err != nil {
		msg := "error decoding json"
		respondWithError(w, http.StatusBadRequest, msg)
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

	hashed_password, err := auth.HashPassword(response.Password)
	if err != nil {
		msg := "Error hashing password"
		respondWithError(w, http.StatusBadRequest, msg)
		return
	}

	user, err := cfg.db.UpdatePasswordEml(r.Context(), database.UpdatePasswordEmlParams{
		HashedPassword: hashed_password,
		Email:          response.Eml,
		ID:             user_id,
	})
	if err != nil {
		msg := "Error updating password"
		respondWithError(w, http.StatusInternalServerError, msg)
		return
	}

	returnVals := User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}

	respondWithJSON(w, http.StatusOK, returnVals)

}
