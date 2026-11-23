package products

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrProductNotFound = errors.New("produit introuvable")

func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			reference TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			category TEXT NOT NULL,
			price_ht_cents INTEGER NOT NULL CHECK (price_ht_cents >= 0),
			price_ttc_cents INTEGER NOT NULL CHECK (price_ttc_cents >= 0)
		);
	`)
	if err != nil {
		return fmt.Errorf("création de la table products : %w", err)
	}

	return nil
}

func CreateProduct(db *sql.DB, product Product) (Product, error) {
	product.Name = strings.TrimSpace(product.Name)
	product.Description = strings.TrimSpace(product.Description)
	product.Category = strings.TrimSpace(product.Category)

	if product.Name == "" || product.Description == "" || product.Category == "" {
		return Product{}, errors.New("le nom, la description et la catégorie sont obligatoires")
	}
	if product.PriceHTCents < 0 || product.PriceTTCCents < 0 {
		return Product{}, errors.New("les prix doivent être positifs ou nuls")
	}

	for range 5 {
		product.Reference = newReference()
		err := db.QueryRow(`
			INSERT INTO products (reference, name, description, category, price_ht_cents, price_ttc_cents)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(reference) DO NOTHING
			RETURNING id
		`, product.Reference, product.Name, product.Description, product.Category,
			product.PriceHTCents, product.PriceTTCCents).Scan(&product.ID)
		if errors.Is(err, sql.ErrNoRows) {
			// La référence existe déjà. On en génère une autre.
			continue
		}
		if err != nil {
			return Product{}, fmt.Errorf("enregistrement du produit : %w", err)
		}

		return product, nil
	}

	return Product{}, errors.New("impossible de générer une référence produit unique")
}

func GetProduct(db *sql.DB, id int64) (Product, error) {
	var product Product
	err := db.QueryRow(`
		SELECT id, reference, name, description, category, price_ht_cents, price_ttc_cents
		FROM products
		WHERE id = ?
	`, id).Scan(&product.ID, &product.Reference, &product.Name, &product.Description,
		&product.Category, &product.PriceHTCents, &product.PriceTTCCents)
	if errors.Is(err, sql.ErrNoRows) {
		return Product{}, ErrProductNotFound
	}
	if err != nil {
		return Product{}, fmt.Errorf("lecture du produit : %w", err)
	}

	return product, nil
}
