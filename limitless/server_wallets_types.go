package limitless

// RedeemServerWalletParams contains the parameters required for server-wallet redeem flows.
type RedeemServerWalletParams struct {
	ConditionID string
	OnBehalfOf  int
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

// RedeemServerWalletResponse is returned after POST /portfolio/redeem.
type RedeemServerWalletResponse struct {
	ServerWalletTransactionEnvelope
	ConditionID string `json:"conditionId"`
	MarketID    int    `json:"marketId"`
}

// WithdrawServerWalletResponse is returned after POST /portfolio/withdraw.
type WithdrawServerWalletResponse struct {
	ServerWalletTransactionEnvelope
	Token       string `json:"token"`
	Destination string `json:"destination"`
	Amount      string `json:"amount"`
}
