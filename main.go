package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"shopping/config"
	"shopping/database"
	db_queries "shopping/database/queries"
	"shopping/repository"
	"slices"
	"strings"
	"time"

	"shopping/docs"

	httpSwagger "github.com/swaggo/http-swagger"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/zerolog/log"
)

type ShoppingList struct {
	ID    int      `json:"id"`
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

var allData []ShoppingList = []ShoppingList{}

type User struct {
	Role     string
	Username string
	Password string
}

type Session struct {
	Expires  time.Time
	Username string
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var sessions = map[string]*Session{}

var allUsers = map[string]*User{
	"admin": {Role: "admin", Username: "admin", Password: "password"},
	"user":  {Role: "user", Username: "user", Password: "password"},
}

type App struct {
	DBQueries              *db_queries.Queries
	Config                 *config.Config
	SessionRepository      repository.SessionRepository
	ShoppingListRepository repository.ShoppingListRepository
	ListsCache             *lru.Cache[string, *db_queries.ShoppingList]
}

// @title Shopping List API
// @version 0.1
// @description Shopping list api with CRUD operations

// @host localhost:8080
// @BasePath /v1

// @securityDefinitions.authToken AuthToken
// @in header
// @name Authorization
// @description Send the jwt auth token in the Authorization token like `Authorization: Bearer <token>`
func main() {
	config := config.SetupConfig()
	dbpool, err := database.NewDB(config)
	if err != nil {
		log.Fatal().Msgf("Cannot connect to the database")
	}
	defer dbpool.Close()

	dbQueries := db_queries.New(dbpool)

	// repositories
	sessionRepo := repository.NewSessionRepository(dbQueries)
	shoppingListRepo := repository.NewShoppingListRepository(dbQueries)

	listsCache, err := lru.New[string, *db_queries.ShoppingList](128)
	if err != nil {
		log.Err(err).Msg("Unable to initialize the lists cache")
		os.Exit(1)
	}

	app := App{
		DBQueries:              dbQueries,
		Config:                 config,
		SessionRepository:      sessionRepo,
		ShoppingListRepository: shoppingListRepo,
		ListsCache:             listsCache,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/lists", app.addCacheHeaders(app.authRequired(app.handleCreateList)))
	mux.HandleFunc("GET /v1/lists", app.authRequired(app.handleGetLists))
	mux.HandleFunc("PUT /v1/lists/{id}", app.adminRequired(app.handleUpdateList))
	mux.HandleFunc("DELETE /v1/lists/{id}", app.adminRequired(app.handleDeleteList))
	mux.HandleFunc("PATCH /v1/lists/{id}", app.adminRequired(app.handlePatchList))
	mux.HandleFunc("GET /v1/lists/{id}", app.authRequired(app.handleGetList))
	mux.HandleFunc("POST /v1/lists/{id}/push", app.adminRequired(app.handleListPush))

	mux.HandleFunc("POST /v1/login", app.handleLogin)

	mux.HandleFunc("GET /v1/swagger/", httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8080/v1/swagger/doc.json"),
	))
	mux.HandleFunc("GET /v1/swagger/doc.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(docs.SwaggerInfo.ReadDoc()))
		if err != nil {
			http.Error(w, "Failed to write response", http.StatusInternalServerError)
			return
		}
	})

	handler := app.enableCors(mux)

	// certManager := autocert.Manager{
	// 	Prompt:     autocert.AcceptTOS,
	// 	HostPolicy: autocert.HostWhitelist("ourdomain.com"),
	// 	Cache:      autocert.DirCache("certs"),
	// }

	// server := &http.Server{
	// 	Addr:      ":https",
	// 	Handler:   handler,
	// 	TLSConfig: certManager.TLSConfig(),
	// }

	// go http.ListenAndServe(fmt.Sprintf(":%d", PORT), certManager.HTTPHandler(nil))
	// server.ListenAndServeTLS("", "")

	log.Info().Msgf("> Server running on http://localhost:%d\n", config.Port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", config.Port), handler)
	if err != nil {
		panic(err)
	}

}

type CreateShoppingListRequest struct {
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

func (app *App) handleCreateList(w http.ResponseWriter, r *http.Request) {

	var newList CreateShoppingListRequest
	err := json.NewDecoder(r.Body).Decode(&newList)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	newShoppingList, err := app.ShoppingListRepository.CreateShoppingList(newList.Name, newList.Items)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)

	// encode automatically sets the content type to application/json
	// more memory efficient for large objects instead of using json.Marshal + w.Header().Set + w.Write()
	// its recommended over the manually marshal, write etc
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(newShoppingList)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetShoppingLists godoc
// @Summary Get all shopping lists
// @Description Retrieve all shopping lists from the database
// @Tags shopping-lists
// @Accept json
// @Produce json
// @Security AuthToken
// @Success 200 {array} object "List of shopping lists" example:[{"id":"123e4567-e89b-12d3-a456-426614174000","name":"Grocery List","items":["milk","bread","eggs"],"created_at":"2023-01-01T00:00:00Z","updated_at":"2023-01-01T00:00:00Z"}]
// @Failure 401 {object} map[string]string "Unauthorized - Invalid or missing token"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /lists [get]
func (app *App) handleGetLists(w http.ResponseWriter, r *http.Request) {
	lists, err := app.ShoppingListRepository.GetAllShoppingLists()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(lists)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (app *App) handleDeleteList(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	err := app.ShoppingListRepository.DeleteShoppingListByID(id)
	if err != nil {
		http.Error(w, "list not found", http.StatusInternalServerError)
		return
	}

	app.ListsCache.Remove(id)

	w.WriteHeader(http.StatusNoContent)
}

type updateListRequest struct {
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

func (app *App) handleUpdateList(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var bodyData updateListRequest
	err := json.NewDecoder(r.Body).Decode(&bodyData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	updatedList, err := app.ShoppingListRepository.UpdateShoppingListByID(
		id,
		bodyData.Name,
		bodyData.Items,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	app.ListsCache.Remove(id)

	// w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(updatedList)
	if err != nil {
		log.Err(err).Msgf("failed to encode updated list data with id: %s", id)
		http.Error(w, "failed to parse data", http.StatusInternalServerError)
		return
	}

}

type ShoppingListPatch struct {
	Name  *string   `json:"name"`
	Items *[]string `json:"items"`
}

func (app *App) handlePatchList(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var data ShoppingListPatch
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, "invalid data", http.StatusBadRequest)
		return
	}

	updated, err := app.ShoppingListRepository.PartialUpdate(
		id,
		data.Name,
		data.Items,
	)
	if err != nil {
		log.Err(err).Msgf("error to patch update the list with id: %s", id)
		http.Error(w, "list not found", http.StatusNotFound)
		return
	}

	app.ListsCache.Remove(id)

	err = json.NewEncoder(w).Encode(updated)
	if err != nil {
		log.Err(err).Msgf("failed to parse the updated data: %+v", updated)
		http.Error(w, "failed to parse data", http.StatusInternalServerError)
		return
	}
}

func (app *App) handleGetList(w http.ResponseWriter, r *http.Request) {
	var err error
	id := r.PathValue("id")

	// check cache first
	list, ok := app.ListsCache.Get(id)
	if !ok {
		list, err = app.ShoppingListRepository.GetShoppingListByID(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		app.ListsCache.Add(id, list)
	}

	data, err := json.Marshal(list)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-cache")

	etag := fmt.Sprintf(`"%x"`, sha256.Sum256(data))
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Etag", etag)

	_, err = w.Write(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type ListPushAction struct {
	Item string `json:"item"`
}

func (app *App) handleListPush(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var data ListPushAction
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, "invalid data", http.StatusBadRequest)
		return
	}

	updated, err := app.ShoppingListRepository.PushItemToShoppingList(
		id,
		data.Item,
	)
	if err != nil {
		http.Error(w, "list not found", http.StatusNotFound)
		return
	}

	err = json.NewEncoder(w).Encode(updated)
	if err != nil {
		http.Error(w, "error to process data", http.StatusInternalServerError)
		return
	}
}

func (app *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var data LoginRequest
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user := allUsers[data.Username]
	if user != nil && user.Password == data.Password {
		session, err := app.SessionRepository.AddSession(user.Username)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		err = json.NewEncoder(w).Encode(map[string]string{"token": session.Token})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		return
	}

	http.Error(w, "invalid credentials", http.StatusUnauthorized)
}

func (app *App) authRequired(next http.HandlerFunc) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if !strings.HasPrefix(token, "Bearer") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		token = token[7:]

		_, err := app.SessionRepository.GetSessionByToken(token)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}

	return fn
}

func (app *App) adminRequired(next http.HandlerFunc) http.HandlerFunc {
	return app.authRequired(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		token = token[7:]
		session, err := app.SessionRepository.GetSessionByToken(token)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		user := allUsers[session.Username]

		if user.Role != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next(w, r)
	})
}

func (app *App) enableCors(next http.Handler) http.Handler {
	trustedOrigins := []string{
		"http://localhost:9000",
		"http://localhost:9002",
		"http://localhost:3000",
	}
	allowedMethods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
	}

	allowedHeaders := []string{
		"Authorization",
		"Content-Type",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		w.Header().Add("Vary", "Access-Control-Request-Method")

		origin := r.Header.Get("Origin")

		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		if slices.Contains(trustedOrigins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)

			// check if the request has the HTTP method OPTIONS and contains
			// the "Access-Control-Request-Method" header. If it does, then we treat
			// it as a preflight request.
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				requestMethod := r.Header.Get("Access-Control-Request-Method")
				if !slices.Contains(allowedMethods, requestMethod) {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}

				requestedHeaders := r.Header.Get("Access-Control-Request-Headers")
				if requestedHeaders != "" {
					headerList := strings.Split(requestedHeaders, ",")
					for _, header := range headerList {
						header := strings.TrimSpace(header)
						if !slices.Contains(allowedHeaders, header) {
							w.WriteHeader(http.StatusForbidden)
							return
						}
					}
				}

				// set the necessary preflight response headers
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(allowedHeaders, ", "))
				// cache preflight requests for 5 minutes
				// preflight requests add latency since the browser has to make an extra round-trip before the actual request
				// caching them for 300seconds is a reasonable default that balances performance with flexibility
				w.Header().Set("Access-Control-Max-Age", "300")

				// write the headers along with a 200 ok status and return from
				// the middleware with no further action
				// set 200 ok and not 204 because some browsers doesn't support 204
				w.WriteHeader(http.StatusOK)
				return
			}

		}

		next.ServeHTTP(w, r)

	})
}

func (app *App) addCacheHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("Expires", time.Now().Add(5*time.Minute).Format(http.TimeFormat))

		next(w, r)
	}
}
