package auth

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("POST /api/auth/register", func(w http.ResponseWriter, r *http.Request) {
		var input RegisterRequest

		r.Body = http.MaxBytesReader(w, r.Body, 4096)

		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "JSON invalide", 400)
			return
		}

		email := strings.ToLower(strings.TrimSpace(input.Email))

		address, err := mail.ParseAddress(email)
		if err != nil || address.Address != email {
			http.Error(w, "Email invalide", 400)
			return
		}

		if len(input.Password) < 8 || len(input.Password) > 72 {
			http.Error(w, "Mot de passe invalide (8 à 72 octets)", 400)
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

		// Générer le code de confirmation
		code, codeHash, err := generateConfirmationCode()
		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		// Insérer utilisateur et code dans une transaction
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}
		defer tx.Rollback()

		result, err := tx.ExecContext(r.Context(), `
            INSERT INTO users (email, password_hash, role, confirmed)
            VALUES (?, ?, 'client', 0)
            ON CONFLICT(email) DO NOTHING
        `, email, string(passwordHash))

		if err != nil {
			http.Error(w, "Erreur base de données", 500)
			return
		}

		affected, err := result.RowsAffected()
		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}
		if affected == 0 {
			http.Error(w, "Email déjà utilisé", 409)
			return
		}

		userID, err := result.LastInsertId()
		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		expiresAt := time.Now().UTC().Add(15 * time.Minute)

		_, err = tx.ExecContext(r.Context(), `
            INSERT INTO confirmation_codes
                (user_id, code_hash, expires_at)
            VALUES (?, ?, ?)
        `, userID, codeHash, expiresAt.Format(time.RFC3339))

		if err != nil {
			http.Error(w, "Erreur création du code", 500)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		// Uniquement pour les tests locaux
		if os.Getenv("APP_ENV") == "development" {
			log.Printf("DEV - Code de confirmation pour %s : %s", email, code)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		json.NewEncoder(w).Encode(map[string]string{
			"message": "Compte créé, confirmation nécessaire",
		})
	})

	// Enregistrer la route de confirmation
	mux.HandleFunc("POST /api/auth/confirm", confirmHandler(db))
}
