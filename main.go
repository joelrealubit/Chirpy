package main

import (
	"fmt"
	"net/http"
)

func main(){

	mux := http.NewServeMux()

	server := http.Server{
		Addr: ":8080",
		Handler: mux,

	}


	if err:=server.ListenAndServe(); err !=nil {
		panic(fmt.Sprintf("could not start server: %s", err.Error()))
	}
}