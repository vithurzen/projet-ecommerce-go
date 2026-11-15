package database

import (
	"database/sql"
	"os"

	_ "modernc.org/sqlite"
)

func Open() (*sql.DB, error) {

	// Créer le dossier data
	err := os.MkdirAll("data", 0755)
	if err != nil {
		return nil, err
	}

	// Ouvrir la base SQLite
	db, err := sql.Open(
		"sqlite",
		"file:data/ecommerce.db?_pragma=foreign_keys(1)",
	)

	if err != nil {
		return nil, err
	}

	// Pour simplifier les premiers accès SQLite
	db.SetMaxOpenConns(1)

	// Vérifier la connexion
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
