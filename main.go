package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"shopping/config"
	"shopping/database"
	db_queries "shopping/database/queries"
	"shopping/repository"
	"slices"
	"strconv"
	"strings"
	"time"

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
}

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

	app := App{
		DBQueries:              dbQueries,
		Config:                 config,
		SessionRepository:      sessionRepo,
		ShoppingListRepository: shoppingListRepo,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/lists", app.adminRequired(handleCreateList))
	mux.HandleFunc("GET /v1/lists", app.authRequired(handleGetLists))
	mux.HandleFunc("PUT /v1/lists/{id}", app.adminRequired(handleUpdateList))
	mux.HandleFunc("DELETE /v1/lists/{id}", app.adminRequired(handleDeleteList))
	mux.HandleFunc("PATCH /v1/lists/{id}", app.adminRequired(handlePatchList))
	mux.HandleFunc("GET /v1/lists/{id}", app.authRequired(handleGetList))
	mux.HandleFunc("POST /v1/lists/{id}/push", app.adminRequired(handleListPush))

	mux.HandleFunc("POST /v1/login", handleLogin)

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

func handleCreateList(w http.ResponseWriter, r *http.Request) {
	var newList ShoppingList
	err := json.NewDecoder(r.Body).Decode(&newList)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	newList.ID = len(allData) + 1

	allData = append(allData, newList)
	w.WriteHeader(http.StatusCreated)

	// encode automatically sets the content type to application/json
	// more memory efficient for large objects instead of using json.Marshal + w.Header().Set + w.Write()
	// its recommended over the manually marshal, write etc
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(newList)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func handleGetLists(w http.ResponseWriter, r *http.Request) {
	data, err := json.Marshal(allData)
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

func handleDeleteList(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	for i, list := range allData {
		if strconv.Itoa(list.ID) == id {
			allData = append(allData[:i], allData[i+1:]...)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	http.Error(w, "List not found", http.StatusNotFound)
}

func handleUpdateList(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	for i, list := range allData {
		if strconv.Itoa(list.ID) == id {
			var updatedList ShoppingList
			err := json.NewDecoder(r.Body).Decode(&updatedList)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			allData[i] = updatedList
			if err := json.NewEncoder(w).Encode(updatedList); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			return
		}
	}

	http.Error(w, "List not found", http.StatusNotFound)
}

type ShoppingListPatch struct {
	Name  *string   `json:"name"`
	Items *[]string `json:"items"`
}

func handlePatchList(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	for i, list := range allData {
		if strconv.Itoa(list.ID) == id {
			var patch ShoppingListPatch

			err := json.NewDecoder(r.Body).Decode(&patch)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if patch.Name != nil {
				list.Name = *patch.Name
			}
			if patch.Items != nil {
				list.Items = *patch.Items
			}

			// this is needed because we are not modifying the reference
			allData[i] = list

			w.Header().Set("Content-Type", "application/json")
			err = json.NewEncoder(w).Encode(list)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			return
		}
	}

	http.Error(w, "list not found", http.StatusNotFound)
}

func handleGetList(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	for _, list := range allData {
		if strconv.Itoa(list.ID) == id {
			w.Header().Set("Content-Type", "application/json")

			// err := json.NewEncoder(w).Encode(list)
			// if err != nil {
			// 	http.Error(w, err.Error(), http.StatusInternalServerError)
			// 	return
			// }

			// option 2
			data, err := json.Marshal(list)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			_, err = w.Write(data)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			return
		}
	}

	http.Error(w, "list not found", http.StatusNotFound)
}

type ListPushAction struct {
	Item string `json:"item"`
}

func handleListPush(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	for i, list := range allData {
		if strconv.Itoa(list.ID) == id {
			w.Header().Set("Content-Type", "application/json")

			var item ListPushAction
			err := json.NewDecoder(r.Body).Decode(&item)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			list.Items = append(list.Items, item.Item)
			allData[i] = list

			err = json.NewEncoder(w).Encode(list)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			return
		}
	}

	http.Error(w, "list not found", http.StatusNotFound)

}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var data LoginRequest
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user := allUsers[data.Username]
	if user != nil && user.Password == data.Password {
		token := strconv.Itoa(rand.Intn(100000000000))
		sessions[token] = &Session{
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			Username: user.Username,
		}

		w.Header().Set("Content-Type", "application/json")

		err := json.NewEncoder(w).Encode(map[string]string{"token": token})
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
		user := allUsers[sessions[token].Username]

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
