package cart

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
)

var (
	ErrCartNotFound    = errors.New("panier introuvable")
	ErrItemNotFound    = errors.New("ligne de panier introuvable")
	ErrInvalidQuantity = errors.New("la quantité doit être strictement positive")
)

func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS carts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			reference TEXT UNIQUE
		);

		CREATE TABLE IF NOT EXISTS cart_items (
			cart_id INTEGER NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
			product_id INTEGER NOT NULL REFERENCES products(id),
			quantity INTEGER NOT NULL CHECK (typeof(quantity) = 'integer' AND quantity > 0),
			PRIMARY KEY (cart_id, product_id)
		);
	`)
	if err != nil {
		return fmt.Errorf("initialisation des tables du panier : %w", err)
	}
	return nil
}

func CreateCart(db *sql.DB, userID int64) (Cart, error) {
	cart := Cart{UserID: userID, Items: []CartItem{}}
	err := db.QueryRow("INSERT INTO carts (user_id) VALUES (?) RETURNING id", userID).Scan(&cart.ID)
	if err != nil {
		return Cart{}, fmt.Errorf("création du panier : %w", err)
	}
	return cart, nil
}

func AddItem(db *sql.DB, userID, cartID, productID, quantity int64) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	result, err := db.Exec(`
		INSERT INTO cart_items (cart_id, product_id, quantity)
		SELECT id, ?, ? FROM carts WHERE id = ? AND user_id = ?
		ON CONFLICT(cart_id, product_id) DO UPDATE
		SET quantity = cart_items.quantity + excluded.quantity
	`, productID, quantity, cartID, userID)
	return checkWrite(result, err, ErrCartNotFound)
}

func UpdateItemQuantity(db *sql.DB, userID, cartID, productID, quantity int64) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	result, err := db.Exec(`
		UPDATE cart_items SET quantity = ?
		WHERE cart_id = (SELECT id FROM carts WHERE id = ? AND user_id = ?)
		  AND product_id = ?
	`, quantity, cartID, userID, productID)
	return checkWrite(result, err, ErrItemNotFound)
}

func RemoveItem(db *sql.DB, userID, cartID, productID int64) error {
	result, err := db.Exec(`
		DELETE FROM cart_items
		WHERE cart_id = (SELECT id FROM carts WHERE id = ? AND user_id = ?)
		  AND product_id = ?
	`, cartID, userID, productID)
	return checkWrite(result, err, ErrItemNotFound)
}

func checkWrite(result sql.Result, err error, notFound error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return notFound
	}
	return nil
}

func GetCart(db *sql.DB, userID, cartID int64) (Cart, error) {
	rows, err := db.Query(`
		SELECT COALESCE(c.reference, ''), COALESCE(p.id, 0),
		       COALESCE(p.reference, ''), COALESCE(p.name, ''),
		       COALESCE(p.description, ''), COALESCE(p.category, ''),
		       COALESCE(p.price_ht_cents, 0), COALESCE(p.price_ttc_cents, 0),
		       COALESCE(i.quantity, 0)
		FROM carts c
		LEFT JOIN cart_items i ON i.cart_id = c.id
		LEFT JOIN products p ON p.id = i.product_id
		WHERE c.id = ? AND c.user_id = ?
		ORDER BY i.product_id
	`, cartID, userID)
	if err != nil {
		return Cart{}, err
	}
	defer rows.Close()

	cart := Cart{ID: cartID, UserID: userID, Items: []CartItem{}}
	found := false
	for rows.Next() {
		found = true
		var item CartItem
		product := &item.Product
		err := rows.Scan(&cart.Reference, &product.ID, &product.Reference, &product.Name,
			&product.Description, &product.Category, &product.PriceHTCents, &product.PriceTTCCents,
			&item.Quantity)
		if err != nil {
			return Cart{}, err
		}
		// Le LEFT JOIN renvoie une ligne sans produit lorsque le panier est vide.
		if item.Quantity == 0 {
			continue
		}
		if product.PriceTTCCents > (math.MaxInt64-cart.TotalTTCCents)/item.Quantity {
			return Cart{}, errors.New("le total TTC du panier dépasse la limite des centimes")
		}
		cart.Items = append(cart.Items, item)
		cart.TotalTTCCents += product.PriceTTCCents * item.Quantity
	}
	if err := rows.Err(); err != nil {
		return Cart{}, err
	}
	if !found {
		return Cart{}, ErrCartNotFound
	}
	return cart, nil
}
