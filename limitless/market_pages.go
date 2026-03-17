package limitless

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const maxRedirectDepth = 3

// MarketPageFetcher provides access to the navigation-driven market discovery API.
type MarketPageFetcher struct {
	client *HttpClient
	logger Logger
}

// MarketPageOption configures a MarketPageFetcher.
type MarketPageOption func(*MarketPageFetcher)

// WithMarketPageLogger sets the logger for the MarketPageFetcher.
func WithMarketPageLogger(l Logger) MarketPageOption {
	return func(f *MarketPageFetcher) {
		f.logger = l
	}
}

// NewMarketPageFetcher creates a new market page fetcher.
func NewMarketPageFetcher(client *HttpClient, opts ...MarketPageOption) *MarketPageFetcher {
	f := &MarketPageFetcher{
		client: client,
		logger: NewNoOpLogger(),
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// GetNavigation fetches the navigation tree.
func (f *MarketPageFetcher) GetNavigation(ctx context.Context) ([]NavigationNode, error) {
	f.logger.Debug("Fetching navigation tree")

	var nodes []NavigationNode
	if err := f.client.Get(ctx, "/navigation", &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

// GetMarketPageByPath resolves a market page by its path.
// Handles 301 redirects manually up to maxRedirectDepth.
func (f *MarketPageFetcher) GetMarketPageByPath(ctx context.Context, path string) (*MarketPage, error) {
	return f.getMarketPageByPathInternal(ctx, path, 0)
}

func (f *MarketPageFetcher) getMarketPageByPathInternal(ctx context.Context, path string, depth int) (*MarketPage, error) {
	query := url.Values{}
	query.Set("path", path)
	endpoint := "/market-pages/by-path?" + query.Encode()

	f.logger.Debug("Resolving market page by path", map[string]any{"path": path, "depth": depth})

	resp, err := f.client.GetRaw(ctx, endpoint, AllowStatus(301))
	if err != nil {
		return nil, err
	}

	if resp.Status == 200 {
		var page MarketPage
		if err := json.Unmarshal(resp.Body, &page); err != nil {
			return nil, fmt.Errorf("failed to decode market page: %w", err)
		}
		return &page, nil
	}

	if resp.Status != 301 {
		return nil, fmt.Errorf("unexpected response status: %d", resp.Status)
	}

	if depth >= maxRedirectDepth {
		return nil, fmt.Errorf("too many redirects while resolving market page path '%s' (max %d)", path, maxRedirectDepth)
	}

	location := resp.Headers.Get("Location")
	if location == "" {
		return nil, fmt.Errorf("redirect response missing valid Location header")
	}

	redirectedPath, err := f.extractRedirectPath(location)
	if err != nil {
		return nil, err
	}

	f.logger.Info("Following market page redirect", map[string]any{
		"from":  path,
		"to":    redirectedPath,
		"depth": depth + 1,
	})

	return f.getMarketPageByPathInternal(ctx, redirectedPath, depth+1)
}

func (f *MarketPageFetcher) extractRedirectPath(location string) (string, error) {
	const directByPathPrefix = "/market-pages/by-path"

	// Already in by-path format
	if strings.HasPrefix(location, directByPathPrefix) {
		u, err := url.Parse("https://api.limitless.exchange" + location)
		if err == nil {
			if p := u.Query().Get("path"); p != "" {
				return p, nil
			}
		}
		return "", fmt.Errorf("redirect location '%s' is missing required 'path' query parameter", directByPathPrefix)
	}

	// Absolute URL
	if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		u, err := url.Parse(location)
		if err == nil {
			if u.Path == directByPathPrefix {
				if p := u.Query().Get("path"); p != "" {
					return p, nil
				}
				return "", fmt.Errorf("redirect location '%s' is missing required 'path' query parameter", directByPathPrefix)
			}
			if u.Path != "" {
				return u.Path, nil
			}
		}
		return "/", nil
	}

	// Path redirect (e.g. /sports/nba)
	return location, nil
}

// GetMarkets fetches markets for a market page with optional filtering and pagination.
func (f *MarketPageFetcher) GetMarkets(ctx context.Context, pageID string, params *MarketPageMarketsParams) (*MarketPageMarketsResponse, error) {
	if params != nil && params.Cursor != "" && params.Page != nil {
		return nil, fmt.Errorf("parameters `cursor` and `page` are mutually exclusive")
	}

	query := url.Values{}

	if params != nil {
		if params.Page != nil {
			query.Set("page", fmt.Sprintf("%d", *params.Page))
		}
		if params.Limit != nil {
			query.Set("limit", fmt.Sprintf("%d", *params.Limit))
		}
		if params.Sort != "" {
			query.Set("sort", string(params.Sort))
		}
		if params.Cursor != "" {
			query.Set("cursor", params.Cursor)
		}
		for key, value := range params.Filters {
			if arr, ok := value.([]any); ok {
				for _, item := range arr {
					query.Add(key, stringifyFilterValue(item))
				}
			} else {
				query.Add(key, stringifyFilterValue(value))
			}
		}
	}

	queryString := query.Encode()
	endpoint := "/market-pages/" + pageID + "/markets"
	if queryString != "" {
		endpoint += "?" + queryString
	}

	f.logger.Debug("Fetching market-page markets", map[string]any{"pageId": pageID})

	var resp MarketPageMarketsResponse
	if err := f.client.Get(ctx, endpoint, &resp); err != nil {
		return nil, err
	}

	if resp.Pagination == nil && resp.Cursor == nil {
		return nil, fmt.Errorf("invalid market-page response: expected `pagination` or `cursor` metadata")
	}

	return &resp, nil
}

// stringifyFilterValue converts a filter value to a string.
func stringifyFilterValue(v any) string {
	switch val := v.(type) {
	case bool:
		if val {
			return "true"
		}
		return "false"
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

// GetPropertyKeys lists all property keys with options.
func (f *MarketPageFetcher) GetPropertyKeys(ctx context.Context) ([]PropertyKey, error) {
	var keys []PropertyKey
	if err := f.client.Get(ctx, "/property-keys", &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

// GetPropertyKey fetches a single property key by ID.
func (f *MarketPageFetcher) GetPropertyKey(ctx context.Context, id string) (*PropertyKey, error) {
	var key PropertyKey
	if err := f.client.Get(ctx, "/property-keys/"+id, &key); err != nil {
		return nil, err
	}
	return &key, nil
}

// GetPropertyOptions lists options for a property key, optionally filtered by parent option ID.
func (f *MarketPageFetcher) GetPropertyOptions(ctx context.Context, keyID string, parentID *string) ([]PropertyOption, error) {
	endpoint := "/property-keys/" + keyID + "/options"
	if parentID != nil && *parentID != "" {
		query := url.Values{}
		query.Set("parentId", *parentID)
		endpoint += "?" + query.Encode()
	}

	var options []PropertyOption
	if err := f.client.Get(ctx, endpoint, &options); err != nil {
		return nil, err
	}
	return options, nil
}
