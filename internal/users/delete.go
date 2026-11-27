package users

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"ecommerce-go/internal/auth"
)

func deleteUser(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// 1. Récupérer l'ID dans l'URL
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

		if err != nil || id <= 0 {
			http.Error(w, "Identifiant invalide", 400)
			return
		}

		// 2. Récupérer l'administrateur connecté
		currentUser, ok := auth.UserFromContext(r)

		if !ok {
			http.Error(w, "Authentification requise", 401)
			return
		}

		// 3. Empêcher la suppression de son propre compte
		if currentUser.ID == id {
			http.Error(w, "Impossible de supprimer son propre compte", 403)
			return
		}

		// 4. Démarrer une transaction SQL
		tx, err := db.BeginTx(r.Context(), nil)

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}
		defer tx.Rollback()

		// 5. Vérifier que l'utilisateur existe
		var role string

		err = tx.QueryRowContext(r.Context(), `
            SELECT role
            FROM users
            WHERE id = ?
        `, id).Scan(&role)

		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Utilisateur introuvable", 404)
			return
		}

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		// 6. Protéger le dernier administrateur
		if role == "admin" {
			var adminCount int

			err = tx.QueryRowContext(r.Context(), `
                SELECT COUNT(*)
                FROM users
                WHERE role = 'admin'
            `).Scan(&adminCount)

			if err != nil {
				http.Error(w, "Erreur serveur", 500)
				return
			}

			if adminCount <= 1 {
				http.Error(w, "Impossible de supprimer le dernier admin", 409)
				return
			}
		}

		// 7. Ne pas supprimer un client avec des commandes
		var orderCount int

		err = tx.QueryRowContext(r.Context(), `
            SELECT COUNT(*)
            FROM orders
            WHERE user_id = ?
        `, id).Scan(&orderCount)

		if err != nil {
			http.Error(w, "Erreur verification commandes", 500)
			return
		}

		if orderCount > 0 {
			http.Error(w, "Utilisateur avec commandes : suppression refusee", 409)
			return
		}

		// 8. Supprimer l'utilisateur
		result, err := tx.ExecContext(r.Context(), `
            DELETE FROM users
            WHERE id = ?
        `, id)

		if err != nil {
			http.Error(w, "Erreur suppression utilisateur", 500)
			return
		}

		affected, err := result.RowsAffected()

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		if affected == 0 {
			http.Error(w, "Utilisateur introuvable", 404)
			return
		}

		// 9. Valider les modifications
		if err := tx.Commit(); err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		// 10. Réponse HTTP 204 : aucun contenu
		w.WriteHeader(http.StatusNoContent)
	})
}
