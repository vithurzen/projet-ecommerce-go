package products

import (
	"database/sql"
	"errors"
	"regexp"
	"slices"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)

	if err := InitSchema(db); err != nil {
		t.Fatal(err)
	}

	return db
}

func TestCreateAndGetProduct(t *testing.T) {
	db := openTestDB(t)
	input := Product{
		Name: "Clavier", Description: "Clavier USB", Category: "Informatique",
		PriceHTCents: 2500, PriceTTCCents: 3000,
	}

	created, err := CreateProduct(db, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID <= 0 || !regexp.MustCompile(`^PDT-[A-Z0-9]{6}$`).MatchString(created.Reference) {
		t.Fatalf("identifiants générés invalides : %+v", created)
	}

	want := input
	want.ID = created.ID
	want.Reference = created.Reference
	if created != want {
		t.Fatalf("produit créé = %+v, attendu %+v", created, want)
	}

	got, err := GetProduct(db, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("produit relu = %+v, attendu %+v", got, want)
	}

	second, err := CreateProduct(db, input)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == created.ID || second.Reference == created.Reference {
		t.Fatalf("identifiants réutilisés : %+v et %+v", created, second)
	}
}

func TestCreateProductRejectsInvalidInput(t *testing.T) {
	db := openTestDB(t)
	tests := []struct {
		name    string
		product Product
	}{
		{"nom vide", Product{Name: " ", Description: "Clavier USB", Category: "Informatique"}},
		{"description vide", Product{Name: "Clavier", Category: "Informatique"}},
		{"catégorie vide", Product{Name: "Clavier", Description: "Clavier USB"}},
		{"HT négatif", Product{Name: "Clavier", Description: "Clavier USB", Category: "Informatique", PriceHTCents: -1}},
		{"TTC négatif", Product{Name: "Clavier", Description: "Clavier USB", Category: "Informatique", PriceTTCCents: -1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := CreateProduct(db, test.product); err == nil {
				t.Fatal("le produit invalide a été accepté")
			}
		})
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM products").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("%d produit(s) invalide(s) enregistré(s)", count)
	}
}

func TestGetProductNotFound(t *testing.T) {
	db := openTestDB(t)

	if _, err := GetProduct(db, 1); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("erreur = %v, attendu %v", err, ErrProductNotFound)
	}
}

func TestSearchProducts(t *testing.T) {
	db := openTestDB(t)
	inputs := []Product{
		{Name: "Clavier USB", Description: "Touches silencieuses", Category: "Informatique", PriceHTCents: 2500, PriceTTCCents: 3000},
		{Name: "Clavier Bluetooth", Description: "Touches rétroéclairées", Category: "Informatique", PriceHTCents: 3000, PriceTTCCents: 3600},
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

	var price int64 = 3000
	var zero int64
	tests := []struct {
		name    string
		filters SearchFilters
		want    []Product
	}{
		{"nom", SearchFilters{Name: "lavier"}, catalog[:2]},
		{"description", SearchFilters{Description: "silencieu"}, catalog[:1]},
		{"catégorie", SearchFilters{Category: "ivre"}, catalog[2:]},
		{"prix HT", SearchFilters{PriceHTCents: &price}, catalog[1:2]},
		{"prix TTC", SearchFilters{PriceTTCCents: &price}, catalog[:1]},
		{"prix HT à zéro", SearchFilters{PriceHTCents: &zero}, catalog[2:]},
		{"prix TTC à zéro", SearchFilters{PriceTTCCents: &zero}, catalog[2:]},
		{"plusieurs critères", SearchFilters{Name: "Clavier", Category: "Info", PriceTTCCents: &price}, catalog[:1]},
		{"critères incompatibles", SearchFilters{Name: "Clavier", Category: "Livres"}, nil},
		{"aucun résultat", SearchFilters{Name: "Souris"}, nil},
		{"aucun filtre", SearchFilters{}, catalog},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := SearchProducts(db, test.filters)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || !slices.Equal(got, test.want) {
				t.Fatalf("résultats = %#v, attendu %#v", got, test.want)
			}
		})
	}
}
