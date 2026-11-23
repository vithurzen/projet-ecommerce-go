package orders

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("GET /api/users/{userID}/orders", listOrdersHandler(db))
	mux.HandleFunc("GET /api/users/{userID}/orders/{reference}", getOrderHandler(db))
}

func listOrdersHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
		if err != nil {
			http.Error(w, "Identifiant utilisateur invalide", http.StatusBadRequest)
			return
		}

		userOrders, err := ListOrders(db, userID)
		if err != nil {
			http.Error(w, "Erreur serveur", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(userOrders)
	}
}

func getOrderHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
		if err != nil {
			http.Error(w, "Identifiant utilisateur invalide", http.StatusBadRequest)
			return
		}

		order, err := GetOrder(db, userID, r.PathValue("reference"))
		if errors.Is(err, ErrOrderNotFound) {
			http.Error(w, "Commande introuvable", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Erreur serveur", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(order)
	}
}
