package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func handleValidate(w http.ResponseWriter, r *http.Request) {
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

	type returnVal struct {
		CleanedBody string `json:"cleaned_body"`
	}

	respBody := returnVal{
		CleanedBody: cleanedBody,
	}

	respondWithJSON(w, http.StatusOK, respBody)

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
	cfg.fileserverHits.Store(0)
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("Hits reset to 0."))

}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
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
	apiCon := apiConfig{}
	serveMux := http.NewServeMux()
	serveStruct := http.Server{}
	serveStruct.Addr = ":8080"
	serveStruct.Handler = serveMux
	serveMux.Handle("/app/", http.StripPrefix("/app", apiCon.middlewareMetricsInc(http.FileServer(http.Dir(".")))))

	// handle registers
	serveMux.HandleFunc("GET /api/healthz", handleEndpoint)
	serveMux.HandleFunc("GET /admin/metrics", apiCon.handleHits)
	serveMux.HandleFunc("POST /admin/reset", apiCon.handleHitsReset)
	serveMux.HandleFunc("POST /api/validate_chirp", handleValidate)

	if err := serveStruct.ListenAndServe(); err != nil {
		fmt.Println(err)
	}

}
