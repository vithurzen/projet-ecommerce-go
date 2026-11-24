package products

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"

	"ecommerce-go/internal/auth"
	"ecommerce-go/internal/database"
)

func TestProductsRoutes(t *testing.T) {
	db := openTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, db)

	inputs := []Product{
		{
			Name: "Clavier USB", Description: "Touches silencieuses", Category: "Informatique",
			PriceHTCents: 2500, PriceTTCCents: 3000,
		},
		{Name: "Guide Go", Description: "Initiation au langage Go", Category: "Livres"},
	}
	var catalog []Product
	for _, input := range inputs {
		product, err := CreateProduct(db, input)
		if err != nil {
			t.Fatal(err)
		}
		catalog = append(catalog, product)
	}

	tests := []struct {
		name   string
		method string
		query  string
		status int
		want   []Product
	}{
		{"tous les produits", http.MethodGet, "", http.StatusOK, catalog},
		{"nom", http.MethodGet, "?name=lavier+USB", http.StatusOK, catalog[:1]},
		{"description", http.MethodGet, "?description=silencieu", http.StatusOK, catalog[:1]},
		{"catégorie", http.MethodGet, "?category=Livres", http.StatusOK, catalog[1:]},
		{"prix HT", http.MethodGet, "?price_ht_cents=2500", http.StatusOK, catalog[:1]},
		{"prix TTC à zéro", http.MethodGet, "?price_ttc_cents=0", http.StatusOK, catalog[1:]},
		{"filtres combinés", http.MethodGet, "?name=Clavier&category=Info&price_ttc_cents=3000", http.StatusOK, catalog[:1]},
		{"aucun résultat", http.MethodGet, "?name=Clavier&category=Livres", http.StatusOK, []Product{}},
		{"prix HT non entier", http.MethodGet, "?price_ht_cents=25.50", http.StatusBadRequest, nil},
		{"prix TTC négatif", http.MethodGet, "?price_ttc_cents=-1", http.StatusBadRequest, nil},
		{"prix vide", http.MethodGet, "?price_ht_cents=", http.StatusBadRequest, nil},
		{"paramètre mal encodé", http.MethodGet, "?name=%zz", http.StatusBadRequest, nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, "/api/products"+test.query, nil)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("statut = %d, attendu %d, corps = %s", response.Code, test.status, response.Body)
			}
			if test.status != http.StatusOK {
				return
			}
			if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
				t.Fatalf("Content-Type = %q, attendu application/json", contentType)
			}

			var got []Product
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got == nil || !slices.Equal(got, test.want) {
				t.Fatalf("produits = %#v, attendu %#v", got, test.want)
			}
		})
	}
}

func TestCreateProductRoute(t *testing.T) {
	db := openTestDB(t)
	if err := database.InitSchema(db); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	auth.RegisterRoutes(mux, db)
	auth.RegisterLoginRoutes(mux, db)
	RegisterRoutes(mux, db)

	adminToken := loginForProductTest(t, mux, db, "admin")
	clientToken := loginForProductTest(t, mux, db, "client")
	const validProduct = `{"name":"Clavier","description":"Clavier USB","category":"Informatique","price_ht_cents":0,"price_ttc_cents":0}`

	tests := []struct {
		name   string
		token  string
		body   string
		status int
	}{
		{"sans session", "", validProduct, http.StatusUnauthorized},
		{"client", clientToken, validProduct, http.StatusForbidden},
		{"JSON mal formé", adminToken, `{"name":`, http.StatusBadRequest},
		{"plusieurs objets", adminToken, validProduct + `{}`, http.StatusBadRequest},
		{"prix manquant", adminToken, `{"name":"Clavier","description":"Clavier USB","category":"Informatique","price_ht_cents":0}`, http.StatusBadRequest},
		{"nom vide", adminToken, strings.Replace(validProduct, "Clavier", " ", 1), http.StatusBadRequest},
		{"prix négatif", adminToken, strings.Replace(validProduct, `"price_ht_cents":0`, `"price_ht_cents":-1`, 1), http.StatusBadRequest},
		{"administrateur", adminToken, validProduct, http.StatusCreated},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := postProductForTest(mux, test.token, test.body)
			if response.Code != test.status {
				t.Fatalf("statut = %d, attendu %d, corps = %s", response.Code, test.status, response.Body)
			}
			if test.status != http.StatusCreated {
				return
			}
			if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
				t.Fatalf("Content-Type = %q, attendu application/json", contentType)
			}
			var product Product
			if err := json.Unmarshal(response.Body.Bytes(), &product); err != nil {
				t.Fatal(err)
			}
			if product.ID <= 0 || !regexp.MustCompile(`^PDT-[A-Z0-9]{6}$`).MatchString(product.Reference) {
				t.Fatalf("identifiants générés invalides : %+v", product)
			}
			want := Product{
				ID: product.ID, Reference: product.Reference, Name: "Clavier",
				Description: "Clavier USB", Category: "Informatique",
			}
			stored, err := GetProduct(db, product.ID)
			if err != nil || product != want || stored != want {
				t.Fatalf("produit renvoyé = %+v, enregistré = %+v, attendu = %+v, erreur = %v", product, stored, want, err)
			}
		})
	}

	products, err := SearchProducts(db, SearchFilters{})
	if err != nil || len(products) != 1 {
		t.Fatalf("un seul produit devait être enregistré, résultat = %+v, erreur = %v", products, err)
	}

	if _, err := db.Exec("DROP TABLE products"); err != nil {
		t.Fatal(err)
	}
	response := postProductForTest(mux, adminToken, validProduct)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("statut = %d, attendu 500, corps = %s", response.Code, response.Body)
	}
}

func postProductForTest(mux *http.ServeMux, token, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/products", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	return response
}

func loginForProductTest(t *testing.T, mux *http.ServeMux, db *sql.DB, role string) string {
	t.Helper()
	email := role + "@example.com"
	body := `{"email":"` + email + `","password":"test-password"}`
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body)))
	if response.Code != http.StatusCreated {
		t.Fatalf("inscription : %d, %s", response.Code, response.Body)
	}
	if _, err := db.Exec("UPDATE users SET confirmed = 1, role = ? WHERE email = ?", role, email); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body)))
	var session struct {
		Token string `json:"token"`
	}
	if response.Code != http.StatusOK {
		t.Fatalf("connexion : %d, %s", response.Code, response.Body)
	}
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil || session.Token == "" {
		t.Fatalf("jeton de connexion absent ou invalide : %v", err)
	}
	return session.Token
}
