package limitless

import (
	"context"
	"fmt"
	"net/url"
	"sync"
)

// MarketFetcher retrieves market data from the Limitless Exchange API.
type MarketFetcher struct {
	client     *HttpClient
	logger     Logger
	venueCache map[string]Venue
	mu         sync.RWMutex
}

// MarketFetcherOption configures a MarketFetcher.
type MarketFetcherOption func(*MarketFetcher)

// WithMarketLogger sets the logger for the MarketFetcher.
func WithMarketLogger(l Logger) MarketFetcherOption {
	return func(f *MarketFetcher) {
		f.logger = l
	}
}

// NewMarketFetcher creates a new market data fetcher.
func NewMarketFetcher(client *HttpClient, opts ...MarketFetcherOption) *MarketFetcher {
	f := &MarketFetcher{
		client:     client,
		logger:     NewNoOpLogger(),
		venueCache: make(map[string]Venue),
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// GetActiveMarkets fetches active markets with optional query parameters.
func (f *MarketFetcher) GetActiveMarkets(ctx context.Context, params *ActiveMarketsParams) (*ActiveMarketsResponse, error) {
	endpoint := "/markets/active"

	if params != nil {
		query := url.Values{}
		if params.Limit > 0 {
			query.Set("limit", fmt.Sprintf("%d", params.Limit))
		}
		if params.Page > 0 {
			query.Set("page", fmt.Sprintf("%d", params.Page))
		}
		if params.SortBy != "" {
			query.Set("sortBy", string(params.SortBy))
		}
		if encoded := query.Encode(); encoded != "" {
			endpoint += "?" + encoded
		}
	}

	f.logger.Debug("Fetching active markets")

	var result ActiveMarketsResponse
	if err := f.client.Get(ctx, endpoint, &result); err != nil {
		f.logger.Error("Failed to fetch active markets", err)
		return nil, err
	}

	// Attach client for fluent API on each market
	for i := range result.Data {
		result.Data[i].client = f.client
	}

	f.logger.Info("Active markets fetched successfully", map[string]any{
		"count": len(result.Data),
		"total": result.TotalMarketsCount,
	})

	return &result, nil
}

// GetMarket fetches a single market by slug and caches its venue data.
func (f *MarketFetcher) GetMarket(ctx context.Context, slug string) (*Market, error) {
	f.logger.Debug("Fetching market", map[string]any{"slug": slug})

	var market Market
	if err := f.client.Get(ctx, "/markets/"+url.PathEscape(slug), &market); err != nil {
		f.logger.Error("Failed to fetch market", err, map[string]any{"slug": slug})
		return nil, err
	}

	// Attach client for fluent API (market.GetUserOrders)
	market.client = f.client

	if market.Venue != nil {
		f.mu.Lock()
		f.venueCache[slug] = *market.Venue
		f.mu.Unlock()

		f.logger.Debug("Venue cached for order signing", map[string]any{
			"slug":     slug,
			"exchange": market.Venue.Exchange,
		})
	} else {
		f.logger.Warn("Market has no venue data", map[string]any{"slug": slug})
	}

	f.logger.Info("Market fetched successfully", map[string]any{
		"slug":  slug,
		"title": market.Title,
	})

	return &market, nil
}

// GetVenue returns cached venue information for a market.
// Returns the venue and true if found, or zero value and false if not cached.
func (f *MarketFetcher) GetVenue(slug string) (Venue, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	v, ok := f.venueCache[slug]
	if ok {
		f.logger.Debug("Venue cache hit", map[string]any{"slug": slug})
	} else {
		f.logger.Debug("Venue cache miss", map[string]any{"slug": slug})
	}
	return v, ok
}

// GetOrderBook fetches the orderbook for a CLOB market.
func (f *MarketFetcher) GetOrderBook(ctx context.Context, slug string) (*OrderBook, error) {
	f.logger.Debug("Fetching orderbook", map[string]any{"slug": slug})

	var ob OrderBook
	if err := f.client.Get(ctx, fmt.Sprintf("/markets/%s/orderbook", url.PathEscape(slug)), &ob); err != nil {
		f.logger.Error("Failed to fetch orderbook", err, map[string]any{"slug": slug})
		return nil, err
	}

	f.logger.Info("Orderbook fetched successfully", map[string]any{
		"slug": slug,
		"bids": len(ob.Bids),
		"asks": len(ob.Asks),
	})

	return &ob, nil
}

// GetUserOrders fetches the authenticated user's orders for a specific market.
// Requires an API key to be set on the HttpClient.
func (f *MarketFetcher) GetUserOrders(ctx context.Context, slug string) ([]UserOrder, error) {
	if err := f.client.requireAuth("GetUserOrders"); err != nil {
		return nil, err
	}

	f.logger.Debug("Fetching user orders", map[string]any{"slug": slug})

	var orders []UserOrder
	if err := f.client.Get(ctx, fmt.Sprintf("/markets/%s/user-orders", url.PathEscape(slug)), &orders); err != nil {
		f.logger.Error("Failed to fetch user orders", err, map[string]any{"slug": slug})
		return nil, err
	}

	f.logger.Info("User orders fetched successfully", map[string]any{
		"slug":  slug,
		"count": len(orders),
	})

	return orders, nil
}

// GetUserOrders fetches the authenticated user's orders for this market.
//
// Deprecated: Prefer MarketFetcher.GetUserOrders or Client.Markets.GetUserOrders
// to keep model values passive and avoid hidden client state.
// The Market must have been obtained via MarketFetcher.GetMarket() or
// MarketFetcher.GetActiveMarkets() so that the HTTP client is attached.
func (m *Market) GetUserOrders(ctx context.Context) ([]UserOrder, error) {
	if m.client == nil {
		return nil, fmt.Errorf(
			"this Market instance has no HTTP client attached; " +
				"fetch the market via MarketFetcher.GetMarket() to use this method",
		)
	}
	if err := m.client.requireAuth("Market.GetUserOrders"); err != nil {
		return nil, err
	}

	var orders []UserOrder
	if err := m.client.Get(ctx, fmt.Sprintf("/markets/%s/user-orders", url.PathEscape(m.Slug)), &orders); err != nil {
		return nil, err
	}

	return orders, nil
}
