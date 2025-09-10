package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"shopping/config"
	"shopping/database"
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

var PORT = 8888
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

func main() {
	config := config.SetupConfig()
	dbpool, err := database.NewDB(config)
	if err != nil {
		log.Fatal().Msgf("Cannot connect to the database")
	}
	defer dbpool.Close()

	http.HandleFunc("POST /v1/lists", adminRequired(handleCreateList))
	http.HandleFunc("GET /v1/lists", authRequired(handleGetLists))
	http.HandleFunc("PUT /v1/lists/{id}", adminRequired(handleUpdateList))
	http.HandleFunc("DELETE /v1/lists/{id}", adminRequired(handleDeleteList))
	http.HandleFunc("PATCH /v1/lists/{id}", adminRequired(handlePatchList))
	http.HandleFunc("GET /v1/lists/{id}", authRequired(handleGetList))
	http.HandleFunc("POST /v1/lists/{id}/push", adminRequired(handleListPush))

	http.HandleFunc("POST /v1/login", handleLogin)

	log.Info().Msgf("> Server running on http://localhost:%d\n", PORT)
	// this blocks the thread so code after this line will not run
	err = http.ListenAndServe(fmt.Sprintf(":%d", PORT), nil)
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

func authRequired(next http.HandlerFunc) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if !strings.HasPrefix(token, "Bearer") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		token = token[7:]

		if sessions[token] == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if sessions[token].Expires.Before(time.Now()) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		userSession := sessions[token]
		user := allUsers[userSession.Username]

		if user == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}

	return fn
}

func adminRequired(next http.HandlerFunc) http.HandlerFunc {
	return authRequired(func(w http.ResponseWriter, r *http.Request) {
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
