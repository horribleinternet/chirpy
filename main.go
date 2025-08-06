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
	bs := func(w http.ResponseWriter, req *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, req)
	}
	return http.HandlerFunc(bs)
}

func (cfg *apiConfig) handleMetrics(writer http.ResponseWriter, req *http.Request) {
	writer.Header()["Content-Type"] = []string{"text/plain; charset=utf-8"}
	writer.WriteHeader(200)
	writer.Write([]byte(fmt.Sprintf("Hits: %d", cfg.fileserverHits.Load())))
}

func (cfg *apiConfig) handleReset(writer http.ResponseWriter, req *http.Request) {
	cfg.fileserverHits.Store(0)
	writer.Header()["Content-Type"] = []string{"text/plain; charset=utf-8"}
	writer.WriteHeader(200)
	writer.Write([]byte(fmt.Sprintf("Hits: %d", cfg.fileserverHits.Load())))
}

func handleHealthz(writer http.ResponseWriter, req *http.Request) {
	writer.Header()["Content-Type"] = []string{"text/plain; charset=utf-8"}
	writer.WriteHeader(200)
	writer.Write([]byte("OK"))
}

func main() {
	apiConf := &apiConfig{}
	serverMux := http.NewServeMux()
	serverMux.Handle("/app/", http.StripPrefix("/app", apiConf.middlewareMetricsInc(http.FileServer(http.Dir(".")))))
	serverMux.HandleFunc("GET /api/healthz", handleHealthz)
	serverMux.HandleFunc("GET /api/metrics", apiConf.handleMetrics)
	serverMux.HandleFunc("POST /api/reset", apiConf.handleReset)
	server := http.Server{Handler: serverMux, Addr: ":8080"}
	err := server.ListenAndServe()
	fmt.Println(err)
}
