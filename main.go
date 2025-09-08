package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
)

type ShoppingList struct {
	ID    int      `json:"id"`
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

var PORT = 8888
var allData []ShoppingList = []ShoppingList{}

func main() {
	http.HandleFunc("POST /v1/lists", handleCreateList)
	http.HandleFunc("GET /v1/lists", handleGetLists)
	http.HandleFunc("DELETE /v1/lists/{id}", handleDeleteList)
	http.HandleFunc("PUT /v1/lists/{id}", handleUpdateList)
	http.HandleFunc("PATCH /v1/lists/{id}", handlePatchList)
	http.HandleFunc("GET /v1/lists/{id}", handleGetList)
	http.HandleFunc("POST /v1/lists/{id}/push", handleListPush)

	log.Info().Msgf("> Server running on http://localhost:%d\n", PORT)
	// this blocks the thread so code after this line will not run
	err := http.ListenAndServe(fmt.Sprintf(":%d", PORT), nil)
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
