package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/towase/backend-playground/experiments/reorder-api/internal/item"
)

type server struct {
	store *item.Store
}

type reorderRequest struct {
	PreviousID *string `json:"previousId"`
}

func main() {
	store := item.NewStore([]item.Item{
		{ID: "A", Name: "alpha", Position: 1},
		{ID: "B", Name: "bravo", Position: 2},
		{ID: "C", Name: "charlie", Position: 3},
		{ID: "D", Name: "delta", Position: 4},
		{ID: "E", Name: "echo", Position: 5},
	})
	app := &server{store: store}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /items", app.listItems)
	mux.HandleFunc("PATCH /items/{id}/position", app.reorderItem)

	log.Println("listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

func (s *server) listItems(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.List())
}

func (s *server) reorderItem(w http.ResponseWriter, r *http.Request) {
	var request reorderRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	err := s.store.Reorder(r.PathValue("id"), request.PreviousID)
	switch {
	case errors.Is(err, item.ErrItemNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, item.ErrPreviousSame):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	default:
		writeJSON(w, http.StatusOK, s.store.List())
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
