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

func TestMarketFetcher_GetUserOrders_DecodesWireShape(t *testing.T) {
	t.Parallel()

	// Exact wire shape of GET /markets/{slug}/user-orders (API UserOrder DTO:
	// numeric values are decimal strings, side/type are string enums, marketId is text).
	const wire = `[{
		"id": "6f1c2a0e-7d7f-4a6e-9c0b-1f2e3d4c5b6a",
		"ownerId": 12345,
		"marketId": "9876",
		"token": "19633204485790857949828516737993423758628930235371629943999544859324645414627",
		"type": "GTC",
		"status": "LIVE",
		"side": "BUY",
		"makerAmount": "5000000",
		"takerAmount": "10000000",
		"price": "0.5",
		"originalSize": "10000000",
		"remainingSize": "10000000",
		"createdAt": "2026-09-30T19:28:00.000Z",
		"clientOrderId": "my-tag-42"
	}]`

	mux := http.NewServeMux()
	mux.HandleFunc("/markets/btc/user-orders", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(wire))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("test-key"))
	fetcher := NewMarketFetcher(client)

	orders, err := fetcher.GetUserOrders(context.Background(), "btc")
	if err != nil {
		t.Fatalf("GetUserOrders returned error: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected one order, got %d", len(orders))
	}
	o := orders[0]
	if o.ID != "6f1c2a0e-7d7f-4a6e-9c0b-1f2e3d4c5b6a" || o.OwnerID != 12345 {
		t.Fatalf("unexpected id/ownerId: %+v", o)
	}
	if o.MarketID != "9876" {
		t.Fatalf("expected marketId %q, got %q", "9876", o.MarketID)
	}
	if o.Token != "19633204485790857949828516737993423758628930235371629943999544859324645414627" {
		t.Fatalf("unexpected token: %q", o.Token)
	}
	if o.Type != "GTC" || o.Status != "LIVE" || o.Side != "BUY" {
		t.Fatalf("unexpected type/status/side: %+v", o)
	}
	if o.MakerAmount != "5000000" || o.TakerAmount != "10000000" || o.Price != "0.5" {
		t.Fatalf("unexpected amounts/price: %+v", o)
	}
	if o.OriginalSize != "10000000" || o.RemainingSize != "10000000" {
		t.Fatalf("unexpected sizes: %+v", o)
	}
	if o.CreatedAt != "2026-09-30T19:28:00.000Z" {
		t.Fatalf("unexpected createdAt: %q", o.CreatedAt)
	}
	if o.ClientOrderID == nil || *o.ClientOrderID != "my-tag-42" {
		t.Fatalf("expected clientOrderId my-tag-42, got %v", o.ClientOrderID)
	}
}

func TestUserOrder_DecodesWithoutClientOrderID(t *testing.T) {
	t.Parallel()

	var orders []UserOrder
	err := json.Unmarshal([]byte(`[{"id":"a","ownerId":1,"marketId":"2","token":"3","type":"FAK","status":"MATCHED","side":"SELL","makerAmount":"1","takerAmount":"2","price":"0.25","originalSize":"4","remainingSize":"0","createdAt":"2026-09-30T19:28:00.000Z"}]`), &orders)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if orders[0].ClientOrderID != nil {
		t.Fatalf("expected nil clientOrderId, got %q", *orders[0].ClientOrderID)
	}
	if orders[0].Side != "SELL" || orders[0].Type != "FAK" {
		t.Fatalf("unexpected side/type: %+v", orders[0])
	}
}

