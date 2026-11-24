package products

import "math/rand/v2"

type Product struct {
	ID            int64  `json:"id"`
	Reference     string `json:"reference"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Category      string `json:"category"`
	PriceHTCents  int64  `json:"price_ht_cents"`
	PriceTTCCents int64  `json:"price_ttc_cents"`
}

func newReference() string {
	const characters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	reference := "PDT-"
	for range 6 {
		reference += string(characters[rand.IntN(len(characters))])
	}

	return reference
}
