package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Données envoyées lors de la connexion
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Calculer l'empreinte d'un token
func sessionHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// Enregistrer les routes de connexion
func RegisterLoginRoutes(mux *http.ServeMux, db *sql.DB) {

	// 1. Se connecter
	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var input LoginRequest

		r.Body = http.MaxBytesReader(w, r.Body, 4096)

		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "JSON invalide", http.StatusBadRequest)
			return
		}

		email := strings.ToLower(strings.TrimSpace(input.Email))

		if email == "" || input.Password == "" {
			http.Error(w, "Email et mot de passe obligatoires", 400)
			return
		}

		// Chercher le compte dans SQLite
		var userID int64
		var storedHash string
		var confirmed int

		err := db.QueryRowContext(r.Context(), `
            SELECT id, password_hash, confirmed
            FROM users
            WHERE email = ?
        `, email).Scan(&userID, &storedHash, &confirmed)

		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Identifiants incorrects", 401)
			return
		}

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		// Vérifier le mot de passe
		err = bcrypt.CompareHashAndPassword(
			[]byte(storedHash),
			[]byte(input.Password),
		)

		if err != nil {
			http.Error(w, "Identifiants incorrects", 401)
			return
		}

		// Vérifier que le compte est confirmé
		if confirmed != 1 {
			http.Error(w, "Compte non confirmé", 403)
			return
		}

		// Générer un token secret aléatoire
		randomBytes := make([]byte, 32)

		if _, err := rand.Read(randomBytes); err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		token := hex.EncodeToString(randomBytes)
		expiresAt := time.Now().UTC().Add(24 * time.Hour)

		// Enregistrer uniquement le hash du token
		_, err = db.ExecContext(r.Context(), `
            INSERT INTO sessions
                (user_id, token_hash, expires_at)
            VALUES (?, ?, ?)
        `,
			userID,
			sessionHash(token),
			expiresAt.Format(time.RFC3339),
		)

		if err != nil {
			http.Error(w, "Erreur création session", 500)
			return
		}

		// Envoyer le token à l'application client
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]string{
			"message":    "Connexion réussie",
			"token":      token,
			"token_type": "Bearer",
			"expires_at": expiresAt.Format(time.RFC3339),
		})
	})

	// 2. Obtenir le compte connecté
	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {

		parts := strings.Fields(r.Header.Get("Authorization"))

		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "Token manquant", 401)
			return
		}

		token := parts[1]

		// Vérifier le format du token
		if len(token) != 64 {
			http.Error(w, "Token invalide", 401)
			return
		}

		if _, err := hex.DecodeString(token); err != nil {
			http.Error(w, "Token invalide", 401)
			return
		}

		var userID int64
		var email, role, expiration string

		err := db.QueryRowContext(r.Context(), `
            SELECT u.id, u.email, u.role, s.expires_at
            FROM sessions s
            JOIN users u ON u.id = s.user_id
            WHERE s.token_hash = ?
              AND u.confirmed = 1
        `, sessionHash(token)).Scan(
			&userID, &email, &role, &expiration,
		)

		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Session invalide", 401)
			return
		}

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		expiresAt, err := time.Parse(time.RFC3339, expiration)

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		if !time.Now().Before(expiresAt) {
			http.Error(w, "Session expirée", 401)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    userID,
			"email": email,
			"role":  role,
		})
	})
}