func TestMarketFetcher_GetActiveMarkets_SortByTrending(t *testing.T) {
	t.Parallel()

	if SortByTrending != "trending" {
		t.Fatalf("expected SortByTrending to be \"trending\", got %q", SortByTrending)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/markets/active", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("sortBy"); got != "trending" {
			t.Fatalf("expected sortBy=trending, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "totalMarketsCount": 0})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	fetcher := NewMarketFetcher(NewHttpClient(WithBaseURL(srv.URL)))
	if _, err := fetcher.GetActiveMarkets(context.Background(), &ActiveMarketsParams{SortBy: SortByTrending}); err != nil {
		t.Fatalf("GetActiveMarkets returned error: %v", err)
	}
}

func TestOrderBook_DecodesStringAndNumberScalars(t *testing.T) {
	t.Parallel()

	// SQL read path (live prod 2026-09-30): maxSpread/minSize are decimal strings.
	var fromStrings OrderBook
	if err := json.Unmarshal([]byte(`{
		"bids":[],"asks":[],
		"tokenId":"123",
		"adjustedMidpoint":0.885,
		"midpoint":0.885,
		"maxSpread":"0.025",
		"minSize":"150000000",
		"lastTradePrice":null
	}`), &fromStrings); err != nil {
		t.Fatalf("string-scalar orderbook failed to decode: %v", err)
	}
	if fromStrings.MaxSpread.Float64() != 0.025 || fromStrings.MinSize.Float64() != 150000000 {
		t.Fatalf("unexpected string-scalar values: maxSpread=%v minSize=%v", fromStrings.MaxSpread, fromStrings.MinSize)
	}
	if fromStrings.Midpoint != 0.885 {
		t.Fatalf("expected midpoint 0.885, got %v", fromStrings.Midpoint)
	}
	if fromStrings.LastTradePrice != nil {
		t.Fatalf("expected nil lastTradePrice, got %v", *fromStrings.LastTradePrice)
	}

	// Projection read path and the declared DTO: maxSpread/minSize are JSON numbers.
	var fromNumbers OrderBook
	if err := json.Unmarshal([]byte(`{
		"bids":[{"price":0.51,"size":100,"side":"BUY"}],
		"asks":[{"price":0.52,"size":120,"side":"SELL"}],
		"tokenId":"123",
		"adjustedMidpoint":0.515,
		"midpoint":0.515,
		"maxSpread":0.05,
		"minSize":1,
		"lastTradePrice":0.75
	}`), &fromNumbers); err != nil {
		t.Fatalf("number-scalar orderbook failed to decode: %v", err)
	}
	if fromNumbers.MaxSpread.Float64() != 0.05 || fromNumbers.MinSize.Float64() != 1 {
		t.Fatalf("unexpected number-scalar values: maxSpread=%v minSize=%v", fromNumbers.MaxSpread, fromNumbers.MinSize)
	}
	if fromNumbers.LastTradePrice == nil || *fromNumbers.LastTradePrice != 0.75 {
		t.Fatalf("expected lastTradePrice 0.75, got %v", fromNumbers.LastTradePrice)
	}
}

func TestMarket_DecodesPublicMetadataAndTakerDelay(t *testing.T) {
	t.Parallel()

	var m Market
	if err := json.Unmarshal([]byte(`{
		"id": 7,
		"slug": "psg-vs-lyon",
		"title": "PSG vs Lyon",
		"metadata": {
			"fee": true,
			"externalSlug": "psg-lyon-2026",
			"externalProvider": "polymarket",
			"feeRate": 0.03,
			"fixtureId": 1234567,
			"homeTeamLogo": "https://cdn.example/psg.png",
			"awayTeamLogo": "https://cdn.example/lyon.png",
			"outcomeLabels": {"0": "PSG", "1": "Lyon"}
		},
		"settings": {
			"minSize": "5",
			"maxSpread": 0.05,
			"dailyReward": "0",
			"rewardsEpoch": null,
			"c": null,
			"takerDelayMs": 1250
		}
	}`), &m); err != nil {
		t.Fatalf("market failed to decode: %v", err)
	}
	md := m.Metadata
	if md.ExternalSlug == nil || *md.ExternalSlug != "psg-lyon-2026" {
		t.Fatalf("expected externalSlug, got %v", md.ExternalSlug)
	}
	if md.ExternalProvider == nil || *md.ExternalProvider != "polymarket" {
		t.Fatalf("expected externalProvider, got %v", md.ExternalProvider)
	}
	if md.FeeRate == nil || *md.FeeRate != 0.03 {
		t.Fatalf("expected feeRate 0.03, got %v", md.FeeRate)
	}
	if md.FixtureID == nil || *md.FixtureID != 1234567 {
		t.Fatalf("expected fixtureId, got %v", md.FixtureID)
	}
	if md.HomeTeamLogo == nil || md.AwayTeamLogo == nil {
		t.Fatalf("expected team logos, got %v / %v", md.HomeTeamLogo, md.AwayTeamLogo)
	}
	if md.OutcomeLabels["0"] != "PSG" || md.OutcomeLabels["1"] != "Lyon" {
		t.Fatalf("unexpected outcomeLabels: %v", md.OutcomeLabels)
	}
	if m.Settings == nil || m.Settings.TakerDelayMs != 1250 {
		t.Fatalf("expected takerDelayMs 1250, got %+v", m.Settings)
	}
}
