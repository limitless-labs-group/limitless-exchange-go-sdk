package limitless

import "time"

// AMMAllowanceSide identifies which market approval is being checked or submitted.
type AMMAllowanceSide string

const (
	// AMMAllowanceSideBuy checks or approves collateral spending for the market FPMM.
	AMMAllowanceSideBuy AMMAllowanceSide = "BUY"
	// AMMAllowanceSideSell checks or approves Conditional Tokens operator access for the market FPMM.
	AMMAllowanceSideSell AMMAllowanceSide = "SELL"
)

// AMMAllowanceStatus identifies the current state of an AMM approval.
type AMMAllowanceStatus string

const (
	AMMAllowanceStatusMissing   AMMAllowanceStatus = "missing"
	AMMAllowanceStatusSubmitted AMMAllowanceStatus = "submitted"
	AMMAllowanceStatusConfirmed AMMAllowanceStatus = "confirmed"
)

// AMMOutcomeIndex identifies a binary AMM outcome.
type AMMOutcomeIndex int

const (
	AMMOutcomeYes AMMOutcomeIndex = 0
	AMMOutcomeNo  AMMOutcomeIndex = 1
)

// AMMTradeStatus identifies the submission state returned by an AMM trade.
type AMMTradeStatus string

const (
	AMMTradeStatusSubmitted AMMTradeStatus = "SUBMITTED"
)

// AMMAllowanceParams selects the wallet, market, and side for an allowance operation.
// Market may be a market slug or a checksummed FPMM address. Leave OnBehalfOf zero
// only when the authenticated profile directly owns the server wallet.
type AMMAllowanceParams struct {
	Market     string
	Side       AMMAllowanceSide
	OnBehalfOf int
}

// AMMAllowancePollOptions configures EnsureAllowance polling. A zero interval
// uses the SDK default. The caller's context controls the overall deadline.
type AMMAllowancePollOptions struct {
	Interval time.Duration
}

// AMMBuyParams contains an exact-collateral AMM buy request. CollateralAmount
// must be a positive integer string in the collateral token's base units.
// SlippageBps nil uses the API default; a pointer to zero explicitly requests
// zero slippage. IdempotencyKey is required and retained by the API for 24 hours.
type AMMBuyParams struct {
	Market           string
	OutcomeIndex     AMMOutcomeIndex
	CollateralAmount string
	SlippageBps      *int
	IdempotencyKey   string
	OnBehalfOf       int
}

// AMMSellParams contains an exact-collateral-return AMM sell request.
// CollateralReturnAmount must be a positive integer string in the collateral
// token's base units. SlippageBps nil uses the API default; a pointer to zero
// explicitly requests zero slippage. IdempotencyKey is required and retained
// by the API for 24 hours.
type AMMSellParams struct {
	Market                 string
	OutcomeIndex           AMMOutcomeIndex
	CollateralReturnAmount string
	SlippageBps            *int
	IdempotencyKey         string
	OnBehalfOf             int
}

type ammAllowanceRequest struct {
	Market     string           `json:"market"`
	Side       AMMAllowanceSide `json:"side"`
	OnBehalfOf int              `json:"onBehalfOf,omitempty"`
}

type ammBuyRequest struct {
	Market           string          `json:"market"`
	OutcomeIndex     AMMOutcomeIndex `json:"outcomeIndex"`
	CollateralAmount string          `json:"collateralAmount"`
	SlippageBps      *int            `json:"slippageBps,omitempty"`
	IdempotencyKey   string          `json:"idempotencyKey"`
	OnBehalfOf       int             `json:"onBehalfOf,omitempty"`
}

type ammSellRequest struct {
	Market                 string          `json:"market"`
	OutcomeIndex           AMMOutcomeIndex `json:"outcomeIndex"`
	CollateralReturnAmount string          `json:"collateralReturnAmount"`
	SlippageBps            *int            `json:"slippageBps,omitempty"`
	IdempotencyKey         string          `json:"idempotencyKey"`
	OnBehalfOf             int             `json:"onBehalfOf,omitempty"`
}

// AMMTransactionIdentifiers contains independently optional transaction
// identifiers returned by sponsored server-wallet operations.
type AMMTransactionIdentifiers struct {
	TransactionID     *string `json:"transactionId,omitempty"`
	UserOperationHash *string `json:"userOperationHash,omitempty"`
	TxHash            *string `json:"txHash,omitempty"`
}

// AMMAllowanceResponse is returned by allowance check and approve operations.
// CurrentAllowance is present for BUY checks and omitted for SELL checks.
type AMMAllowanceResponse struct {
	AMMTransactionIdentifiers
	Status            AMMAllowanceStatus `json:"status"`
	Confirmed         bool               `json:"confirmed"`
	Market            string             `json:"market"`
	MarketAddress     string             `json:"marketAddress"`
	Side              AMMAllowanceSide   `json:"side"`
	WalletAddress     string             `json:"walletAddress"`
	TokenAddress      string             `json:"tokenAddress"`
	SpenderOrOperator string             `json:"spenderOrOperator"`
	CurrentAllowance  *string            `json:"currentAllowance,omitempty"`
}

// AMMBuyResponse is returned after an AMM buy has been submitted.
type AMMBuyResponse struct {
	AMMTransactionIdentifiers
	Status           AMMTradeStatus  `json:"status"`
	Market           string          `json:"market"`
	OutcomeIndex     AMMOutcomeIndex `json:"outcomeIndex"`
	CollateralAmount string          `json:"collateralAmount"`
	ExpectedShares   string          `json:"expectedShares"`
	MinShares        string          `json:"minShares"`
}

// AMMSellResponse is returned after an AMM sell has been submitted.
type AMMSellResponse struct {
	AMMTransactionIdentifiers
	Status                 AMMTradeStatus  `json:"status"`
	Market                 string          `json:"market"`
	OutcomeIndex           AMMOutcomeIndex `json:"outcomeIndex"`
	CollateralReturnAmount string          `json:"collateralReturnAmount"`
	ExpectedShares         string          `json:"expectedShares"`
	MaxShares              string          `json:"maxShares"`
}
