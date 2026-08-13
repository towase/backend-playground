package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/towase/backend-playground/experiments/reorder/full-reindex/internal/item"
)

type server struct {
	store *item.SQLiteStore
}

type reorderRequest struct {
	PreviousID *string `json:"previousId"`
}

func main() {
	databasePath := os.Getenv("DATABASE_PATH")
	if databasePath == "" {
		databasePath = "reorder.db"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	store, err := item.OpenSQLiteStore(context.Background(), databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	if err := store.SeedIfEmpty(context.Background(), []item.Item{
		{ID: "A", Name: "alpha", Position: 1},
		{ID: "B", Name: "bravo", Position: 2},
		{ID: "C", Name: "charlie", Position: 3},
		{ID: "D", Name: "delta", Position: 4},
		{ID: "E", Name: "echo", Position: 5},
	}); err != nil {
		log.Fatal(err)
	}
	app := &server{store: store}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /items", app.listItems)
	mux.HandleFunc("PATCH /items/{id}/position", app.reorderItem)

	log.Printf("listening on http://localhost:%s (database: %s)", port, databasePath)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func (s *server) listItems(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *server) reorderItem(w http.ResponseWriter, r *http.Request) {
	var request reorderRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	err := s.store.Reorder(r.Context(), r.PathValue("id"), request.PreviousID)
	switch {
	case errors.Is(err, item.ErrItemNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, item.ErrPreviousSame):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	default:
		items, err := s.store.List(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		writeJSON(w, http.StatusOK, items)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
