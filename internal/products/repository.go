package products

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrProductNotFound = errors.New("produit introuvable")

var (
	errRequiredProductFields = errors.New("le nom, la description et la catégorie sont obligatoires")
	errNegativeProductPrice  = errors.New("les prix doivent être positifs ou nuls")
)

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
		return Product{}, errRequiredProductFields
	}
	if product.PriceHTCents < 0 || product.PriceTTCCents < 0 {
		return Product{}, errNegativeProductPrice
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

type SearchFilters struct {
	Name        string
	Description string
	Category    string

	// nil désactive le filtre de prix. Un pointeur vers zéro recherche un prix de zéro.
	PriceHTCents  *int64
	PriceTTCCents *int64
}

func SearchProducts(db *sql.DB, filters SearchFilters) ([]Product, error) {
	query := `
		SELECT id, reference, name, description, category, price_ht_cents, price_ttc_cents
		FROM products
		WHERE 1 = 1
	`
	var args []any

	if name := strings.TrimSpace(filters.Name); name != "" {
		query += " AND name LIKE ?"
		args = append(args, "%"+name+"%")
	}
	if description := strings.TrimSpace(filters.Description); description != "" {
		query += " AND description LIKE ?"
		args = append(args, "%"+description+"%")
	}
	if category := strings.TrimSpace(filters.Category); category != "" {
		query += " AND category LIKE ?"
		args = append(args, "%"+category+"%")
	}
	if filters.PriceHTCents != nil {
		query += " AND price_ht_cents = ?"
		args = append(args, *filters.PriceHTCents)
	}
	if filters.PriceTTCCents != nil {
		query += " AND price_ttc_cents = ?"
		args = append(args, *filters.PriceTTCCents)
	}

	rows, err := db.Query(query+" ORDER BY id", args...)
	if err != nil {
		return nil, fmt.Errorf("recherche des produits : %w", err)
	}
	defer rows.Close()

	products := []Product{}
	for rows.Next() {
		var product Product
		err := rows.Scan(&product.ID, &product.Reference, &product.Name, &product.Description,
			&product.Category, &product.PriceHTCents, &product.PriceTTCCents)
		if err != nil {
			return nil, fmt.Errorf("lecture d'un résultat de recherche : %w", err)
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lecture des résultats de recherche : %w", err)
	}

	return products, nil
}
