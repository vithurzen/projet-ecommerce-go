package products

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("GET /api/products", listProductsHandler(db))
}

func listProductsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			http.Error(w, "Paramètres de recherche invalides", http.StatusBadRequest)
			return
		}

		priceHT, err := parsePriceFilter(query, "price_ht_cents")
		if err != nil {
			http.Error(w, "Prix HT invalide", http.StatusBadRequest)
			return
		}
		priceTTC, err := parsePriceFilter(query, "price_ttc_cents")
		if err != nil {
			http.Error(w, "Prix TTC invalide", http.StatusBadRequest)
			return
		}

		results, err := SearchProducts(db, SearchFilters{
			Name:          query.Get("name"),
			Description:   query.Get("description"),
			Category:      query.Get("category"),
			PriceHTCents:  priceHT,
			PriceTTCCents: priceTTC,
		})
		if err != nil {
			http.Error(w, "Erreur serveur", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

func parsePriceFilter(query url.Values, name string) (*int64, error) {
	if !query.Has(name) {
		return nil, nil
	}

	price, err := strconv.ParseInt(query.Get(name), 10, 64)
	if err != nil {
		return nil, err
	}
	if price < 0 {
		return nil, strconv.ErrRange
	}
	return &price, nil
}
