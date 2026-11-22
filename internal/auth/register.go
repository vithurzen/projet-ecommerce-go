package auth

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Données reçues lors de l'inscription
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Ajouter la route d'inscription au serveur
func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("POST /api/auth/register", func(w http.ResponseWriter, r *http.Request) {

		var input RegisterRequest

		// Lire le JSON envoyé par le client
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		err := json.NewDecoder(r.Body).Decode(&input)
		if err != nil {
			http.Error(w, "JSON invalide", http.StatusBadRequest)
			return
		}

		// Vérifier l'email
		email := strings.ToLower(strings.TrimSpace(input.Email))
		adresse, err := mail.ParseAddress(email)
		if err != nil || adresse.Address != email {
			http.Error(w, "Email invalide", http.StatusBadRequest)
			return
		}

		// Vérifier le mot de passe
		if len(input.Password) < 8 || len(input.Password) > 72 {
			http.Error(w, "Mot de passe : 8 à 72 octets", http.StatusBadRequest)
			return
		}

		// Hacher le mot de passe
		hash, err := bcrypt.GenerateFromPassword(
			[]byte(input.Password),
			bcrypt.DefaultCost,
		)
		if err != nil {
			http.Error(w, "Erreur serveur", http.StatusInternalServerError)
			return
		}

		// Ajouter le compte dans SQLite
		result, err := db.ExecContext(
			r.Context(),
			`INSERT INTO users (email, password_hash, role, confirmed)
             VALUES (?, ?, 'client', 0)
             ON CONFLICT(email) DO NOTHING`,
			email, string(hash),
		)

		if err != nil {
			http.Error(w, "Erreur base de données", http.StatusInternalServerError)
			return
		}

		// Vérifier si cet email existe déjà
		count, err := result.RowsAffected()
		if err != nil {
			http.Error(w, "Erreur serveur", http.StatusInternalServerError)
			return
		}
		if count == 0 {
			http.Error(w, "Email déjà utilisé", http.StatusConflict)
			return
		}

		// Réponse après inscription
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		json.NewEncoder(w).Encode(map[string]string{
			"message": "Compte créé, confirmation nécessaire",
		})
	})
}
