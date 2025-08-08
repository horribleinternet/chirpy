package main

import (
	"chirpy/internal/database"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
}

func (cfg *apiConfig) middlewareMetricsInc(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		cfg.fileserverHits.Add(1)
		next(w, req)
	}
}

func (cfg *apiConfig) middlewareHandlerMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(cfg.middlewareMetricsInc(next.ServeHTTP))
}

const adminTemplate = "<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>"

func (cfg *apiConfig) handleMetrics(writer http.ResponseWriter, req *http.Request) {
	writer.Header()["Content-Type"] = []string{"text/html; charset=utf-8"}
	writer.WriteHeader(200)
	writer.Write([]byte(fmt.Sprintf(adminTemplate, cfg.fileserverHits.Load())))
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

type chirpMsg struct {
	Body string `json:"body"`
}

type chirpErr struct {
	Error string `json:"error"`
}

type chirpResp struct {
	//Valid bool `json:"valid"`
	Cleaned string `json:"cleaned_body"`
}

type addUser struct {
	//Valid bool `json:"valid"`
	Email string `json:"email"`
}

type addedUser struct {
	Id        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

const lengthLimit = 140
const jsonContent = "application/json"
const marshalErrorTemplate = "{\"error\":\"Marshal error \"%s\" when trying to respond to \"%s\"}"
const marshalError = 500
const genericError = 400
const genericSuccess = 200
const createdSuccess = 201

var dirtyWords = []string{"kerfuffle", "sharbert", "fornax"}

func handleJsonWrite(writer http.ResponseWriter, code int, msg string, jsonStruct any) {
	resp, err := json.Marshal(jsonStruct)
	if err != nil {
		writer.WriteHeader(marshalError)
		fmt.Fprintf(writer, marshalErrorTemplate, err, msg)
	} else {
		writer.WriteHeader(code)
		writer.Write(resp)
	}
}

func clean(dirty string) string {
	words := strings.Split(dirty, " ")
	for i, word := range words {
		if slices.Contains(dirtyWords, strings.ToLower(word)) {
			words[i] = "****"
		}
	}
	return strings.Join(words, " ")
}

func handleValidate(writer http.ResponseWriter, req *http.Request) {
	writer.Header()["Content-Type"] = []string{jsonContent}
	decoder := json.NewDecoder(req.Body)
	msg := chirpMsg{}
	if err := decoder.Decode(&msg); err != nil {
		handleJsonWrite(writer, genericError, msg.Body, chirpErr{Error: err.Error()})
	} else if len(msg.Body) > lengthLimit {
		handleJsonWrite(writer, genericError, msg.Body, chirpErr{Error: "Chirp is too long"})
	} else {
		//handleJsonWrite(writer, genericSuccess, msg.Body, chirpResp{Valid: true})
		handleJsonWrite(writer, genericSuccess, msg.Body, chirpResp{Cleaned: clean(msg.Body)})
	}
}

func userConv(dbUser database.User) addedUser {
	return addedUser{Id: dbUser.ID, CreatedAt: dbUser.CreatedAt, UpdatedAt: dbUser.UpdatedAt, Email: dbUser.Email}
}

func (cfg *apiConfig) handleUsers(writer http.ResponseWriter, req *http.Request) {
	writer.Header()["Content-Type"] = []string{jsonContent}
	decoder := json.NewDecoder(req.Body)
	msg := addUser{}
	if err := decoder.Decode(&msg); err != nil {
		handleJsonWrite(writer, genericError, msg.Email, chirpErr{Error: err.Error()})
		return
	}
	user, err := cfg.dbQueries.CreateUser(req.Context(), msg.Email)
	if err != nil {
		handleJsonWrite(writer, genericError, msg.Email, chirpErr{Error: err.Error()})
		return
	}
	handleJsonWrite(writer, createdSuccess, msg.Email, userConv(user))
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	apiConf := &apiConfig{dbQueries: database.New(db)}
	serverMux := http.NewServeMux()
	serverMux.Handle("/app/", http.StripPrefix("/app", apiConf.middlewareHandlerMetricsInc(http.FileServer(http.Dir(".")))))
	serverMux.HandleFunc("GET /api/healthz", handleHealthz)
	serverMux.HandleFunc("GET /admin/metrics", apiConf.handleMetrics)
	serverMux.HandleFunc("POST /admin/reset", apiConf.handleReset)
	serverMux.HandleFunc("POST /api/validate_chirp", apiConf.middlewareMetricsInc(handleValidate))
	serverMux.HandleFunc("POST /api/users", apiConf.middlewareMetricsInc(apiConf.handleUsers))
	server := http.Server{Handler: serverMux, Addr: ":8080"}
	err = server.ListenAndServe()
	fmt.Println(err)
}
