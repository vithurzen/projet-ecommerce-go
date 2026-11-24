package products

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"ecommerce-go/internal/auth"
)

func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("GET /api/products", listProductsHandler(db))
	mux.HandleFunc("POST /api/products", createProductHandler(db))
}

func createProductHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.ValidateSession(w, r, db)
		if !ok {
			return
		}
		if user.Role != "admin" {
			http.Error(w, "Accès réservé aux administrateurs", http.StatusForbidden)
			return
		}

		var input struct {
			Name          string `json:"name"`
			Description   string `json:"description"`
			Category      string `json:"category"`
			PriceHTCents  *int64 `json:"price_ht_cents"`
			PriceTTCCents *int64 `json:"price_ttc_cents"`
		}

		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || decoder.Decode(new(json.RawMessage)) != io.EOF {
			http.Error(w, "JSON invalide", http.StatusBadRequest)
			return
		}
		if input.PriceHTCents == nil || input.PriceTTCCents == nil {
			http.Error(w, "Les prix HT et TTC sont obligatoires", http.StatusBadRequest)
			return
		}

		product, err := CreateProduct(db, Product{
			Name:          input.Name,
			Description:   input.Description,
			Category:      input.Category,
			PriceHTCents:  *input.PriceHTCents,
			PriceTTCCents: *input.PriceTTCCents,
		})
		if errors.Is(err, errRequiredProductFields) || errors.Is(err, errNegativeProductPrice) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, "Erreur serveur", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(product)
	}
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
