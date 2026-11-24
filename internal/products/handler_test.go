package products

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
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
		{"création indisponible", http.MethodPost, "", http.StatusMethodNotAllowed, nil},
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
