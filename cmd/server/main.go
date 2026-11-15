package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	// Création du routeur HTTP
	mux := http.NewServeMux()

	// Route pour tester le serveur
	mux.HandleFunc("GET /health", healthHandler)

	// Démarrage du serveur
	log.Println("Serveur démarré sur le port 8080")

	err := http.ListenAndServe(":8080", mux)

	if err != nil {
		log.Fatal(err)
	}
}

// Fonction appelée lorsqu'on visite /health
func healthHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	response := map[string]string{
		"status":  "ok",
		"message": "Serveur e-commerce opérationnel",
	}

	json.NewEncoder(w).Encode(response)
}
