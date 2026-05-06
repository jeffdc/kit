package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"forage/internal/model"
)

const maxPricePageBytes = 2 << 20

var (
	reScriptBlock      = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>|<noscript[^>]*>.*?</noscript>|<svg[^>]*>.*?</svg>`)
	reTags             = regexp.MustCompile(`(?is)<[^>]+>`)
	reWhitespace       = regexp.MustCompile(`\s+`)
	reMetaPrice        = regexp.MustCompile(`(?is)itemprop\s*=\s*["']price["'][^>]*content\s*=\s*["']([^"']+)["']`)
	reJSONPrice        = regexp.MustCompile(`(?i)"price"\s*:\s*"?(?:[A-Z]{3}\s*)?[$£€]?([0-9]+(?:\.[0-9]{2})?)"?`)
	reCurrencyCode     = regexp.MustCompile(`(?i)"priceCurrency"\s*:\s*"([A-Z]{3})"`)
	reCurrencyPatterns = []currencyPattern{
		{Currency: "USD", Prefix: "$", Regex: regexp.MustCompile(`(?:US\$|\$)\s*([0-9]{1,4}(?:,[0-9]{3})*(?:\.[0-9]{2})?)`)},
		{Currency: "GBP", Prefix: "£", Regex: regexp.MustCompile(`£\s*([0-9]{1,4}(?:,[0-9]{3})*(?:\.[0-9]{2})?)`)},
		{Currency: "EUR", Prefix: "€", Regex: regexp.MustCompile(`€\s*([0-9]{1,4}(?:,[0-9]{3})*(?:\.[0-9]{2})?)`)},
		{Currency: "USD", Prefix: "USD ", Regex: regexp.MustCompile(`USD\s*([0-9]{1,4}(?:,[0-9]{3})*(?:\.[0-9]{2})?)`)},
	}
)

type currencyPattern struct {
	Currency string
	Prefix   string
	Regex    *regexp.Regexp
}

type PriceFetcher interface {
	FetchPrices(ctx context.Context, book model.Book, sellers []model.Bookseller) []model.PriceQuote
}

type priceFetcher struct {
	client *http.Client
}

func NewPriceFetcher(client *http.Client) PriceFetcher {
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	return &priceFetcher{client: client}
}

func (f *priceFetcher) FetchPrices(ctx context.Context, book model.Book, sellers []model.Bookseller) []model.PriceQuote {
	quotes := make([]model.PriceQuote, len(sellers))
	var wg sync.WaitGroup

	for i, seller := range sellers {
		wg.Add(1)
		go func(i int, seller model.Bookseller) {
			defer wg.Done()
			quotes[i] = f.fetchSellerPrice(ctx, book, seller)
		}(i, seller)
	}

	wg.Wait()
	return quotes
}

func (f *priceFetcher) fetchSellerPrice(ctx context.Context, book model.Book, seller model.Bookseller) model.PriceQuote {
	searchURL := buildSellerSearchURL(seller, book)
	quote := model.PriceQuote{
		SellerID:   seller.ID,
		SellerName: seller.Name,
		SellerURL:  seller.URL,
		SearchURL:  searchURL,
		Status:     "unavailable",
		FetchedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	if searchURL == "" {
		quote.Message = "missing search URL"
		return quote
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		quote.Status = "error"
		quote.Message = "invalid search URL"
		return quote
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ForagePriceBot/1.0; +https://example.invalid/forage)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := f.client.Do(req)
	if err != nil {
		quote.Status = "error"
		quote.Message = "request failed"
		return quote
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		quote.Status = "error"
		quote.Message = "HTTP " + strconv.Itoa(resp.StatusCode)
		return quote
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPricePageBytes))
	if err != nil {
		quote.Status = "error"
		quote.Message = "failed to read response"
		return quote
	}

	display, amount, currency, ok := extractBestPrice(body)
	if !ok {
		return quote
	}

	quote.Status = "ok"
	quote.Price = display
	quote.Amount = amount
	quote.Currency = currency
	quote.Message = ""
	return quote
}

type priceRequest struct {
	ID string `json:"id"`
}

func (h *handler) handlePrices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if r.Header.Get("Authorization") != "Bearer "+h.apiKey {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req priceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}

	book, err := h.store.GetBook(req.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	sellers, err := h.store.LoadBooksellers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	quotes := h.priceFetcher.FetchPrices(r.Context(), *book, sellers)
	updated, err := h.store.UpdateBookPriceQuotes(req.ID, quotes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.mu.Lock()
	h.version = time.Now()
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func buildSellerSearchURL(seller model.Bookseller, book model.Book) string {
	query := strings.TrimSpace(strings.Join([]string{book.Title, book.Author}, " "))
	if book.ISBN != "" && strings.Contains(strings.ToLower(seller.URL), "isbn") {
		query = book.ISBN
	}
	if query == "" {
		return seller.URL
	}
	if strings.Contains(seller.URL, "{query}") {
		return strings.ReplaceAll(seller.URL, "{query}", url.QueryEscape(query))
	}
	return seller.URL
}

func extractBestPrice(body []byte) (string, float64, string, bool) {
	page := string(body)

	if match := reMetaPrice.FindStringSubmatch(page); len(match) == 2 {
		if display, amount, currency, ok := normalizePrice("", match[1]); ok {
			return display, amount, currency, true
		}
	}

	currency := ""
	if match := reCurrencyCode.FindStringSubmatch(page); len(match) == 2 {
		currency = strings.ToUpper(match[1])
	}
	if match := reJSONPrice.FindStringSubmatch(page); len(match) == 2 {
		if display, amount, normalizedCurrency, ok := normalizePrice(currency, match[1]); ok {
			return display, amount, normalizedCurrency, true
		}
	}

	text := pageText(page)
	candidates := collectPriceCandidates(text)
	if len(candidates) == 0 {
		return "", 0, "", false
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Amount == candidates[j].Amount {
			return i < j
		}
		return candidates[i].Amount < candidates[j].Amount
	})

	best := candidates[0]
	return best.Display, best.Amount, best.Currency, true
}

type priceCandidate struct {
	Display  string
	Amount   float64
	Currency string
}

func collectPriceCandidates(text string) []priceCandidate {
	var candidates []priceCandidate
	for _, pattern := range reCurrencyPatterns {
		matches := pattern.Regex.FindAllStringSubmatch(text, 12)
		for _, match := range matches {
			if len(match) != 2 {
				continue
			}
			display, amount, currency, ok := normalizePrice(pattern.Currency, pattern.Prefix+match[1])
			if !ok {
				continue
			}
			candidates = append(candidates, priceCandidate{
				Display:  display,
				Amount:   amount,
				Currency: currency,
			})
		}
	}
	return dedupeCandidates(candidates)
}

func dedupeCandidates(candidates []priceCandidate) []priceCandidate {
	seen := make(map[string]bool, len(candidates))
	out := make([]priceCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		key := fmt.Sprintf("%s|%.2f|%s", candidate.Display, candidate.Amount, candidate.Currency)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, candidate)
	}
	return out
}

func pageText(page string) string {
	page = reScriptBlock.ReplaceAllString(page, " ")
	page = reTags.ReplaceAllString(page, " ")
	page = html.UnescapeString(page)
	page = reWhitespace.ReplaceAllString(page, " ")
	return strings.TrimSpace(page)
}

func normalizePrice(currencyHint, raw string) (string, float64, string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0, "", false
	}

	currency := currencyHint
	switch {
	case strings.Contains(raw, "$"):
		currency = "USD"
	case strings.Contains(raw, "£"):
		currency = "GBP"
	case strings.Contains(raw, "€"):
		currency = "EUR"
	case strings.HasPrefix(strings.ToUpper(raw), "USD"):
		currency = "USD"
	}

	value := strings.ToUpper(raw)
	value = strings.TrimPrefix(value, "USD")
	value = strings.ReplaceAll(value, "$", "")
	value = strings.ReplaceAll(value, "£", "")
	value = strings.ReplaceAll(value, "€", "")
	value = strings.ReplaceAll(value, ",", "")
	value = strings.TrimSpace(value)

	amount, err := strconv.ParseFloat(value, 64)
	if err != nil || amount < 1 || amount > 1000 {
		return "", 0, "", false
	}

	switch currency {
	case "GBP":
		return "£" + fmt.Sprintf("%.2f", amount), amount, currency, true
	case "EUR":
		return "€" + fmt.Sprintf("%.2f", amount), amount, currency, true
	default:
		return "$" + fmt.Sprintf("%.2f", amount), amount, "USD", true
	}
}
