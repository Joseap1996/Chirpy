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
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	secret         string
}

func handleEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
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
	serveMux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCon.handleDeleteChirp)
	serveMux.HandleFunc("POST /api/login", apiCon.handleLogin)
	serveMux.HandleFunc("POST /api/refresh", apiCon.handleRefresh)
	serveMux.HandleFunc("POST /api/revoke", apiCon.handleRevoke)
	serveMux.HandleFunc("PUT /api/users", apiCon.handleUpdate)

	if err := serveStruct.ListenAndServe(); err != nil {
		fmt.Println(err)
	}

}
