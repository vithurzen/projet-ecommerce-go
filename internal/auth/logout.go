package auth

import (
	"database/sql"
	"net/http"
	"strings"
)

// Enregistrer la route de déconnexion
func RegisterLogoutRoutes(mux *http.ServeMux, db *sql.DB) {

	mux.HandleFunc("POST /api/auth/logout",
		func(w http.ResponseWriter, r *http.Request) {

			// 1. Vérifier que la session est valide
			user, ok := ValidateSession(w, r, db)

			if !ok {
				return
			}

			// 2. Récupérer le token
			parts := strings.Fields(
				r.Header.Get("Authorization"),
			)

			// ValidateSession a déjà vérifié son format
			token := parts[1]

			// 3. Supprimer cette session dans SQLite
			result, err := db.ExecContext(r.Context(), `
                DELETE FROM sessions
                WHERE user_id = ?
                  AND token_hash = ?
            `, user.ID, sessionHash(token))

			if err != nil {
				http.Error(w, "Erreur serveur", 500)
				return
			}

			// 4. Vérifier que la session a été supprimée
			affected, err := result.RowsAffected()

			if err != nil {
				http.Error(w, "Erreur serveur", 500)
				return
			}

			if affected == 0 {
				http.Error(w, "Session introuvable", 401)
				return
			}

			// 5. Déconnexion réussie
			w.WriteHeader(http.StatusNoContent)
		},
	)
}
