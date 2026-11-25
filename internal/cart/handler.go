package cart

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"ecommerce-go/internal/auth"
	"ecommerce-go/internal/products"
)

func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("GET /api/cart", getCartHandler(db))
	mux.HandleFunc("POST /api/cart/items", addItemHandler(db))
	mux.HandleFunc("PATCH /api/cart/items/{productID}", updateItemHandler(db))
	mux.HandleFunc("DELETE /api/cart/items/{productID}", removeItemHandler(db))
}

func getCartHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.ValidateSession(w, r, db)
		if !ok {
			return
		}

		cart, err := GetCartByUserID(db, user.ID)
		if errors.Is(err, ErrCartNotFound) {
			cart = Cart{UserID: user.ID, Items: []CartItem{}}
		} else if err != nil {
			writeCartError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cart)
	}
}

func addItemHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.ValidateSession(w, r, db)
		if !ok {
			return
		}

		var input struct {
			ProductID int64 `json:"product_id"`
			Quantity  int64 `json:"quantity"`
		}
		if !decodeCartJSON(w, r, &input) {
			return
		}
		if input.ProductID <= 0 {
			http.Error(w, "Identifiant produit invalide", http.StatusBadRequest)
			return
		}
		if input.Quantity <= 0 {
			writeCartError(w, ErrInvalidQuantity)
			return
		}

		if _, err := products.GetProduct(db, input.ProductID); err != nil {
			writeCartError(w, err)
			return
		}
		cart, err := CreateCart(db, user.ID)
		if err != nil {
			writeCartError(w, err)
			return
		}
		if err := AddItem(db, user.ID, cart.ID, input.ProductID, input.Quantity); err != nil {
			writeCartError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func updateItemHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.ValidateSession(w, r, db)
		if !ok {
			return
		}
		productID, ok := readProductID(w, r)
		if !ok {
			return
		}

		var input struct {
			Quantity int64 `json:"quantity"`
		}
		if !decodeCartJSON(w, r, &input) {
			return
		}
		if input.Quantity <= 0 {
			writeCartError(w, ErrInvalidQuantity)
			return
		}

		cart, err := GetCartByUserID(db, user.ID)
		if err == nil {
			err = UpdateItemQuantity(db, user.ID, cart.ID, productID, input.Quantity)
		}
		if err != nil {
			writeCartError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func removeItemHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.ValidateSession(w, r, db)
		if !ok {
			return
		}
		productID, ok := readProductID(w, r)
		if !ok {
			return
		}

		cart, err := GetCartByUserID(db, user.ID)
		if err == nil {
			err = RemoveItem(db, user.ID, cart.ID, productID)
		}
		if err != nil {
			writeCartError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func readProductID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("productID"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "Identifiant produit invalide", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func decodeCartJSON(w http.ResponseWriter, r *http.Request, input any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil || decoder.Decode(new(json.RawMessage)) != io.EOF {
		http.Error(w, "JSON invalide", http.StatusBadRequest)
		return false
	}
	return true
}

func writeCartError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidQuantity):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, ErrCartNotFound), errors.Is(err, ErrItemNotFound), errors.Is(err, products.ErrProductNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	default:
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
	}
}
