package orders

import (
	"database/sql"
	"errors"
)

var ErrOrderNotFound = errors.New("commande introuvable")
var ErrNoItems = errors.New("une commande doit contenir au moins un produit")

func CreateOrder(db *sql.DB, userID int64, items []Item) (Order, error) {
	if len(items) == 0 {
		return Order{}, ErrNoItems
	}

	tx, err := db.Begin()
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()

	reference := newReference()

	result, err := tx.Exec(
		"INSERT INTO orders (reference, user_id) VALUES (?, ?)",
		reference, userID,
	)
	if err != nil {
		return Order{}, err
	}

	orderID, err := result.LastInsertId()
	if err != nil {
		return Order{}, err
	}

	for _, item := range items {
		_, err := tx.Exec(`
            INSERT INTO order_items (order_id, product_id, product_name, unit_price_ttc, quantity)
            VALUES (?, ?, ?, ?, ?)
        `, orderID, item.ProductID, item.ProductName, item.UnitPriceTTC, item.Quantity)
		if err != nil {
			return Order{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return Order{}, err
	}

	return GetOrder(db, userID, reference)
}

func ListOrders(db *sql.DB, userID int64) ([]Order, error) {
	rows, err := db.Query(`
        SELECT reference
        FROM orders
        WHERE user_id = ?
        ORDER BY created_at DESC, id DESC
    `, userID)
	if err != nil {
		return nil, err
	}

	var references []string
	for rows.Next() {
		var reference string
		if err := rows.Scan(&reference); err != nil {
			rows.Close()
			return nil, err
		}
		references = append(references, reference)
	}
	rows.Close()

	if err := rows.Err(); err != nil {
		return nil, err
	}

	orders := []Order{}
	for _, reference := range references {
		order, err := GetOrder(db, userID, reference)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	return orders, nil
}

func GetOrder(db *sql.DB, userID int64, reference string) (Order, error) {
	var orderID int64
	order := Order{Reference: reference, UserID: userID}

	err := db.QueryRow(`
        SELECT id, status, cancel_reason, created_at
        FROM orders
        WHERE user_id = ? AND reference = ?
    `, userID, reference).Scan(&orderID, &order.Status, &order.CancelReason, &order.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrOrderNotFound
	}
	if err != nil {
		return Order{}, err
	}

	order.Items, err = getItems(db, orderID)
	if err != nil {
		return Order{}, err
	}

	order.TotalTTC = totalTTC(order.Items)

	return order, nil
}

func getItems(db *sql.DB, orderID int64) ([]Item, error) {
	rows, err := db.Query(`
        SELECT product_id, product_name, unit_price_ttc, quantity
        FROM order_items
        WHERE order_id = ?
    `, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		var item Item
		err := rows.Scan(&item.ProductID, &item.ProductName, &item.UnitPriceTTC, &item.Quantity)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}
