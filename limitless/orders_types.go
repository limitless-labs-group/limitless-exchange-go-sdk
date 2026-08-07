package limitless

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Side represents the order side.
type Side int

const (
	SideBuy  Side = 0
	SideSell Side = 1
)

// OrderType represents the order type.
type OrderType string

const (
	OrderTypeFOK OrderType = "FOK"
	OrderTypeFAK OrderType = "FAK"
	OrderTypeGTC OrderType = "GTC"
)

// SignatureType represents the signature type for order signing.
type SignatureType int

const (
	SignatureTypeEOA            SignatureType = 0
	SignatureTypePolyProxy      SignatureType = 1
	SignatureTypePolyGnosisSafe SignatureType = 2
)

// FOKOrderArgs contains arguments for a Fill-or-Kill market order.
// MakerAmount is human-readable (max 6 decimals):
//   - BUY: USDC to spend (e.g., 50 = $50 USDC)
//   - SELL: shares to sell (e.g., 18.64 shares)
type FOKOrderArgs struct {
	TokenID     string
	Side        Side
	MakerAmount float64
	Expiration  string // Default: "0"
	Nonce       *int
	Taker       string // Default: ZeroAddress
}

func (FOKOrderArgs) orderArgs() {}

// GTCOrderArgs contains arguments for a Good-Til-Cancelled limit order.
type GTCOrderArgs struct {
	TokenID    string
	Side       Side
	Price      float64 // 0.0-1.0, tick-aligned to 0.001
	Size       float64 // Number of shares
	Expiration string
	Nonce      *int
	Taker      string
	PostOnly   bool // Optional. When true, rejects the order if it would immediately match. Default false.
}

func (GTCOrderArgs) orderArgs() {}

// FAKOrderArgs contains arguments for a Fill-And-Kill limit order.
// FAK orders share the price/size construction with GTC, but the unmatched
// remainder is killed instead of resting on the book. PostOnly is not supported
// for FAK and is rejected by the API.
type FAKOrderArgs struct {
	TokenID    string
	Side       Side
	Price      float64 // 0.0-1.0, tick-aligned to 0.001
	Size       float64 // Number of shares
	Expiration string
	Nonce      *int
	Taker      string
}

func (FAKOrderArgs) orderArgs() {}

// OrderArgs is the interface satisfied by FOKOrderArgs, GTCOrderArgs, and FAKOrderArgs.
type OrderArgs interface {
	orderArgs()
}

// UnsignedOrder is an order payload ready for EIP-712 signing.
type UnsignedOrder struct {
	Salt          int64         `json:"salt"`
	Maker         string        `json:"maker"`
	Signer        string        `json:"signer"`
	Taker         string        `json:"taker"`
	TokenID       string        `json:"tokenId"`
	MakerAmount   int64         `json:"makerAmount"`
	TakerAmount   int64         `json:"takerAmount"`
	Expiration    string        `json:"expiration"`
	Nonce         int           `json:"nonce"`
	FeeRateBps    int           `json:"feeRateBps"`
	Side          Side          `json:"side"`
	SignatureType SignatureType `json:"signatureType"`
	Price         *float64      `json:"price,omitempty"`
}

// SignedOrder is an unsigned order with an EIP-712 signature attached.
type SignedOrder struct {
	Salt          int64         `json:"salt"`
	Maker         string        `json:"maker"`
	Signer        string        `json:"signer"`
	Taker         string        `json:"taker"`
	TokenID       string        `json:"tokenId"`
	MakerAmount   int64         `json:"makerAmount"`
	TakerAmount   int64         `json:"takerAmount"`
	Expiration    string        `json:"expiration"`
	Nonce         int           `json:"nonce"`
	FeeRateBps    int           `json:"feeRateBps"`
	Side          Side          `json:"side"`
	SignatureType SignatureType `json:"signatureType"`
	Price         *float64      `json:"price,omitempty"`
	Signature     string        `json:"signature"`
}

