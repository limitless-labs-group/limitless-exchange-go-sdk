package limitless

import "encoding/json"

// WebSocketState represents the WebSocket connection state.
type WebSocketState string

const (
	StateDisconnected WebSocketState = "disconnected"
	StateConnecting   WebSocketState = "connecting"
	StateConnected    WebSocketState = "connected"
	StateReconnecting WebSocketState = "reconnecting"
	StateError        WebSocketState = "error"
)

// SubscriptionChannel represents a WebSocket subscription channel.
type SubscriptionChannel string

const (
	ChannelOrderbook             SubscriptionChannel = "orderbook"
	ChannelTrades                SubscriptionChannel = "trades"
	ChannelOrders                SubscriptionChannel = "orders"
	ChannelFills                 SubscriptionChannel = "fills"
	ChannelMarkets               SubscriptionChannel = "markets"
	ChannelPrices                SubscriptionChannel = "prices"
	ChannelSubscribeMarketPrices SubscriptionChannel = "subscribe_market_prices"
	ChannelSubscribePositions    SubscriptionChannel = "subscribe_positions"
	ChannelSubscribeTransactions SubscriptionChannel = "subscribe_transactions"
)

// SubscriptionOptions contains options for a WebSocket subscription.
type SubscriptionOptions struct {
	MarketSlug      string                 `json:"marketSlug,omitempty"`
	MarketSlugs     []string               `json:"marketSlugs,omitempty"`
	MarketAddress   string                 `json:"marketAddress,omitempty"`
	MarketAddresses []string               `json:"marketAddresses,omitempty"`
	Filters         map[string]interface{} `json:"filters,omitempty"`
}

// OrderbookData contains orderbook data within an update event.
type OrderbookData struct {
	Bids             []OrderBookEntry `json:"bids"`
	Asks             []OrderBookEntry `json:"asks"`
	TokenID          string           `json:"tokenId"`
	AdjustedMidpoint float64          `json:"adjustedMidpoint"`
	MaxSpread        float64          `json:"maxSpread"`
	MinSize          float64          `json:"minSize"`
}

// OrderbookUpdate is the orderbook update event from the WebSocket.
type OrderbookUpdate struct {
	MarketSlug string          `json:"marketSlug"`
	Orderbook  OrderbookData   `json:"orderbook"`
	Timestamp  json.RawMessage `json:"timestamp"` // Can be string, number, or date
}

// TradeEvent is emitted when a trade occurs.
type TradeEvent struct {
	MarketSlug string  `json:"marketSlug"`
	Side       string  `json:"side"` // "BUY" or "SELL"
	Price      float64 `json:"price"`
	Size       float64 `json:"size"`
	Timestamp  float64 `json:"timestamp"`
	TradeID    string  `json:"tradeId"`
}

// OrderUpdate is emitted when an order status changes.
type OrderUpdate struct {
	OrderID    string   `json:"orderId"`
	MarketSlug string   `json:"marketSlug"`
	Side       string   `json:"side"` // "BUY" or "SELL"
	Price      *float64 `json:"price,omitempty"`
	Size       float64  `json:"size"`
	Filled     float64  `json:"filled"`
	Status     string   `json:"status"` // "OPEN", "FILLED", "CANCELLED", "PARTIALLY_FILLED"
	Timestamp  float64  `json:"timestamp"`
}

// FillEvent is emitted when an order is filled.
type FillEvent struct {
	OrderID    string  `json:"orderId"`
	MarketSlug string  `json:"marketSlug"`
	Side       string  `json:"side"` // "BUY" or "SELL"
	Price      float64 `json:"price"`
	Size       float64 `json:"size"`
	Timestamp  float64 `json:"timestamp"`
	FillID     string  `json:"fillId"`
}

// MarketUpdateEvent is emitted when market data changes.
type MarketUpdateEvent struct {
	MarketSlug     string   `json:"marketSlug"`
	LastPrice      *float64 `json:"lastPrice,omitempty"`
	Volume24h      *float64 `json:"volume24h,omitempty"`
	PriceChange24h *float64 `json:"priceChange24h,omitempty"`
	Timestamp      float64  `json:"timestamp"`
}

// AmmPriceEntry is a single AMM price entry in updatedPrices array.
type AmmPriceEntry struct {
	MarketID      int     `json:"marketId"`
	MarketAddress string  `json:"marketAddress"`
	YesPrice      float64 `json:"yesPrice"`
	NoPrice       float64 `json:"noPrice"`
}

// NewPriceData is the AMM price update event.
type NewPriceData struct {
	MarketAddress string          `json:"marketAddress"`
	UpdatedPrices []AmmPriceEntry `json:"updatedPrices"`
	BlockNumber   int64           `json:"blockNumber"`
	Timestamp     json.RawMessage `json:"timestamp"` // Can be string, number, or date
}

// TransactionEvent is a blockchain transaction status event.
type TransactionEvent struct {
	UserID           *int    `json:"userId,omitempty"`
	TxHash           *string `json:"txHash,omitempty"`
	Status           string  `json:"status"` // "CONFIRMED" or "FAILED"
	Source           string  `json:"source"`
	Timestamp        string  `json:"timestamp"`
	MarketAddress    *string `json:"marketAddress,omitempty"`
	MarketSlug       *string `json:"marketSlug,omitempty"`
	TokenID          *string `json:"tokenId,omitempty"`
	ConditionID      *string `json:"conditionId,omitempty"`
	AmountContracts  *string `json:"amountContracts,omitempty"`
	AmountCollateral *string `json:"amountCollateral,omitempty"`
	Price            *string `json:"price,omitempty"`
	Side             *string `json:"side,omitempty"` // "BUY" or "SELL"
}

// WebSocketConfig contains WebSocket connection configuration.
type WebSocketConfig struct {
	URL                  string
	APIKey               string
	AutoReconnect        bool
	ReconnectDelayMs     int
	MaxReconnectAttempts int // 0 = infinite
	TimeoutMs            int
}
