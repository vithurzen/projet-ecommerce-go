package main

import (
	"encoding/json"
	"log"
	"net/http"

	"ecommerce-go/internal/auth"
	"ecommerce-go/internal/database"
	"ecommerce-go/internal/orders"
	"ecommerce-go/internal/products"
)

func main() {

	// Connexion à SQLite
	db, err := database.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := database.InitSchema(db); err != nil {
		log.Fatal("Erreur initialisation de la base :", err)
	}

	log.Println("Table users prête")

	if err := products.InitSchema(db); err != nil {
		log.Fatal("Erreur initialisation de la table products :", err)
	}

	// Routeur HTTP
	mux := http.NewServeMux()
	auth.RegisterRoutes(mux, db)
	orders.RegisterRoutes(mux, db)

	// Route de test
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {

		if err := db.PingContext(r.Context()); err != nil {
			http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]string{
			"status":   "ok",
			"database": "connected",
		})
	})

	log.Println("Serveur démarré sur le port 8080")

	log.Fatal(http.ListenAndServe(":8080", mux))
}
