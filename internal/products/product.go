package products

import "math/rand/v2"

type Product struct {
	ID            int64
	Reference     string
	Name          string
	Description   string
	Category      string
	PriceHTCents  int64
	PriceTTCCents int64
}

func newReference() string {
	const characters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	reference := "PDT-"
	for range 6 {
		reference += string(characters[rand.IntN(len(characters))])
	}

	return reference
}
