package cart

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"ecommerce-go/internal/auth"
	"ecommerce-go/internal/database"
	"ecommerce-go/internal/products"
)

func TestCartRoutes(t *testing.T) {
	db := openTestDB(t, "file::memory:?_pragma=foreign_keys(1)")
	if err := database.InitSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := products.InitSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := InitSchema(db); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	auth.RegisterRoutes(mux, db)
	auth.RegisterLoginRoutes(mux, db)
	RegisterRoutes(mux, db)

	firstToken := loginForCartTest(t, mux, db, "premier@example.com")
	secondToken := loginForCartTest(t, mux, db, "second@example.com")
	product, err := products.CreateProduct(db, products.Product{
		Name: "Clavier", Description: "Clavier USB", Category: "Informatique",
		PriceHTCents: 2500, PriceTTCCents: 3000,
	})
	if err != nil {
		t.Fatal(err)
	}
	const cartPath = "/api/cart"
	const itemsPath = "/api/cart/items"
	itemPath := fmt.Sprintf("%s/%d", itemsPath, product.ID)
	addTwo := fmt.Sprintf(`{"product_id":%d,"quantity":2}`, product.ID)
	const updateQuantity = `{"quantity":4}`

	errorsBeforeAdd := []struct {
		name, method, path, token, body string
		status                          int
	}{
		{"consultation sans session", http.MethodGet, cartPath, "", "", http.StatusUnauthorized},
		{"ajout sans session", http.MethodPost, itemsPath, "", addTwo, http.StatusUnauthorized},
		{"modification sans session", http.MethodPatch, itemPath, "", updateQuantity, http.StatusUnauthorized},
		{"suppression sans session", http.MethodDelete, itemPath, "", "", http.StatusUnauthorized},
		{"session inconnue", http.MethodGet, cartPath, strings.Repeat("0", 64), "", http.StatusUnauthorized},
		{"produit inconnu", http.MethodPost, itemsPath, firstToken, `{"product_id":999,"quantity":2}`, http.StatusNotFound},
		{"identifiant nul", http.MethodPost, itemsPath, firstToken, `{"product_id":0,"quantity":2}`, http.StatusBadRequest},
		{"identifiant non numérique", http.MethodPatch, itemsPath + "/abc", firstToken, updateQuantity, http.StatusBadRequest},
		{"identifiant négatif", http.MethodDelete, itemsPath + "/-1", firstToken, "", http.StatusBadRequest},
		{"quantité nulle", http.MethodPost, itemsPath, firstToken, `{"product_id":1,"quantity":0}`, http.StatusBadRequest},
		{"quantité négative", http.MethodPatch, itemPath, firstToken, `{"quantity":-1}`, http.StatusBadRequest},
		{"quantité décimale", http.MethodPatch, itemPath, firstToken, `{"quantity":1.5}`, http.StatusBadRequest},
		{"JSON incomplet", http.MethodPost, itemsPath, firstToken, `{"product_id":`, http.StatusBadRequest},
		{"identité dans le JSON", http.MethodPost, itemsPath, firstToken, `{"user_id":2,"product_id":1,"quantity":1}`, http.StatusBadRequest},
		{"modification sans panier", http.MethodPatch, itemPath, firstToken, updateQuantity, http.StatusNotFound},
		{"suppression sans panier", http.MethodDelete, itemPath, firstToken, "", http.StatusNotFound},
	}
	for _, test := range errorsBeforeAdd {
		t.Run(test.name, func(t *testing.T) {
			cartRequest(t, mux, test.method, test.path, test.token, test.body, test.status)
		})
	}

	empty := readCartForTest(t, mux, cartPath, firstToken, 0, []CartItem{})
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM carts").Scan(&count); err != nil || count != 0 || empty.ID != 0 {
		t.Fatalf("panier vide : panier = %+v, nombre en base = %d, erreur = %v", empty, count, err)
	}

	cartRequest(t, mux, http.MethodPost, itemsPath, firstToken, addTwo, http.StatusNoContent)
	first := readCartForTest(t, mux, cartPath, firstToken, 6000, []CartItem{{Product: product, Quantity: 2}})
	cartRequest(t, mux, http.MethodPost, itemsPath, firstToken, addTwo, http.StatusNoContent)
	reused := readCartForTest(t, mux, cartPath, firstToken, 12000, []CartItem{{Product: product, Quantity: 4}})
	if first.ID == 0 || reused.ID != first.ID {
		t.Fatalf("identifiants après deux ajouts : %d, %d", first.ID, reused.ID)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM carts").Scan(&count); err != nil || count != 1 {
		t.Fatalf("nombre de paniers après deux ajouts = %d, erreur = %v", count, err)
	}

	second := readCartForTest(t, mux, fmt.Sprintf("%s?user_id=%d", cartPath, first.UserID), secondToken, 0, []CartItem{})
	if second.UserID == first.UserID {
		t.Fatalf("le deuxième compte a reçu l'identifiant du premier : %d", first.UserID)
	}
	cartRequest(t, mux, http.MethodPatch, itemPath, secondToken, updateQuantity, http.StatusNotFound)
	cartRequest(t, mux, http.MethodDelete, itemPath, secondToken, "", http.StatusNotFound)
	cartRequest(t, mux, http.MethodPost, itemsPath, secondToken, addTwo, http.StatusNoContent)
	second = readCartForTest(t, mux, cartPath, secondToken, 6000, []CartItem{{Product: product, Quantity: 2}})
	if second.ID == first.ID {
		t.Fatalf("les deux comptes partagent le panier %d", first.ID)
	}
	readCartForTest(t, mux, cartPath, firstToken, 12000, []CartItem{{Product: product, Quantity: 4}})

	cartRequest(t, mux, http.MethodPatch, itemPath, firstToken, `{"quantity":3}`, http.StatusNoContent)
	readCartForTest(t, mux, cartPath, firstToken, 9000, []CartItem{{Product: product, Quantity: 3}})
	cartRequest(t, mux, http.MethodDelete, itemPath, firstToken, "", http.StatusNoContent)
	readCartForTest(t, mux, cartPath, firstToken, 0, []CartItem{})
	cartRequest(t, mux, http.MethodDelete, itemPath, firstToken, "", http.StatusNotFound)
	readCartForTest(t, mux, cartPath, secondToken, 6000, []CartItem{{Product: product, Quantity: 2}})
}

