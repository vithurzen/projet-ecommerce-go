package users

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"ecommerce-go/internal/auth"
)

// Informations publiques d'un utilisateur
type UserResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Confirmed bool   `json:"confirmed"`
	CreatedAt string `json:"created_at"`
}

// Enregistrer les routes administrateur
func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {

	// Afficher les utilisateurs
	mux.Handle(
		"GET /api/admin/users",
		auth.RequireAdmin(db, listUsers(db)),
	)

	// Créer un utilisateur
	mux.Handle(
		"POST /api/admin/users",
		auth.RequireAdmin(db, createUser(db)),
	)
}

// Afficher tous les utilisateurs
func listUsers(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		rows, err := db.QueryContext(r.Context(), `
            SELECT id, email, role, confirmed, created_at
            FROM users
            ORDER BY id
        `)

		if err != nil {
			log.Printf("Erreur SQL : %v", err)
			http.Error(w, "Erreur serveur", 500)
			return
		}
		defer rows.Close()

		users := make([]UserResponse, 0)

		for rows.Next() {
			var user UserResponse
			var confirmed int

			err := rows.Scan(
				&user.ID,
				&user.Email,
				&user.Role,
				&confirmed,
				&user.CreatedAt,
			)

			if err != nil {
				http.Error(w, "Erreur serveur", 500)
				return
			}

			user.Confirmed = confirmed == 1
			users = append(users, user)
		}

		if err := rows.Err(); err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	})
}
