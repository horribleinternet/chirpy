package main

import (
	"fmt"
	"net/http"
)

func main() {
	serverMux := http.NewServeMux()
	server := http.Server{Handler: serverMux, Addr: ":8080"}
	err := server.ListenAndServe()
	fmt.Println(err)
}
