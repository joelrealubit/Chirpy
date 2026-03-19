package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
	
)

type apiConfig struct {
		fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
 			w.Header().Set("Cache-Control", "no-cache")
			cfg.fileserverHits.Add(1)
			next.ServeHTTP(w, r)
 		})
}

func healthzHandler (w http.ResponseWriter ,req *http.Request) {
	w.Header().Set("Content-type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, req *http.Request){
	
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

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, req *http.Request){
	cfg.fileserverHits.Store(0)
	
	msg := fmt.Sprintf("Hits: %d",cfg.fileserverHits.Load())
	w.Header().Set("Content-type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(msg))
}

func main(){

	

	mux := http.NewServeMux()

	server := http.Server{
		Addr: ":8080",
		Handler: mux,

	}
	

	var apiCfg apiConfig

	fileServer := http.FileServer(http.Dir("."))
	mux.Handle("/app/", http.StripPrefix("/app", apiCfg.middlewareMetricsInc(fileServer)))
	
	
	//mux.Handle("GET /app/", http.StripPrefix("/app", fileServer))
	mux.HandleFunc("GET /api/healthz",healthzHandler )

	mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)

	mux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)

	if err:=server.ListenAndServe(); err !=nil {
		panic(fmt.Sprintf("could not start server: %s", err.Error()))
	}
}