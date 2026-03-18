package main

import (
	"fmt"
	"net/http"
)

func healthzHandler (w http.ResponseWriter ,req *http.Request) {
	w.Header().Set("Content-type", "text/plain; charset=utf-8")
	w.Write([]byte("OK"))
}

func main(){

	mux := http.NewServeMux()

	server := http.Server{
		Addr: ":8080",
		Handler: mux,

	}
	fileServer := http.FileServer(http.Dir("."))
	mux.Handle("GET /app/", http.StripPrefix("/app", fileServer))
	mux.HandleFunc("GET /healthz",healthzHandler )


	if err:=server.ListenAndServe(); err !=nil {
		panic(fmt.Sprintf("could not start server: %s", err.Error()))
	}
}