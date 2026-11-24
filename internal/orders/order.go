package orders

import (
	"math"
	"math/rand/v2"
)

const (
	StatusPending   = "pending"
	StatusPaid      = "paid"
	StatusShipping  = "shipping"
	StatusDelivered = "delivered"
	StatusCancelled = "cancelled"
)

type Item struct {
	ProductID    int64   `json:"product_id"`
	ProductName  string  `json:"product_name"`
	UnitPriceTTC float64 `json:"unit_price_ttc"`
	Quantity     int     `json:"quantity"`
}

type Order struct {
	Reference    string  `json:"reference"`
	UserID       int64   `json:"user_id"`
	Status       string  `json:"status"`
	CancelReason string  `json:"cancel_reason,omitempty"`
	CreatedAt    string  `json:"created_at"`
	Items        []Item  `json:"items"`
	TotalTTC     float64 `json:"total_ttc"`
}

func newReference() string {
	const characters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	reference := "CMD-"
	for range 6 {
		reference += string(characters[rand.IntN(len(characters))])
	}

	return reference
}

func totalTTC(items []Item) float64 {
	total := 0.0
	for _, item := range items {
		total += item.UnitPriceTTC * float64(item.Quantity)
	}

	return math.Round(total*100) / 100
}
