package limitless

// CreateDelegatedOrderParams contains the parameters required for delegated signing.
type CreateDelegatedOrderParams struct {
	MarketSlug    string
	OrderType     OrderType
	OnBehalfOf    int
	FeeRateBps    int
	Args          OrderArgs
	ReceiveWindow ReceiveWindowOptions
	// StpPolicy is an optional self-trade-prevention policy. When empty the
	// matching engine applies its default (cancel_maker).
	StpPolicy StpPolicy
}

// OrderSubmission is the request payload used for POST /orders when signature may be omitted.
type OrderSubmission struct {
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
	Signature     string        `json:"signature,omitempty"`
}

// CreateOrderRequest is the request envelope for POST /orders in delegated flows.
//
// StpPolicy is a top-level request field, never part of the signed Order.
type CreateOrderRequest struct {
	Order      OrderSubmission `json:"order"`
	OrderType  OrderType       `json:"orderType"`
	MarketSlug string          `json:"marketSlug"`
	OwnerID    int             `json:"ownerId"`
	OnBehalfOf *int            `json:"onBehalfOf,omitempty"`
	StpPolicy  StpPolicy       `json:"stpPolicy,omitempty"`
	PostOnly   *bool           `json:"postOnly,omitempty"`
	Timestamp  *int64          `json:"timestamp,omitempty"`
	RecvWindow *int64          `json:"recvWindow,omitempty"`
}
