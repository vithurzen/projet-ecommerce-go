package cart

import "ecommerce-go/internal/products"

type Cart struct {
	ID            int64
	UserID        int64
	Reference     string
	Items         []CartItem
	TotalTTCCents int64
}

type CartItem struct {
	Product  products.Product
	Quantity int64
}
