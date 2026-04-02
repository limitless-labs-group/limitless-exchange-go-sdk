package limitless

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMarketFetcher_GetMarket_CachesVenue(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/markets/test-market", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"slug":  "test-market",
			"title": "Test Market",
			"venue": map[string]any{
				"exchange": "0xa4409D988CA2218d956BeEFD3874100F444f0DC3",
				"adapter":  nil,
			},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(WithBaseURL(srv.URL))
	fetcher := NewMarketFetcher(client)

	market, err := fetcher.GetMarket(context.Background(), "test-market")
	if err != nil {
		t.Fatalf("GetMarket returned error: %v", err)
	}
	if market.Slug != "test-market" {
		t.Fatalf("expected slug test-market, got %q", market.Slug)
	}

	venue, ok := fetcher.GetVenue("test-market")
	if !ok {
		t.Fatal("expected venue to be cached")
	}
	if venue.Exchange != "0xa4409D988CA2218d956BeEFD3874100F444f0DC3" {
		t.Fatalf("unexpected cached venue exchange: %s", venue.Exchange)
	}
}

func TestMarketFetcher_GetActiveMarkets_UsesParams(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/markets/active", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("limit") != "20" || q.Get("page") != "2" || q.Get("sortBy") != "newest" {
			t.Fatalf("unexpected query params: %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":              []any{},
			"totalMarketsCount": 0,
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(WithBaseURL(srv.URL))
	fetcher := NewMarketFetcher(client)

	_, err := fetcher.GetActiveMarkets(context.Background(), &ActiveMarketsParams{
		Limit:  20,
		Page:   2,
		SortBy: SortByNewest,
	})
	if err != nil {
		t.Fatalf("GetActiveMarkets returned error: %v", err)
	}
}

func TestMarketFetcher_GetOrderBook(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/markets/btc/orderbook", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"bids": []map[string]any{
				{"price": 0.51, "size": 100, "side": "buy"},
			},
			"asks": []map[string]any{
				{"price": 0.52, "size": 120, "side": "sell"},
			},
			"tokenId":          "123",
			"adjustedMidpoint": 0.515,
			"maxSpread":        "0.1",
			"minSize":          "10",
			"lastTradePrice":   0.515,
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(WithBaseURL(srv.URL))
	fetcher := NewMarketFetcher(client)

	ob, err := fetcher.GetOrderBook(context.Background(), "btc")
	if err != nil {
		t.Fatalf("GetOrderBook returned error: %v", err)
	}
	if len(ob.Bids) != 1 || len(ob.Asks) != 1 {
		t.Fatalf("expected one bid and one ask, got bids=%d asks=%d", len(ob.Bids), len(ob.Asks))
	}
}

func TestMarketFetcher_GetUserOrders_RequiresAPIKey(t *testing.T) {
	t.Parallel()

	client := NewHttpClient(WithBaseURL("https://example.com"))
	fetcher := NewMarketFetcher(client)

	_, err := fetcher.GetUserOrders(context.Background(), "btc")
	if err == nil {
		t.Fatal("expected GetUserOrders to fail without API key")
	}
	if !strings.Contains(err.Error(), "WithAPIKey") {
		t.Fatalf("expected error to mention WithAPIKey, got: %v", err)
	}
}
