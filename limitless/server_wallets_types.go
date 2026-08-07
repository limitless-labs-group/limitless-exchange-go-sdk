package limitless

import "encoding/json"

// RedeemServerWalletParams contains the parameters required for server-wallet redeem flows.
type RedeemServerWalletParams struct {
	ConditionID string
	OnBehalfOf  int
}

// SplitServerWalletParams contains the parameters required for server-wallet split flows.
// ConditionID identifies the target market. Amount must be in the collateral token's
// smallest unit. Set Venue from the market response: CLOB/simple markets require
// venue.exchange and no adapter; NegRisk markets require venue.adapter and may
// include venue.exchange. Set OnBehalfOf for partner child-profile splits; leave
// it zero only when splitting the authenticated caller's own server wallet.
type SplitServerWalletParams struct {
	ConditionID string
	Amount      string
	Venue       *ServerWalletVenue
	OnBehalfOf  int
}

// MergeServerWalletParams contains the parameters required for server-wallet merge flows.
// ConditionID identifies the target market. Amount must be in the collateral token's
// smallest unit. Set Venue from the market response: CLOB/simple markets require
// venue.exchange and no adapter; NegRisk markets require venue.adapter and may
// include venue.exchange. Set OnBehalfOf for partner child-profile merges; leave
// it zero only when merging the authenticated caller's own server wallet.
type MergeServerWalletParams struct {
	ConditionID string
	Amount      string
	Venue       *ServerWalletVenue
	OnBehalfOf  int
}

// ServerWalletVenue contains market route data from the public market response.
// Adapter routes the operation through the NegRisk adapter when present; without
// adapter, exchange is required for the CLOB/simple market contract context.
type ServerWalletVenue struct {
	Exchange string `json:"exchange,omitempty"`
	Adapter  string `json:"adapter,omitempty"`
}

// WithdrawServerWalletParams contains the parameters required for server-wallet withdraw flows.
// Amount must be provided in the token's smallest unit. Destination is optional; when
// omitted, the API defaults to the authenticated partner's smart wallet when present,
// otherwise the authenticated partner account. Explicit destinations must be the
// authenticated partner account, authenticated partner smart wallet, or an active
// withdrawal address allowlisted on the authenticated partner profile. Set
// OnBehalfOf for partner child-profile withdrawals. Leave OnBehalfOf as zero
// only when withdrawing the authenticated caller's own server wallet to an
// explicit Destination.
type WithdrawServerWalletParams struct {
	Amount      string
	OnBehalfOf  int
	Token       string
	Destination string
}

type redeemServerWalletRequest struct {
	ConditionID string `json:"conditionId"`
	OnBehalfOf  int    `json:"onBehalfOf"`
}

type splitMergeServerWalletRequest struct {
	ConditionID string             `json:"conditionId"`
	Amount      string             `json:"amount"`
	Venue       *ServerWalletVenue `json:"venue"`
	OnBehalfOf  int                `json:"onBehalfOf,omitempty"`
}

type withdrawServerWalletRequest struct {
	Amount      string  `json:"amount"`
	OnBehalfOf  int     `json:"onBehalfOf,omitempty"`
	Token       *string `json:"token,omitempty"`
	Destination *string `json:"destination,omitempty"`
}

// ServerWalletTransactionEnvelope holds common transaction metadata returned by server-wallet operations.
type ServerWalletTransactionEnvelope struct {
	Hash              string `json:"hash"`
	UserOperationHash string `json:"userOperationHash"`
	TransactionID     string `json:"transactionId"`
	WalletAddress     string `json:"walletAddress"`
}

// ServerWalletPositionOperationResponse holds the API response returned by split/merge operations.
type ServerWalletPositionOperationResponse struct {
	ServerWalletTransactionEnvelope
	ConditionID string `json:"conditionId"`
	MarketID    int    `json:"marketId"`
	Operation   string `json:"operation"`
	Route       string `json:"route"`
	raw         json.RawMessage
}

// RawJSON returns the exact response body received from the API.
func (r ServerWalletPositionOperationResponse) RawJSON() json.RawMessage {
	if len(r.raw) == 0 {
		return nil
	}
	raw := make(json.RawMessage, len(r.raw))
	copy(raw, r.raw)
	return raw
}

func (r *ServerWalletPositionOperationResponse) setRawJSON(raw json.RawMessage) {
	if len(raw) == 0 {
		r.raw = nil
		return
	}
	r.raw = make(json.RawMessage, len(raw))
	copy(r.raw, raw)
}

// RedeemServerWalletResponse is returned after POST /portfolio/redeem.
type RedeemServerWalletResponse struct {
	ServerWalletTransactionEnvelope
	ConditionID string `json:"conditionId"`
	MarketID    int    `json:"marketId"`
}

// SplitServerWalletResponse is returned after POST /portfolio/split.
type SplitServerWalletResponse struct {
	ServerWalletPositionOperationResponse
}

// MergeServerWalletResponse is returned after POST /portfolio/merge.
type MergeServerWalletResponse struct {
	ServerWalletPositionOperationResponse
}

// WithdrawServerWalletResponse is returned after POST /portfolio/withdraw.
type WithdrawServerWalletResponse struct {
	ServerWalletTransactionEnvelope
	Token       string `json:"token"`
	Destination string `json:"destination"`
	Amount      string `json:"amount"`
}
