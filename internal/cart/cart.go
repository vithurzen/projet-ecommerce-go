package cart

import "ecommerce-go/internal/products"

type Cart struct {
	ID            int64      `json:"id,omitempty"`
	UserID        int64      `json:"user_id"`
	Reference     string     `json:"reference,omitempty"`
	Items         []CartItem `json:"items"`
	TotalTTCCents int64      `json:"total_ttc_cents"`
}

type CartItem struct {
	Product  products.Product `json:"product"`
	Quantity int64            `json:"quantity"`
}
