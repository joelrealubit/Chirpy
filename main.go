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

// wrap a handler with middleware
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func healthzHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte("OK"))
}

// handler - ie request handler for metrics
func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, req *http.Request) {

	msg := fmt.Sprintf(`<html> 
			<body>
				<h1>Welcome, Chirpy Admin</h1>
				<p>Chirpy has been visited %d times!</p>
			</body>
			</html>`, cfg.fileserverHits.Load())
	w.Header().Set("Content-type", "text/html")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(msg))
}

// handler for reset
func (cfg *apiConfig) resetHandler(w http.ResponseWriter, req *http.Request) {
	cfg.fileserverHits.Store(0)

	msg := fmt.Sprintf("Hits: %d", cfg.fileserverHits.Load())
	w.Header().Set("Content-type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(msg))
}

// validate chirp handler
func validateChirpHandler(w http.ResponseWriter, r *http.Request) {
	type bodyparam struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	bodparam := bodyparam{}
	err := decoder.Decode(&bodparam)
	if err != nil {
		log.Printf("error: something went wrong: %s", err)
		w.WriteHeader(500)
		return
	}

	type returnVal struct {
		Valid bool   `json:"valid"`
		Body  string `json:"cleaned_body"`
	}

	respBody := returnVal{
		Valid: true,
	}

	if len(bodparam.Body) > 140 {
		log.Printf("error: Chirp is too long")
		w.WriteHeader(400)
		respBody.Valid = false

	} else {
		respBody.Valid = true
	}

	// curseWords := map[string]string{
	// 	"kerfuffle": "kerfuffle",
	// 	"sharbert":  "sharbert",
	// 	"fornax":    "fornax",
	// }

	curseWords := []string{"kerfuffle", "sharbert", "fornax"}

	var newBody = bodparam.Body
	parts := strings.Split(bodparam.Body, " ")
	for i, part := range parts {
		for _, curse := range curseWords {
			if strings.ToLower(part) == curse {
				parts[i] = "****"
			}
		}
	}

	newBody = strings.Join(parts, " ")

	//}

	if strings.Contains(newBody, "****") {
		bodparam.Body = newBody
	}

	respBody.Body = bodparam.Body
	//check body for curse words, then replace with asterisk

	dat, err := json.Marshal(respBody)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if !respBody.Valid {
		w.WriteHeader(500)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(dat)

}

func main() {

	mux := http.NewServeMux()

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	var apiCfg apiConfig

	fileServer := http.FileServer(http.Dir("."))
	mux.Handle("/app/", http.StripPrefix("/app", apiCfg.middlewareMetricsInc(fileServer)))

	mux.HandleFunc("GET /api/healthz", healthzHandler)

	mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)

	mux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)

	mux.HandleFunc("POST /api/validate_chirp", validateChirpHandler)

	if err := server.ListenAndServe(); err != nil {
		panic(fmt.Sprintf("could not start server: %s", err.Error()))
	}
}
