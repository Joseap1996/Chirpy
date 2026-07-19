package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Joseap1996/Chirpy/internal/auth"
	"github.com/Joseap1996/Chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	secret         string
}

type User struct {
	ID             uuid.UUID `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Email          string    `json:"email"`
	HashedPassword string    `json:"hashed_password"`
}

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

func handleEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
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
func (cfg *apiConfig) handleHits(w http.ResponseWriter, r *http.Request) {
	hits := cfg.fileserverHits.Load()
	str := fmt.Sprintf("Welcome, Chirpy Admin\nChirpy has been visited %d times!", hits)
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte(str))
}

func (cfg *apiConfig) handleHitsReset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(403)
		w.Write([]byte("Forbidden"))
		return
	}

	err := cfg.db.DeleteUsers(r.Context())
	if err != nil {
		log.Printf("Error deleting users: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Something went wrong"))
		return
	}
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("Users deleted."))

}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handleRefresh(w http.ResponseWriter, r *http.Request) {
	bearer, err := auth.GetBearerToken(r.Header)
	if err != nil {
		msg := "error getting bearer token"
		respondWithError(w, http.StatusInternalServerError, msg)
		return
	}

	user_id, err := cfg.db.GetUserFromRefreshToken(r.Context(), bearer)
	if err != nil {
		msg := "error getting user"
		respondWithError(w, http.StatusUnauthorized, msg)
		return
	}

	token, err := auth.MakeJWT(user_id, cfg.secret, time.Hour)
	if err != nil {
		msg := "Error creating token"
		respondWithError(w, http.StatusInternalServerError, msg)
		return
	}
	type Response struct {
		Token string `json:"token"`
	}
	returnVals := Response{
		Token: token,
	}

	respondWithJSON(w, http.StatusOK, returnVals)

}

func (cfg *apiConfig) handleRevoke(w http.ResponseWriter, r *http.Request) {
	bearer, err := auth.GetBearerToken(r.Header)
	if err != nil {
		msg := "error getting bearer token"
		respondWithError(w, http.StatusInternalServerError, msg)
		return
	}
	err = cfg.db.RevokeRefreshToken(r.Context(), bearer)
	if err != nil {
		msg := "error revoking token"
		respondWithError(w, http.StatusInternalServerError, msg)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

//helper funcs

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type returnVal struct {
		Error string `json:"error"`
	}
	message := returnVal{
		Error: msg,
	}
	dat, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func removeBadWords(msg string) string {
	badwords := []string{"kerfuffle", "sharbert", "fornax"}
	words := strings.Split(msg, " ")

	for i, word := range words {
		for _, badword := range badwords {
			if strings.ToLower(word) == badword {
				words[i] = "****"
			}
		}
	}
	return strings.Join(words, " ")

}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	secret := os.Getenv("SECRET")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println(err)
	}
	dbQueries := database.New(db)

	apiCon := apiConfig{
		db:       dbQueries,
		platform: platform,
		secret:   secret,
	}
	serveMux := http.NewServeMux()
	serveStruct := http.Server{}
	serveStruct.Addr = ":8080"
	serveStruct.Handler = serveMux
	serveMux.Handle("/app/", http.StripPrefix("/app", apiCon.middlewareMetricsInc(http.FileServer(http.Dir(".")))))

	// handle registers
	serveMux.HandleFunc("GET /api/healthz", handleEndpoint)
	serveMux.HandleFunc("GET /admin/metrics", apiCon.handleHits)
	serveMux.HandleFunc("POST /admin/reset", apiCon.handleHitsReset)
	serveMux.HandleFunc("POST /api/users", apiCon.handleUsers)
	serveMux.HandleFunc("POST /api/chirps", apiCon.handleChirps)
	serveMux.HandleFunc("GET /api/chirps", apiCon.handleGetChirps)
	serveMux.HandleFunc("GET /api/chirps/{id}", apiCon.handleGetChirp)
	serveMux.HandleFunc("POST /api/login", apiCon.handleLogin)
	serveMux.HandleFunc("POST /api/refresh", apiCon.handleRefresh)
	serveMux.HandleFunc("POST /api/revoke", apiCon.handleRevoke)
	serveMux.HandleFunc("PUT /api/users", apiCon.handleUpdate)

	if err := serveStruct.ListenAndServe(); err != nil {
		fmt.Println(err)
	}

}
