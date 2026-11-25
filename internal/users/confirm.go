package users

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// Confirmer un utilisateur depuis l'administration
func confirmUser(db *sql.DB) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// 1. Récupérer l'ID depuis l'URL
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

		if err != nil || id <= 0 {
			http.Error(w, "Identifiant invalide", 400)
			return
		}

		// 2. Démarrer une transaction SQL
		tx, err := db.BeginTx(r.Context(), nil)

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}
		defer tx.Rollback()

		// 3. Vérifier que l'utilisateur existe
		var existingID int64

		err = tx.QueryRowContext(r.Context(), `
            SELECT id FROM users WHERE id = ?
        `, id).Scan(&existingID)

		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Utilisateur introuvable", 404)
			return
		}

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		// 4. Confirmer le compte
		_, err = tx.ExecContext(r.Context(), `
            UPDATE users
            SET confirmed = 1
            WHERE id = ?
        `, id)

		if err != nil {
			http.Error(w, "Erreur confirmation", 500)
			return
		}

		// 5. Supprimer un éventuel code de confirmation
		_, err = tx.ExecContext(r.Context(), `
            DELETE FROM confirmation_codes
            WHERE user_id = ?
        `, id)

		if err != nil {
			http.Error(w, "Erreur suppression du code", 500)
			return
		}

		// 6. Valider les modifications
		if err := tx.Commit(); err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		// 7. Retourner la réponse
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":        id,
			"confirmed": true,
			"message":   "Compte confirmé avec succès",
		})
	})
}
