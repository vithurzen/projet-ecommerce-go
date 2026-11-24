package cart

import (
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"ecommerce-go/internal/database"
	"ecommerce-go/internal/products"
)

func TestCartLifecycle(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "cart.db") + "?_pragma=foreign_keys(1)"
	db := openTestDB(t, dsn)
	if err := database.InitSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := products.InitSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := InitSchema(db); err != nil {
		t.Fatal(err)
	}

	account, err := db.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "client@example.com", "test")
	if err != nil {
		t.Fatal(err)
	}
	userID, err := account.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateCart(db, userID+1); err == nil {
		t.Fatal("création d'un panier pour un utilisateur inexistant")
	}
	if _, err := GetCartByUserID(db, userID); !errors.Is(err, ErrCartNotFound) {
		t.Fatalf("lecture avant la création du panier : %v", err)
	}
	created, err := CreateCart(db, userID)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID <= 0 || created.UserID != userID {
		t.Fatalf("panier créé invalide : %+v", created)
	}
	assertCart(t, db, userID, created.ID, 0, []CartItem{})

	keyboard, err := products.CreateProduct(db, products.Product{
		Name: "Clavier", Description: "Clavier USB", Category: "Informatique",
		PriceHTCents: 2500, PriceTTCCents: 3000,
	})
	if err != nil {
		t.Fatal(err)
	}
	mouse, err := products.CreateProduct(db, products.Product{
		Name: "Souris", Description: "Souris USB", Category: "Informatique",
		PriceHTCents: 1000, PriceTTCCents: 1200,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := AddItem(db, userID, created.ID, keyboard.ID, 1); err != nil {
		t.Fatal(err)
	}
	assertCart(t, db, userID, created.ID, 3000, []CartItem{{Product: keyboard, Quantity: 1}})
	if err := AddItem(db, userID, created.ID, keyboard.ID, 2); err != nil {
		t.Fatal(err)
	}
	assertCart(t, db, userID, created.ID, 9000, []CartItem{{Product: keyboard, Quantity: 3}})
	if err := AddItem(db, userID, created.ID, mouse.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := UpdateItemQuantity(db, userID, created.ID, keyboard.ID, 1); err != nil {
		t.Fatal(err)
	}
	wantItems := []CartItem{{Product: keyboard, Quantity: 1}, {Product: mouse, Quantity: 2}}
	assertCart(t, db, userID, created.ID, 5400, wantItems)

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db = openTestDB(t, dsn)
	assertCart(t, db, userID, created.ID, 5400, wantItems)

	got, err := GetCartByUserID(db, userID)
	if err != nil || got.ID != created.ID || got.UserID != userID ||
		got.TotalTTCCents != 5400 || !slices.Equal(got.Items, wantItems) {
		t.Fatalf("lecture par utilisateur : panier = %+v, erreur = %v", got, err)
	}
	for range 2 {
		got, err := CreateCart(db, userID)
		if err != nil || got.ID != created.ID || got.UserID != userID ||
			got.TotalTTCCents != 5400 || !slices.Equal(got.Items, wantItems) {
			t.Fatalf("réutilisation du panier : panier = %+v, erreur = %v", got, err)
		}
	}

	otherAccount, err := db.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "autre@example.com", "test")
	if err != nil {
		t.Fatal(err)
	}
	otherUserID, err := otherAccount.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	otherCart, err := CreateCart(db, otherUserID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := GetCart(db, otherUserID, created.ID); !errors.Is(err, ErrCartNotFound) {
		t.Fatalf("lecture par un autre utilisateur : %v", err)
	}
	if err := AddItem(db, otherUserID, created.ID, keyboard.ID, 1); !errors.Is(err, ErrCartNotFound) {
		t.Fatalf("ajout par un autre utilisateur : %v", err)
	}
	if err := UpdateItemQuantity(db, otherUserID, created.ID, keyboard.ID, 2); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("modification par un autre utilisateur : %v", err)
	}
	if err := RemoveItem(db, otherUserID, created.ID, keyboard.ID); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("suppression par un autre utilisateur : %v", err)
	}
	assertCart(t, db, userID, created.ID, 5400, wantItems)
	assertCart(t, db, otherUserID, otherCart.ID, 0, []CartItem{})

	if err := AddItem(db, userID, created.ID, mouse.ID+1, 1); err == nil {
		t.Fatal("ajout d'un produit inexistant")
	}
	if err := AddItem(db, userID, created.ID, keyboard.ID, -1); !errors.Is(err, ErrInvalidQuantity) {
		t.Fatalf("ajout avec une quantité négative : %v", err)
	}
	if err := UpdateItemQuantity(db, userID, created.ID, keyboard.ID, 0); !errors.Is(err, ErrInvalidQuantity) {
		t.Fatalf("modification avec une quantité nulle : %v", err)
	}
	if err := RemoveItem(db, userID, created.ID, keyboard.ID); err != nil {
		t.Fatal(err)
	}
	assertCart(t, db, userID, created.ID, 2400, wantItems[1:])
	if err := RemoveItem(db, userID, created.ID, mouse.ID); err != nil {
		t.Fatal(err)
	}
	assertCart(t, db, userID, created.ID, 0, []CartItem{})
}

func TestInitSchemaExistingCarts(t *testing.T) {
	for _, test := range []struct {
		name      string
		cartCount int
	}{
		{"un seul panier", 1},
		{"paniers en double", 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := openTestDB(t, "file::memory:?_pragma=foreign_keys(1)")
			if err := database.InitSchema(db); err != nil {
				t.Fatal(err)
			}
			if err := products.InitSchema(db); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`
				INSERT INTO users (id, email, password_hash) VALUES (1, 'client@example.com', 'test');
				CREATE TABLE carts (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
					reference TEXT UNIQUE
				);
			`); err != nil {
				t.Fatal(err)
			}
			for range test.cartCount {
				if _, err := db.Exec("INSERT INTO carts (user_id) VALUES (1)"); err != nil {
					t.Fatal(err)
				}
			}

			err := InitSchema(db)
			if test.cartCount == 1 {
				if err != nil {
					t.Fatal(err)
				}
				if err := InitSchema(db); err != nil {
					t.Fatalf("réinitialisation : %v", err)
				}
				cart, err := GetCartByUserID(db, 1)
				if err != nil || cart.ID != 1 {
					t.Fatalf("panier existant : panier = %+v, erreur = %v", cart, err)
				}
				if _, err := db.Exec("INSERT INTO carts (user_id) VALUES (1)"); err == nil {
					t.Fatal("SQLite a accepté un deuxième panier")
				}
			} else if err == nil {
				t.Fatal("l'installation de l'index a accepté des paniers en double")
			}

			var count int
			if err := db.QueryRow("SELECT COUNT(*) FROM carts WHERE user_id = 1").Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != test.cartCount {
				t.Fatalf("paniers conservés = %d, attendu %d", count, test.cartCount)
			}
		})
	}
}

func openTestDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	return db
}

func assertCart(t *testing.T, db *sql.DB, userID, cartID, total int64, items []CartItem) {
	t.Helper()
	got, err := GetCart(db, userID, cartID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != cartID || got.UserID != userID || got.TotalTTCCents != total ||
		got.Items == nil || !slices.Equal(got.Items, items) {
		t.Fatalf("panier = %+v, lignes attendues = %+v, total attendu = %d", got, items, total)
	}
}
