package limitless

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

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
	ChannelSubscribeMarketPrices      SubscriptionChannel = "subscribe_market_prices"
	ChannelSubscribePositions         SubscriptionChannel = "subscribe_positions"
	ChannelSubscribeTransactions      SubscriptionChannel = "subscribe_transactions"
	ChannelSubscribeOrderEvents       SubscriptionChannel = "subscribe_order_events"
	ChannelSubscribeLiveSports        SubscriptionChannel = "subscribe_live_sports"
	ChannelSubscribeLiveEsports       SubscriptionChannel = "subscribe_live_esports"
	ChannelSubscribeMarketLifecycle   SubscriptionChannel = "subscribe_market_lifecycle"
	ChannelUnsubscribeMarketLifecycle SubscriptionChannel = "unsubscribe_market_lifecycle"
)

// SubscriptionOptions contains options for a WebSocket subscription.
type SubscriptionOptions struct {
	MarketSlug      string                 `json:"marketSlug,omitempty"`
	MarketSlugs     []string               `json:"marketSlugs,omitempty"`
	MarketAddress   string                 `json:"marketAddress,omitempty"`
	MarketAddresses []string               `json:"marketAddresses,omitempty"`
	Filters         map[string]interface{} `json:"filters,omitempty"`
}

// flexFloat unmarshals a JSON value that may be either a number or a string containing a number.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(data []byte) error {
	// Try number first
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		*f = flexFloat(n)
		return nil
	}
	// Try quoted string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("flexFloat: cannot parse %q: %w", s, err)
		}
		*f = flexFloat(n)
		return nil
	}
	return fmt.Errorf("flexFloat: cannot unmarshal %s", string(data))
}

func (f flexFloat) Float64() float64 { return float64(f) }

// OrderbookData contains orderbook data within an update event.
type OrderbookData struct {
	Bids             []OrderBookEntry `json:"bids"`
	Asks             []OrderBookEntry `json:"asks"`
	TokenID          string           `json:"tokenId"`
	AdjustedMidpoint float64          `json:"adjustedMidpoint"`
	MaxSpread        flexFloat        `json:"maxSpread"`
	MinSize          flexFloat        `json:"minSize"`
}

// OrderbookUpdate is the orderbook update event from the WebSocket.
type OrderbookUpdate struct {
	MarketSlug string          `json:"marketSlug"`
	Orderbook  OrderbookData   `json:"orderbook"`
	Timestamp  json.RawMessage `json:"timestamp"` // Can be string, number, or date
}

// AmmPriceEntry is a single AMM price entry in newPriceData.updatedPrices.
// CLOB orderbookUpdate events use OrderbookData instead.
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

// OraclePriceData is the oracle price update event.
type OraclePriceData struct {
	MarketAddress *string `json:"marketAddress,omitempty"`
	MarketSlug    string  `json:"marketSlug"`
	Timestamp     int64   `json:"timestamp"`
	Value         float64 `json:"value"`
}

// OrderEvent is the raw order lifecycle event payload.
type OrderEvent = json.RawMessage

// LiveSportsUpdate is the raw live sports snapshot payload.
type LiveSportsUpdate = json.RawMessage

// LiveEsportsUpdate is the raw live esports snapshot payload.
type LiveEsportsUpdate = json.RawMessage

// SystemEvent is the raw websocket system message payload.
type SystemEvent = json.RawMessage

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

// MarketCreatedEvent is emitted when a market becomes visible to websocket lifecycle subscribers.
type MarketCreatedEvent struct {
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Type        string    `json:"type"` // "AMM" or "CLOB"
	GroupSlug   *string   `json:"groupSlug,omitempty"`
	CategoryIDs []int     `json:"categoryIds,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// MarketResolvedEvent is emitted when a market resolves.
type MarketResolvedEvent struct {
	Slug           string    `json:"slug"`
	Type           string    `json:"type"`           // "AMM" or "CLOB"
	WinningOutcome string    `json:"winningOutcome"` // "YES" or "NO"
	WinningIndex   int       `json:"winningIndex"`   // 0 or 1
	ResolutionDate time.Time `json:"resolutionDate"`
}

// WebSocketConfig contains WebSocket connection configuration.
type WebSocketConfig struct {
	URL                  string
	APIKey               string
	HMACCreds            *HMACCredentials
	AutoReconnect        bool
	ReconnectDelayMs     int
	MaxReconnectAttempts int // 0 = infinite
	TimeoutMs            int
}
