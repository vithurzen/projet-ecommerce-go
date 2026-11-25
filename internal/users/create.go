package users

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Informations reçues pour créer un utilisateur
type CreateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func createUser(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Lire les données JSON
		var input CreateUserRequest

		r.Body = http.MaxBytesReader(w, r.Body, 4096)

		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "JSON invalide", http.StatusBadRequest)
			return
		}

		// Vérifier l'adresse email
		email := strings.ToLower(strings.TrimSpace(input.Email))

		address, err := mail.ParseAddress(email)

		if err != nil || address.Address != email {
			http.Error(w, "Email invalide", http.StatusBadRequest)
			return
		}

		// Vérifier le mot de passe
		if len(input.Password) < 8 || len(input.Password) > 72 {
			http.Error(w, "Mot de passe invalide", http.StatusBadRequest)
			return
		}

		// Hacher le mot de passe
		passwordHash, err := bcrypt.GenerateFromPassword(
			[]byte(input.Password),
			bcrypt.DefaultCost,
		)

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		// Enregistrer le nouvel utilisateur
		result, err := db.ExecContext(r.Context(), `
            INSERT INTO users
                (email, password_hash, role, confirmed)
            VALUES (?, ?, 'client', 0)
            ON CONFLICT(email) DO NOTHING
        `, email, string(passwordHash))

		if err != nil {
			http.Error(w, "Erreur base de données", 500)
			return
		}

		// Vérifier si l'utilisateur a été créé
		affected, err := result.RowsAffected()

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		if affected == 0 {
			http.Error(w, "Email déjà utilisé", 409)
			return
		}

		// Récupérer l'identifiant créé
		userID, err := result.LastInsertId()

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		// Renvoyer la réponse
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":        userID,
			"email":     email,
			"role":      "client",
			"confirmed": false,
		})
	})
}
