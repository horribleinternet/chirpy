package main

import (
	"fmt"
	"net/http"
)

func main() {
	serverMux := http.NewServeMux()
	serverMux.Handle("/", http.FileServer(http.Dir(".")))
	server := http.Server{Handler: serverMux, Addr: ":8080"}
	err := server.ListenAndServe()
	fmt.Println(err)
}
