package limitless

import "fmt"

// Client is the idiomatic top-level SDK entrypoint.
// It wires a shared HTTP client into the domain services.
type Client struct {
	HTTP            *HttpClient
	Markets         *MarketFetcher
	Portfolio       *PortfolioFetcher
	Pages           *MarketPageFetcher
	ApiTokens       *ApiTokenService
	PartnerAccounts *PartnerAccountService
	DelegatedOrders *DelegatedOrderService
}

// NewClient constructs a root SDK client with shared domain services.
func NewClient(opts ...ClientOption) *Client {
	return NewClientFromHTTP(NewHttpClient(opts...))
}

// NewClientFromHTTP constructs a root SDK client from an existing HTTP client.
func NewClientFromHTTP(httpClient *HttpClient) *Client {
	logger := NewNoOpLogger()
	if httpClient != nil && httpClient.logger != nil {
		logger = httpClient.logger
	}

	return &Client{
		HTTP:            httpClient,
		Markets:         NewMarketFetcher(httpClient, WithMarketLogger(logger)),
		Portfolio:       NewPortfolioFetcher(httpClient, WithPortfolioLogger(logger)),
		Pages:           NewMarketPageFetcher(httpClient, WithMarketPageLogger(logger)),
		ApiTokens:       NewApiTokenService(httpClient, WithApiTokenLogger(logger)),
		PartnerAccounts: NewPartnerAccountService(httpClient, WithPartnerAccountLogger(logger)),
		DelegatedOrders: NewDelegatedOrderService(httpClient, WithDelegatedOrderLogger(logger)),
	}
}

// NewOrderClient creates an order client that reuses the root client's shared services.
func (c *Client) NewOrderClient(privateKeyHex string, opts ...OrderClientOption) (*OrderClient, error) {
	if c == nil || c.HTTP == nil {
		return nil, fmt.Errorf("client HTTP transport is not configured")
	}

	merged := make([]OrderClientOption, 0, len(opts)+3)
	if c.Markets != nil {
		merged = append(merged, WithOrderMarketFetcher(c.Markets))
	}
	if c.Portfolio != nil {
		merged = append(merged, WithOrderPortfolioFetcher(c.Portfolio))
	}
	if c.HTTP.logger != nil {
		merged = append(merged, WithOrderLogger(c.HTTP.logger))
	}
	merged = append(merged, opts...)

	return NewOrderClient(c.HTTP, privateKeyHex, merged...)
}

// NewWebSocketClient creates a WebSocket client and reuses the HTTP client's API key when present.
func (c *Client) NewWebSocketClient(opts ...WebSocketOption) *WebSocketClient {
	merged := make([]WebSocketOption, 0, len(opts)+3)
	if c != nil && c.HTTP != nil {
		if apiKey := c.HTTP.APIKey(); apiKey != "" {
			merged = append(merged, WithWebSocketAPIKey(apiKey))
		}
		if hmacCreds := c.HTTP.HMACCredentials(); hmacCreds != nil {
			merged = append(merged, WithWebSocketHMACCredentials(*hmacCreds))
		}
		if c.HTTP.logger != nil {
			merged = append(merged, WithWebSocketLogger(c.HTTP.logger))
		}
	}
	merged = append(merged, opts...)
	return NewWebSocketClient(merged...)
}