// ReceiveWindowOptions contains optional freshness controls for POST /orders.
//
// Timestamp and RecvWindow are serialized as top-level request fields only.
// They are not part of the EIP-712 signed order payload.
type ReceiveWindowOptions struct {
	// Timestamp is the client-stamped Unix time in milliseconds.
	Timestamp *int64

	// RecvWindow is the maximum allowed request staleness in milliseconds.
	RecvWindow *int64
}

// NewOrderPayload is the payload submitted to the API for order creation.
type NewOrderPayload struct {
	Order      SignedOrder `json:"order"`
	OrderType  OrderType   `json:"orderType"`
	MarketSlug string      `json:"marketSlug"`
	OwnerID    int         `json:"ownerId"`
	PostOnly   *bool       `json:"postOnly,omitempty"`
	Timestamp  *int64      `json:"timestamp,omitempty"`
	RecvWindow *int64      `json:"recvWindow,omitempty"`
}

// CreatedOrder contains order data returned from the API.
type CreatedOrder struct {
	ID            string   `json:"id"`
	CreatedAt     string   `json:"createdAt"`
	MakerAmount   int64    `json:"makerAmount"`
	TakerAmount   int64    `json:"takerAmount"`
	Expiration    *string  `json:"expiration"`
	SignatureType int      `json:"signatureType"`
	Salt          int64    `json:"salt"`
	Maker         string   `json:"maker"`
	Signer        string   `json:"signer"`
	Taker         string   `json:"taker"`
	TokenID       string   `json:"tokenId"`
	Side          Side     `json:"side"`
	FeeRateBps    int      `json:"feeRateBps"`
	Nonce         int      `json:"nonce"`
	Signature     string   `json:"signature"`
	OrderType     string   `json:"orderType"`
	Price         *float64 `json:"price"`
	MarketID      int      `json:"marketId"`
}

// UnmarshalJSON accepts both JSON numbers and JSON strings for
// makerAmount, takerAmount, price, and salt in create-order responses.
func (c *CreatedOrder) UnmarshalJSON(data []byte) error {
	type createdOrderWire struct {
		ID            string          `json:"id"`
		CreatedAt     string          `json:"createdAt"`
		MakerAmount   json.RawMessage `json:"makerAmount"`
		TakerAmount   json.RawMessage `json:"takerAmount"`
		Expiration    *string         `json:"expiration"`
		SignatureType int             `json:"signatureType"`
		Salt          json.RawMessage `json:"salt"`
		Maker         string          `json:"maker"`
		Signer        string          `json:"signer"`
		Taker         string          `json:"taker"`
		TokenID       string          `json:"tokenId"`
		Side          Side            `json:"side"`
		FeeRateBps    int             `json:"feeRateBps"`
		Nonce         int             `json:"nonce"`
		Signature     string          `json:"signature"`
		OrderType     string          `json:"orderType"`
		Price         json.RawMessage `json:"price"`
		MarketID      int             `json:"marketId"`
	}

	var wire createdOrderWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	makerAmount, err := parseInt64FromNumberOrString(wire.MakerAmount)
	if err != nil {
		return fmt.Errorf("makerAmount: %w", err)
	}

	takerAmount, err := parseInt64FromNumberOrString(wire.TakerAmount)
	if err != nil {
		return fmt.Errorf("takerAmount: %w", err)
	}

	salt, err := parseInt64FromNumberOrString(wire.Salt)
	if err != nil {
		return fmt.Errorf("salt: %w", err)
	}

	price, err := parseNullableFloat64FromNumberOrString(wire.Price)
	if err != nil {
		return fmt.Errorf("price: %w", err)
	}

	*c = CreatedOrder{
		ID:            wire.ID,
		CreatedAt:     wire.CreatedAt,
		MakerAmount:   makerAmount,
		TakerAmount:   takerAmount,
		Expiration:    wire.Expiration,
		SignatureType: wire.SignatureType,
		Salt:          salt,
		Maker:         wire.Maker,
		Signer:        wire.Signer,
		Taker:         wire.Taker,
		TokenID:       wire.TokenID,
		Side:          wire.Side,
		FeeRateBps:    wire.FeeRateBps,
		Nonce:         wire.Nonce,
		Signature:     wire.Signature,
		OrderType:     wire.OrderType,
		Price:         price,
		MarketID:      wire.MarketID,
	}

	return nil
}

