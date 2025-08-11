package main

import (
	"chirpy/internal/auth"
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
	platform       string
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
	writer.Header()["Content-Type"] = []string{textContent}
	writer.WriteHeader(http.StatusOK)
	writer.Write([]byte(fmt.Sprintf(adminTemplate, cfg.fileserverHits.Load())))
}

func handleHealthz(writer http.ResponseWriter, req *http.Request) {
	writer.Header()["Content-Type"] = []string{textContent}
	writer.WriteHeader(http.StatusOK)
	writer.Write([]byte("OK"))
}

type chirpMsg struct {
	Body   string    `json:"body"`
	UserId uuid.UUID `json:"user_id"`
}

type createHeader struct {
	Id        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type chirpErr struct {
	Error string `json:"error"`
}

type chirpResp struct {
	createHeader
	chirpMsg
}

type addUser struct {
	Password string `json:"password"`
	Email    string `json:"email"`
}

type addedUser struct {
	createHeader
	Email string `json:"email"`
}

const dbEnv = "DB_URL"
const platformEnv = "PLATFORM"
const devPlatform = "dev"
const lengthLimit = 140
const jsonContent = "application/json"
const textContent = "text/plain; charset=utf-8"
const marshalErrorTemplate = "{\"error\":\"Marshal error \"%s\" when trying to respond to \"%s\"}"

var dirtyWords = []string{"kerfuffle", "sharbert", "fornax"}

func handleJsonWrite(writer http.ResponseWriter, code int, msg string, jsonStruct any) {
	resp, err := json.Marshal(jsonStruct)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
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

func chirpConv(dbChirp database.Chirp) chirpResp {
	return chirpResp{createHeader: createHeader{Id: dbChirp.ID, CreatedAt: dbChirp.CreatedAt, UpdatedAt: dbChirp.UpdatedAt},
		chirpMsg: chirpMsg{Body: dbChirp.Body, UserId: dbChirp.UserID}}
}

func (cfg *apiConfig) handleMakeChirp(writer http.ResponseWriter, req *http.Request) {
	writer.Header()["Content-Type"] = []string{jsonContent}
	decoder := json.NewDecoder(req.Body)
	msg := chirpMsg{}
	if err := decoder.Decode(&msg); err != nil {
		handleJsonWrite(writer, http.StatusBadRequest, msg.Body, chirpErr{Error: err.Error()})
	} else if len(msg.Body) > lengthLimit {
		handleJsonWrite(writer, http.StatusBadRequest, msg.Body, chirpErr{Error: "Chirp is too long"})
	} else {
		chirp, err := cfg.dbQueries.CreateChirp(req.Context(), database.CreateChirpParams{Body: clean(msg.Body), UserID: msg.UserId})
		if err != nil {
			handleJsonWrite(writer, http.StatusBadRequest, msg.Body, chirpErr{Error: err.Error()})
		}
		handleJsonWrite(writer, http.StatusCreated, msg.Body, chirpConv(chirp))
	}
}

func userConv(dbUser database.CreateUserRow) addedUser {
	return addedUser{createHeader: createHeader{Id: dbUser.ID, CreatedAt: dbUser.CreatedAt, UpdatedAt: dbUser.UpdatedAt}, Email: dbUser.Email}
}

func (cfg *apiConfig) handleUsers(writer http.ResponseWriter, req *http.Request) {
	writer.Header()["Content-Type"] = []string{jsonContent}
	decoder := json.NewDecoder(req.Body)
	msg := addUser{}
	if err := decoder.Decode(&msg); err != nil {
		handleJsonWrite(writer, http.StatusBadRequest, "Createuser", chirpErr{Error: err.Error()})
		return
	}
	hashed, err := auth.HashPassword(msg.Password)
	if err != nil {
		handleJsonWrite(writer, http.StatusInternalServerError, "hash password", chirpErr{Error: err.Error()})
		return
	}
	user, err := cfg.dbQueries.CreateUser(req.Context(), database.CreateUserParams{Email: msg.Email, HashedPassword: hashed})
	if err != nil {
		handleJsonWrite(writer, http.StatusBadRequest, msg.Email, chirpErr{Error: err.Error()})
		return
	}
	handleJsonWrite(writer, http.StatusCreated, msg.Email, userConv(user))
}

func (cfg *apiConfig) handleReset(writer http.ResponseWriter, req *http.Request) {
	writer.Header()["Content-Type"] = []string{textContent}
	if cfg.platform != devPlatform {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write([]byte("forbidden"))
		return
	}
	cfg.fileserverHits.Store(0)
	if err := cfg.dbQueries.ResetUsers(req.Context()); err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		writer.Write([]byte(err.Error()))
	} else {
		writer.WriteHeader(http.StatusOK)
		writer.Write([]byte(fmt.Sprintf("Hits: %d", cfg.fileserverHits.Load())))
	}
}

func (cfg *apiConfig) handleGetChirps(writer http.ResponseWriter, req *http.Request) {
	writer.Header()["Content-Type"] = []string{jsonContent}
	chirps, err := cfg.dbQueries.GetChirps(req.Context())
	if err != nil {
		handleJsonWrite(writer, http.StatusBadRequest, "GetChirps", chirpErr{Error: err.Error()})
		return
	}
	jsonChirps := make([]chirpResp, len(chirps))
	for i := range chirps {
		jsonChirps[i] = chirpConv(chirps[i])
	}
	handleJsonWrite(writer, http.StatusOK, "GetChirps", jsonChirps)
}

func (cfg *apiConfig) handleGetChirp(writer http.ResponseWriter, req *http.Request) {
	writer.Header()["Content-Type"] = []string{jsonContent}
	idstr := req.PathValue("id")
	if len(idstr) == 0 {
		handleJsonWrite(writer, http.StatusNotFound, "GetChirp", chirpErr{Error: "invalid id"})
		return
	}
	id, err := uuid.Parse(idstr)
	if err != nil {
		handleJsonWrite(writer, http.StatusNotFound, "GetChirp", chirpErr{Error: fmt.Sprint("invalid id ", idstr)})
		return
	}
	chirp, err := cfg.dbQueries.GetChirp(req.Context(), id)
	if err != nil {
		handleJsonWrite(writer, http.StatusNotFound, "GetChirp", chirpErr{Error: err.Error()})
		return
	}
	handleJsonWrite(writer, http.StatusOK, "GetChirps", chirpConv(chirp))
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv(dbEnv)
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	apiConf := &apiConfig{dbQueries: database.New(db), platform: os.Getenv(platformEnv)}
	serverMux := http.NewServeMux()
	serverMux.Handle("/app/", http.StripPrefix("/app", apiConf.middlewareHandlerMetricsInc(http.FileServer(http.Dir(".")))))
	serverMux.HandleFunc("GET /api/healthz", handleHealthz)
	serverMux.HandleFunc("GET /admin/metrics", apiConf.handleMetrics)
	serverMux.HandleFunc("POST /admin/reset", apiConf.handleReset)
	serverMux.HandleFunc("POST /api/chirps", apiConf.middlewareMetricsInc(apiConf.handleMakeChirp))
	serverMux.HandleFunc("POST /api/users", apiConf.middlewareMetricsInc(apiConf.handleUsers))
	serverMux.HandleFunc("GET /api/chirps", apiConf.middlewareMetricsInc(apiConf.handleGetChirps))
	serverMux.HandleFunc("GET /api/chirps/{id}", apiConf.middlewareMetricsInc(apiConf.handleGetChirp))
	server := http.Server{Handler: serverMux, Addr: ":8080"}
	err = server.ListenAndServe()
	fmt.Println(err)
}
