package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"
)

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// Créer un code secret de 16 octets (32 caractères)
func generateResetCode() (string, string, error) {
	randomBytes := make([]byte, 16)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", err
	}

	code := strings.ToUpper(hex.EncodeToString(randomBytes))
	return code, hashResetCode(code), nil
}

// Calculer le hash du code
func hashResetCode(code string) string {
	hash := sha256.Sum256([]byte(strings.ToUpper(code)))
	return hex.EncodeToString(hash[:])
}

// Réponse identique même si l'email n'existe pas
func resetRequestedResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]string{
		"message": "Si le compte existe, un code a été demandé",
	})
}

func forgotPasswordHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input ForgotPasswordRequest

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

		// Chercher l'utilisateur
		var userID int64

		err = db.QueryRowContext(r.Context(),
			"SELECT id FROM users WHERE email = ?",
			email,
		).Scan(&userID)

		if errors.Is(err, sql.ErrNoRows) {
			resetRequestedResponse(w)
			return
		}

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		// Générer le code de réinitialisation
		code, codeHash, err := generateResetCode()
		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		expiresAt := time.Now().UTC().Add(15 * time.Minute)

		// Enregistrer ou remplacer le code existant
		_, err = db.ExecContext(r.Context(), `
            INSERT INTO password_reset_codes
                (user_id, code_hash, expires_at)
            VALUES (?, ?, ?)
            ON CONFLICT(user_id) DO UPDATE SET
                code_hash = excluded.code_hash,
                expires_at = excluded.expires_at
        `, userID, codeHash, expiresAt.Format(time.RFC3339))

		if err != nil {
			http.Error(w, "Erreur base de données", 500)
			return
		}

		// Afficher le code seulement en développement local
		if os.Getenv("APP_ENV") == "development" {
			log.Printf("DEV - Reset pour %s : %s", email, code)
		}

		resetRequestedResponse(w)
	}
}

// Enregistrer les deux routes
func RegisterPasswordResetRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc(
		"POST /api/auth/forgot-password",
		forgotPasswordHandler(db),
	)

	mux.HandleFunc(
		"POST /api/auth/reset-password",
		resetPasswordHandler(db),
	)
}