func parseInt64FromNumberOrString(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 {
		return 0, fmt.Errorf("missing value")
	}

	var num int64
	if err := json.Unmarshal(raw, &num); err == nil {
		return num, nil
	}

	var str string
	if err := json.Unmarshal(raw, &str); err != nil {
		return 0, fmt.Errorf("expected number or numeric string, got %s", string(raw))
	}

	parsed, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid int64 value %q: %w", str, err)
	}

	return parsed, nil
}

func parseNullableFloat64FromNumberOrString(raw json.RawMessage) (*float64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var num float64
	if err := json.Unmarshal(raw, &num); err == nil {
		return &num, nil
	}

	var str string
	if err := json.Unmarshal(raw, &str); err != nil {
		return nil, fmt.Errorf("expected number, numeric string, or null, got %s", string(raw))
	}

	parsed, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid float64 value %q: %w", str, err)
	}

	return &parsed, nil
}

// OrderMatch contains match information for filled orders.
type OrderMatch struct {
	ID          string  `json:"id"`
	CreatedAt   *string `json:"createdAt"`
	MatchedSize string  `json:"matchedSize"`
	OrderID     string  `json:"orderId"`
}

// OrderExecutionTotalsRaw holds the raw integer-string settlement totals for an
// order execution. All six fields are decimal strings (raw integer strings).
type OrderExecutionTotalsRaw struct {
	ContractsGross string `json:"contractsGross"`
	ContractsFee   string `json:"contractsFee"`
	ContractsNet   string `json:"contractsNet"`
	UsdGross       string `json:"usdGross"`
	UsdFee         string `json:"usdFee"`
	UsdNet         string `json:"usdNet"`
}

// OrderExecution carries the taker-delay outcome plus the settlement/fee summary
// returned alongside a created order.
type OrderExecution struct {
	Matched bool `json:"matched"`
	// SettlementStatus is a plain string, not a closed enum: the server adds
	// values over time. Known values:
	// "UNMATCHED" | "MATCHED" | "MINED" | "CONFIRMED" | "RETRYING" | "FAILED" | "DELAYED".
	// "DELAYED" = the taker order was accepted but is held by a per-market taker
	// delay before being released to the matching engine.
	SettlementStatus string   `json:"settlementStatus"`
	TradeEventID     *string  `json:"tradeEventId,omitempty"`
	TxHash           *string  `json:"txHash,omitempty"`
	ClientOrderID    *string  `json:"clientOrderId,omitempty"`
	Reason           *string  `json:"reason,omitempty"`
	StpMakerCancels  []string `json:"stpMakerCancels,omitempty"`
	// EligibleAt is an ISO-8601 timestamp present only when
	// SettlementStatus == "DELAYED": the time the order is released to the
	// matching engine.
	EligibleAt      *string                 `json:"eligibleAt,omitempty"`
	FeeRateBps      float64                 `json:"feeRateBps"`
	EffectiveFeeBps float64                 `json:"effectiveFeeBps"`
	TotalsRaw       OrderExecutionTotalsRaw `json:"totalsRaw"`
}

// OrderResponse is returned after successfully creating an order.
type OrderResponse struct {
	Order        CreatedOrder    `json:"order"`
	MakerMatches []OrderMatch    `json:"makerMatches,omitempty"`
	Execution    *OrderExecution `json:"execution,omitempty"`
}

// OrderSigningConfig contains EIP-712 signing configuration.
type OrderSigningConfig struct {
	ChainID         int
	ContractAddress string // Fallback verifyingContract. Prefer market.venue.exchange when available.
}

// CreateOrderParams contains parameters for creating an order.
type CreateOrderParams struct {
	OrderType     OrderType
	MarketSlug    string
	Args          OrderArgs
	ReceiveWindow ReceiveWindowOptions
}

// UserData contains user-specific information for order operations.
type UserData struct {
	UserID     int
	FeeRateBps int
}