func cartRequest(t *testing.T, mux *http.ServeMux, method, path, token, body string, status int) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != status {
		t.Fatalf("%s %s : statut = %d, attendu %d, corps = %s", method, path, response.Code, status, response.Body)
	}
	if status == http.StatusNoContent && response.Body.Len() != 0 {
		t.Fatalf("la réponse 204 contient un corps : %s", response.Body)
	}
	return response
}

func readCartForTest(t *testing.T, mux *http.ServeMux, path, token string, total int64, items []CartItem) Cart {
	t.Helper()
	response := cartRequest(t, mux, http.MethodGet, path, token, "", http.StatusOK)
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, attendu application/json", contentType)
	}
	var cart Cart
	if err := json.Unmarshal(response.Body.Bytes(), &cart); err != nil {
		t.Fatal(err)
	}
	if cart.UserID <= 0 || cart.TotalTTCCents != total || cart.Items == nil || !slices.Equal(cart.Items, items) {
		t.Fatalf("panier = %+v, lignes attendues = %+v, total attendu = %d", cart, items, total)
	}
	return cart
}

func loginForCartTest(t *testing.T, mux *http.ServeMux, db *sql.DB, email string) string {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":"test-password"}`, email)
	cartRequest(t, mux, http.MethodPost, "/api/auth/register", "", body, http.StatusCreated)
	if _, err := db.Exec("UPDATE users SET confirmed = 1 WHERE email = ?", email); err != nil {
		t.Fatal(err)
	}
	response := cartRequest(t, mux, http.MethodPost, "/api/auth/login", "", body, http.StatusOK)
	var session struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil || session.Token == "" {
		t.Fatalf("jeton de connexion absent ou invalide : %v", err)
	}
	return session.Token
}
