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

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS confirmation_codes (
			user_id INTEGER PRIMARY KEY,
			code_hash TEXT NOT NULL UNIQUE,
			expires_at TEXT NOT NULL,

			FOREIGN KEY (user_id)
				REFERENCES users(id)
				ON DELETE CASCADE
		);
	`)

	if err != nil {
		return fmt.Errorf(
			"creation table confirmation_codes : %w",
			err,
		)
	}

	return nil
}
