package api

import (
	"testing"

	"forage/internal/model"
)

func TestBuildSellerSearchURLUsesISBNWhenSellerURLLooksISBNBased(t *testing.T) {
	seller := model.Bookseller{
		Name: "Biblio",
		URL:  "https://www.biblio.com/search.php?keyisbn={query}",
	}
	book := model.Book{
		Title:  "Dune",
		Author: "Frank Herbert",
		ISBN:   "9780441172719",
	}

	got := buildSellerSearchURL(seller, book)
	want := "https://www.biblio.com/search.php?keyisbn=9780441172719"
	if got != want {
		t.Fatalf("buildSellerSearchURL() = %q, want %q", got, want)
	}
}

func TestExtractBestPrice(t *testing.T) {
	body := []byte(`
		<html>
			<body>
				<div class="result">Used from $12.99</div>
				<div class="result">New from $18.50</div>
			</body>
		</html>
	`)

	display, amount, currency, ok := extractBestPrice(body)
	if !ok {
		t.Fatal("expected a price match")
	}
	if display != "$12.99" {
		t.Fatalf("display = %q, want $12.99", display)
	}
	if amount != 12.99 {
		t.Fatalf("amount = %.2f, want 12.99", amount)
	}
	if currency != "USD" {
		t.Fatalf("currency = %q, want USD", currency)
	}
}
