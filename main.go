package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joelrealubit/Chirpy/internal/auth"
	"github.com/joelrealubit/Chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQ            *database.Queries
	dbPlatform     string
	secret         string
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

	if cfg.dbPlatform != "dev" {
		log.Printf("RESETHANDLER: Forbidden\n")
		w.WriteHeader(400)
		return
	}
	cfg.fileserverHits.Store(0)

	cfg.dbQ.DeleteUsers(req.Context())

	msg := fmt.Sprintf("RESETHANDLER: Hits: %d\n", cfg.fileserverHits.Load())
	w.Header().Set("Content-type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(msg))
}

// CHIRP STUFF
type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

// POST API for adding new chirps
func IsJWT(tokenStr string) bool {
	parts := strings.Split(tokenStr, ".")
	return len(parts) == 3
}

func (cfg *apiConfig) newChirpHandler(w http.ResponseWriter, r *http.Request) {

	log.Print("#### IN NEW CHIRP HANDLER!!! ####\n")
	type bodyparam struct {
		Body         string    `json:"body"`
		UserID       uuid.UUID `json:"user_id"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
	}

	decoder := json.NewDecoder(r.Body)
	bodparam := bodyparam{}
	err := decoder.Decode(&bodparam)
	if err != nil {
		log.Printf("NEWCHIRPHANDLER::error: something went wrong decoding to bodparam: %s\n", err)
		w.WriteHeader(500)
		return
	}

	// log.Printf("NEWCHIRPHANDLER CHIRP BODY: %s\n", bodparam.Body)
	// log.Printf("NEWCHIRPHANDLER :: TOKEN : %s\n", bodparam.Token)
	// log.Printf("NEWCHIRPHANDLER :: REFRESH TOKEN: %s\n ", strings.TrimSpace(bodparam.RefreshToken))
	// log.Printf("NEWCHIRPHANDLER :: USERID : %s\n", bodparam.UserID)

	bearerToken, err := auth.GetBearerToken(r.Header)
	log.Printf("NEWCHIRPHANDLER:: bearerToken: %s\n", bearerToken)

	if err != nil {
		log.Printf("NEWCHIRPHANDLER:: error: something went wrong getting bearer token: %s\n", err)
		w.WriteHeader(401)
		return
	}
	var userID uuid.UUID

	if !IsJWT(bearerToken) {
		log.Print("NEWCHIPHANDLER:: BEARERTOKEN IS NOT JWT! ITS A REFRESH TOKEN!!!")

		refresh_token, err := cfg.dbQ.GetRefreshToken(r.Context(), bearerToken)

		if err != nil {
			log.Printf("NEWCHIRPHANDLER:: error getting refreshtoken from db: %s\n", err)
			w.WriteHeader(401)
			return
		}

		if refresh_token.Token != bearerToken {
			log.Print("NEWCHIRPHANDLER :: BEARERTOKEN INVALID")
			w.WriteHeader(401)
			return
		}

		if refresh_token.ExpiresAt.Valid && refresh_token.ExpiresAt.Time.UTC().Local().Before(time.Now().UTC().Local()) {
			log.Printf("NEWCHIRPHANDLER::NOTJWT:: error: refresh token expired: %s", err)
			w.WriteHeader(401)
			return
		}
		if refresh_token.RevokedAt.Valid && refresh_token.RevokedAt.Time.UTC().Local().Before(time.Now().UTC().Local()) {
			log.Printf("NEWCHIRPHANDLER:: NOTJWT:: error: refresh token expired: %s \n", err)
			w.WriteHeader(401)
			return
		}
		userID = refresh_token.UserID
		log.Print("NEWCHIRPHANDLER:: REFRESH TOKEN OK!!!\n")
	} else {
		log.Print("NEWCHIRPHANDLER:: BEARER TOKEN IS JWT!!!\n")
		uid, err := auth.ValidateJWT(bearerToken, cfg.secret)
		if err != nil {
			log.Printf("NEWCHIRPHANDLER:: error: auth.ValidateJWT failed->unauthorized: %s \n", err)
			w.WriteHeader(401)
			return
		}
		log.Print("NEWCHIRPHANDLER:: ValidateJWT OK!!!")
		userID = uid

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
		return

	} else {
		respBody.Valid = true
	}

	curseWords := []string{"kerfuffle", "sharbert", "fornax"}

	//replace curse words with asterisk
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

	if strings.Contains(newBody, "****") {
		bodparam.Body = newBody
	}

	w.Header().Set("Content-Type", "application/json")
	if !respBody.Valid {
		w.WriteHeader(500)
		return
	}

	createChirpParam := database.CreateChirpParams{}
	createChirpParam.Body = bodparam.Body
	createChirpParam.UserID = userID
	log.Print("CREATING CHIRP ON DB...")
	chirp, err := cfg.dbQ.CreateChirp(r.Context(), createChirpParam)
	if err != nil {
		log.Printf("error: something went wrong creating chirp: %s", err)
		w.WriteHeader(500)
		return
	}
	log.Print("CREATING CHIRP ON DB OK!!!")
	w.WriteHeader(201)
	w.Header().Set("Content-Type", "application/json")
	var chirpJson Chirp
	chirpJson.ID = chirp.ID
	chirpJson.CreatedAt = chirp.CreatedAt.Time
	chirpJson.Body = chirp.Body
	chirpJson.UpdatedAt = chirp.UpdatedAt.Time
	chirpJson.UserID = chirp.UserID

	chirpdat, err := json.Marshal(chirpJson)
	if err != nil {
		log.Printf("NEWCHIRPHANDLER:: error: something went wrong creating chirp: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Write(chirpdat)
	log.Print("WRITING CHIRPDAT OK")

	log.Print("####### NEWCHIRPHANDLER END!!!! ######")
}

func (cfg *apiConfig) getAllChirpsHandler(w http.ResponseWriter, r *http.Request) {
	allChirps, err := cfg.dbQ.GetAllChirps(r.Context())
	if err != nil {
		log.Printf("error: something went wrong getting all chirps: %s", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")

	var chirpArr []Chirp

	for _, ch := range allChirps {
		var chirpJson Chirp
		chirpJson.Body = ch.Body
		chirpJson.CreatedAt = ch.CreatedAt.Time
		chirpJson.ID = ch.ID
		chirpJson.UpdatedAt = ch.UpdatedAt.Time
		chirpJson.UserID = ch.UserID
		chirpArr = append(chirpArr, chirpJson)
	}

	chirpdat, err := json.Marshal(chirpArr)

	w.Write(chirpdat)

}

func (cfg *apiConfig) getAChirpHandler(w http.ResponseWriter, r *http.Request) {
	id_param := r.PathValue("id")
	fmt.Printf("id_param = %s", id_param)
	id, err := uuid.Parse(id_param)
	if err != nil {
		log.Printf("GETACHIRPHANDLER:: error: something went wrong: %s", err)
		w.WriteHeader(500)
		return
	}

	chirp, err := cfg.dbQ.GetAChirp(r.Context(), id)
	if err != nil {
		log.Printf("GETACHIRPHANDLER:: error: something went wrong: %s", err)
		w.WriteHeader(404)
		return
	}

	var chirpJson Chirp
	chirpJson.Body = chirp.Body
	chirpJson.CreatedAt = chirp.CreatedAt.Time
	chirpJson.ID = chirp.ID
	chirpJson.UpdatedAt = chirp.UpdatedAt.Time
	chirpJson.UserID = chirp.UserID

	chirpdat, err := json.Marshal(chirpJson)
	if err != nil {
		log.Printf("GETACHIRPHANDLER:: error: something went wrong: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Write(chirpdat)

}

// USER STUFF
type User struct {
	ID           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
}

// handler for creating user
func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, req *http.Request) {

	type bodyparam struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(req.Body)
	param := bodyparam{}
	err := decoder.Decode(&param)
	if err != nil {
		log.Printf("error: something went wrong: %s", err)
		w.WriteHeader(500)
		return
	}

	create_user_params := database.CreateUserParams{}
	create_user_params.Email = param.Email

	log.Printf("createUserHandler:: email = %s\n", param.Email)
	log.Printf("createUserHandler:: password = %s\n", param.Password)
	hashedPw, err := auth.HashPassword(param.Password)

	if err != nil {
		log.Printf("error: something went wrong: HashPassword: %s", err)
		w.WriteHeader(500)
		return
	}
	log.Printf("createUserHandler:: hashedPw =  %s", hashedPw)

	create_user_params.Userpassword = hashedPw

	user, err := cfg.dbQ.CreateUser(req.Context(), create_user_params)
	if err != nil {
		log.Printf("error: something went wrong creating user: %s", err)
		w.WriteHeader(500)
		return
	} else {
		w.WriteHeader(201)
		w.Header().Set("Content-Type", "application/json")
		var userJson User
		userJson.ID = user.ID
		userJson.CreatedAt = user.CreatedAt.Time
		userJson.Email = user.Email
		userJson.UpdatedAt = user.UpdatedAt.Time

		userdat, err := json.Marshal(userJson)
		if err != nil {
			log.Printf("error: something went wrong creating user: %s", err)
			w.WriteHeader(500)
			return
		} else {
			w.Write(userdat)
		}

	}

}

func (cfg *apiConfig) loginHandler(w http.ResponseWriter, r *http.Request) {
	log.Print("##### IN LOGIN HANDLER! #####\n")

	type bodyparam struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	param := bodyparam{}
	err := decoder.Decode(&param)
	if err != nil {
		log.Printf("error: something went wrong: %s", err)
		w.WriteHeader(500)
		return
	}

	log.Printf("loginHandler:: param.Email = %s", param.Email)
	log.Printf("loginHandler:: param.Password = %s", param.Password)

	//retrieve stored password hash from db

	user, err := cfg.dbQ.GetUserByEmail(r.Context(), param.Email)
	if err != nil {
		log.Printf("error: something went wrong getting uer password %s", err)
		w.WriteHeader(500)
		return
	}

	log.Printf("loginHandler:: user.Userpassword = %s", user.Userpassword)

	welp, err := auth.CheckPasswordHash(param.Password, user.Userpassword)
	if err != nil {
		log.Printf("error: something went wrong: CheckPasswordHash: %s", err)
		w.WriteHeader(401)
		return
	}

	if !welp {
		log.Printf("Incorrect email or password")
		w.WriteHeader(401)
		return
	}
	var userJson User
	userJson.CreatedAt = user.CreatedAt.Time
	userJson.Email = user.Email
	userJson.ID = user.ID
	userJson.UpdatedAt = user.UpdatedAt.Time

	token, err := auth.MakeJWT(userJson.ID, cfg.secret, time.Duration(1)*time.Hour)
	log.Printf("LOGINHANDLER: JWT TOKEN = %s \n", token)

	refresh_token := auth.MakeRefreshToken()
	log.Printf("LOGINHANDLER :: REFRESH_TOKEN = %s \n", refresh_token)
	refresh_token_params := database.CreateRefreshTokenParams{}
	refresh_token_params.Token = refresh_token //1
	sixty_days := time.Duration(60*24) * time.Hour
	expiry := sql.NullTime{
		Time:  time.Now().UTC().Local().Add(sixty_days),
		Valid: true,
	}
	refresh_token_params.ExpiresAt = expiry //2
	refresh_token_params.UserID = user.ID   //3
	refresh_token_from_db, err := cfg.dbQ.CreateRefreshToken(r.Context(), refresh_token_params)
	if err != nil {
		log.Printf("LOGINHANDLER: SOMETHING WENT WRONG WRITING REFRESH TOKEN TO DB %s", err)
	}

	userJson.Token = token
	userJson.RefreshToken = refresh_token_from_db.Token
	userdat, err := json.Marshal(userJson)
	if err != nil {
		log.Printf("error: something went wrong creating user: %d\n", err)
		w.WriteHeader(500)
		return
	}

	log.Printf("LOGINHANDLER USERDAT: %s \n", userdat)

	w.WriteHeader(200)
	w.Write(userdat)

}

func (cfg *apiConfig) refreshHandler(w http.ResponseWriter, r *http.Request) {
	//get refresh token in the header
	log.Print("###### AT REFRESHHANDLER!!! #####")

	log.Printf("REFRESHHANDLER CONTENT-LENGTH: %d", r.ContentLength)

	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("REFRESHHANDLER:: error: something went wrong getting bearer token: %s\n", err)
	}

	log.Printf("REFRESHHANDLER BEARER TOKEN: %s\n", bearerToken)
	log.Printf("IS BEARER TOKEN JWT? %t\n", IsJWT(bearerToken))
	//lookup refresh token in the database
	token, err := cfg.dbQ.GetRefreshToken(r.Context(), bearerToken)
	log.Printf("REFRESHHANDLER :: token from db: %s", token.Token)
	if err != nil {
		w.WriteHeader(401)
		return
	}
	log.Printf("REFRESHHANDLER:: ExpiresAt Valid %t\n", token.ExpiresAt.Valid)
	log.Printf("REFRESHHANDLER:: ExpiresAt Time %s\n", token.ExpiresAt.Time.String())

	if token.ExpiresAt.Valid && token.ExpiresAt.Time.UTC().Local().Before(time.Now().UTC().Local()) {
		w.WriteHeader(401)
		return
	}

	if token.RevokedAt.Valid && token.RevokedAt.Time.UTC().Local().Before(time.Now().UTC().Local()) {
		w.WriteHeader(401)
		return
	}

	//newtoken, err := auth.MakeJWT(userJson.ID, cfg.secret, time.Duration(3600))
	refresh_token := auth.MakeRefreshToken()
	refresh_token_params := database.CreateRefreshTokenParams{}
	refresh_token_params.Token = refresh_token
	one_hour := time.Duration(1) * time.Hour

	refresh_token_params.ExpiresAt.Time = time.Now().Add(one_hour)
	refresh_token_params.UserID = token.UserID
	saved_refresh_token, err := cfg.dbQ.CreateRefreshToken(r.Context(), refresh_token_params)
	if err != nil {
		log.Print("REFRESHHANDLER:: saving refresh token failed!")
		w.WriteHeader(500)
	}
	log.Print("REFRESHHANDLER:: new refresh token saved in DB!")

	type Token struct {
		Token string `json:"token"`
	}

	var tokeJson Token
	tokeJson.Token = saved_refresh_token.Token
	tokedat, err := json.Marshal(tokeJson)
	if err != nil {
		log.Print("error: something went wrong creating refresh token")
		w.WriteHeader(500)
		return
	}

	log.Printf("REFRESHHANDLER :: tokedat = %s\n", tokedat)
	w.WriteHeader(200)
	w.Write(tokedat)
}

func (cfg *apiConfig) revokeHandler(w http.ResponseWriter, r *http.Request) {

	log.Print("#### IN REVOKEHANDLER !!! ###")
	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("REVOKEHANDLER error: something went wrong getting bearer token: %s\n", err)
	}

	//lookup refresh token in the database
	token, err := cfg.dbQ.GetRefreshToken(r.Context(), bearerToken)
	if err != nil {
		w.WriteHeader(401)
		return
	}

	var revokeParams database.RevokeRefreshTokenParams
	revokeParams.Token = token.Token
	revokeParams.RevokedAt = sql.NullTime{
		Time:  time.Now(),
		Valid: true,
	}
	revoked_token, err := cfg.dbQ.RevokeRefreshToken(r.Context(), revokeParams)
	if err != nil {
		log.Printf("REVOKEHANDLER:: error revoking token in db! %s\n", err)
		w.WriteHeader(500)
	}
	log.Printf("REVOKEHANDLER:: revoked token: %s\n", revoked_token.Token)
	log.Printf("REVOKEHANDLER:: REVOKED AT %s\n", revoked_token.RevokedAt.Time)
	log.Print("### REVOKEHANDLER OK..###")
	w.WriteHeader(204)

}

func main() {

	//handle db
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	dbPLATFORM := os.Getenv("PLATFORM")
	SECRET := os.Getenv("SECRET")

	if dbPLATFORM != "dev" {

	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic(fmt.Sprintf("could open db connection: %s", err.Error()))

	}
	dbQueries := database.New(db)

	//end handle db
	mux := http.NewServeMux()

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	var apiCfg apiConfig
	apiCfg.dbQ = dbQueries
	apiCfg.dbPlatform = dbPLATFORM
	apiCfg.secret = SECRET

	fileServer := http.FileServer(http.Dir("."))
	mux.Handle("/app/", http.StripPrefix("/app", apiCfg.middlewareMetricsInc(fileServer)))

	mux.HandleFunc("GET /api/healthz", healthzHandler)

	mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)

	mux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)

	mux.HandleFunc("POST /api/users", apiCfg.createUserHandler)

	mux.HandleFunc("POST /api/login", apiCfg.loginHandler)

	mux.HandleFunc("POST /api/refresh", apiCfg.refreshHandler)

	mux.HandleFunc("POST /api/revoke", apiCfg.revokeHandler)

	mux.HandleFunc("POST /api/chirps", apiCfg.newChirpHandler)

	mux.HandleFunc("GET /api/chirps/", apiCfg.getAllChirpsHandler)

	mux.HandleFunc("GET /api/chirps/{id}", apiCfg.getAChirpHandler)
	if err := server.ListenAndServe(); err != nil {
		panic(fmt.Sprintf("could not start server: %s", err.Error()))
	}
}
