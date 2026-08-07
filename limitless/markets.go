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
	result, err := f.GetActiveMarketsWithRawResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// GetActiveMarketsWithRawResponse is the raw-response variant of GetActiveMarkets.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (f *MarketFetcher) GetActiveMarketsWithRawResponse(ctx context.Context, params *ActiveMarketsParams) (*RawResult[ActiveMarketsResponse], error) {
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

	raw, err := f.client.GetRaw(ctx, endpoint)
	result, err := decodeRawResult[ActiveMarketsResponse](raw, err)
	if err != nil {
		f.logger.Error("Failed to fetch active markets", err)
		return nil, err
	}

	// Attach client for fluent API on each market
	for i := range result.Data.Data {
		result.Data.Data[i].client = f.client
	}

	f.logger.Info("Active markets fetched successfully", map[string]any{
		"count": len(result.Data.Data),
		"total": result.Data.TotalMarketsCount,
	})

	return result, nil
}

// GetMarket fetches a single market by slug and caches its venue data.
func (f *MarketFetcher) GetMarket(ctx context.Context, slug string) (*Market, error) {
	result, err := f.GetMarketWithRawResponse(ctx, slug)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// GetMarketWithRawResponse is the raw-response variant of GetMarket.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (f *MarketFetcher) GetMarketWithRawResponse(ctx context.Context, slug string) (*RawResult[Market], error) {
	f.logger.Debug("Fetching market", map[string]any{"slug": slug})

	raw, err := f.client.GetRaw(ctx, "/markets/"+url.PathEscape(slug))
	result, err := decodeRawResult[Market](raw, err)
	if err != nil {
		f.logger.Error("Failed to fetch market", err, map[string]any{"slug": slug})
		return nil, err
	}

	// Attach client for fluent API (market.GetUserOrders)
	result.Data.client = f.client

	if result.Data.Venue != nil {
		f.mu.Lock()
		f.venueCache[slug] = *result.Data.Venue
		f.mu.Unlock()

		f.logger.Debug("Venue cached for order signing", map[string]any{
			"slug":     slug,
			"exchange": result.Data.Venue.Exchange,
		})
	} else {
		f.logger.Warn("Market has no venue data", map[string]any{"slug": slug})
	}

	f.logger.Info("Market fetched successfully", map[string]any{
		"slug":  slug,
		"title": result.Data.Title,
	})

	return result, nil
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
	result, err := f.GetOrderBookWithRawResponse(ctx, slug)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// GetOrderBookWithRawResponse is the raw-response variant of GetOrderBook.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (f *MarketFetcher) GetOrderBookWithRawResponse(ctx context.Context, slug string) (*RawResult[OrderBook], error) {
	f.logger.Debug("Fetching orderbook", map[string]any{"slug": slug})

	raw, err := f.client.GetRaw(ctx, fmt.Sprintf("/markets/%s/orderbook", url.PathEscape(slug)))
	result, err := decodeRawResult[OrderBook](raw, err)
	if err != nil {
		f.logger.Error("Failed to fetch orderbook", err, map[string]any{"slug": slug})
		return nil, err
	}

	f.logger.Info("Orderbook fetched successfully", map[string]any{
		"slug": slug,
		"bids": len(result.Data.Bids),
		"asks": len(result.Data.Asks),
	})

	return result, nil
}

// GetUserOrders fetches the authenticated user's orders for a specific market.
// Requires an API key to be set on the HttpClient.
func (f *MarketFetcher) GetUserOrders(ctx context.Context, slug string) ([]UserOrder, error) {
	result, err := f.GetUserOrdersWithRawResponse(ctx, slug)
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// GetUserOrdersWithRawResponse is the raw-response variant of GetUserOrders.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (f *MarketFetcher) GetUserOrdersWithRawResponse(ctx context.Context, slug string) (*RawResult[[]UserOrder], error) {
	if err := f.client.requireAuth("GetUserOrders"); err != nil {
		return nil, err
	}

	f.logger.Debug("Fetching user orders", map[string]any{"slug": slug})

	raw, err := f.client.GetRaw(ctx, fmt.Sprintf("/markets/%s/user-orders", url.PathEscape(slug)))
	result, err := decodeRawResult[[]UserOrder](raw, err)
	if err != nil {
		f.logger.Error("Failed to fetch user orders", err, map[string]any{"slug": slug})
		return nil, err
	}

	f.logger.Info("User orders fetched successfully", map[string]any{
		"slug":  slug,
		"count": len(result.Data),
	})

	return result, nil
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
