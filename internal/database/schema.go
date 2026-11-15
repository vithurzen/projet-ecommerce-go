package database

import (
	"database/sql"
	"fmt"
)

func InitSchema(db *sql.DB) error {

	query := `
    CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,

        email TEXT NOT NULL COLLATE NOCASE UNIQUE,

        password_hash TEXT NOT NULL,

        role TEXT NOT NULL DEFAULT 'client'
            CHECK (role IN ('client', 'admin')),

        confirmed INTEGER NOT NULL DEFAULT 0
            CHECK (confirmed IN (0, 1)),

        created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    `

	_, err := db.Exec(query)

	if err != nil {
		return fmt.Errorf("creation table users : %w", err)
	}

	return nil
}
