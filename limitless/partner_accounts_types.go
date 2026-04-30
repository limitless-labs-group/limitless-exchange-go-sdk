package limitless

// CreatePartnerAccountInput is the request payload for partner account creation.
type CreatePartnerAccountInput struct {
	DisplayName        string `json:"displayName,omitempty"`
	CreateServerWallet *bool  `json:"createServerWallet,omitempty"`
}

// CreatePartnerAccountEOAHeaders contains wallet-ownership verification headers for EOA mode.
type CreatePartnerAccountEOAHeaders struct {
	Account        string
	SigningMessage string
	Signature      string
}

// PartnerAccountResponse is returned after creating a partner-owned profile.
type PartnerAccountResponse struct {
	ProfileID int    `json:"profileId"`
	Account   string `json:"account"`
}

// PartnerAccountAllowanceTargetType identifies the approval target category.
type PartnerAccountAllowanceTargetType string

// PartnerAccountAllowanceRequiredFor identifies which trading side needs an approval.
type PartnerAccountAllowanceRequiredFor string

// PartnerAccountAllowanceTargetStatus is the per-target recovery status.
type PartnerAccountAllowanceTargetStatus string

// PartnerAccountAllowanceErrorCode identifies a known allowance recovery failure reason.
type PartnerAccountAllowanceErrorCode string

const (
	PartnerAccountAllowanceTypeUSDCAllowance PartnerAccountAllowanceTargetType = "USDC_ALLOWANCE"
	PartnerAccountAllowanceTypeCTFApproval   PartnerAccountAllowanceTargetType = "CTF_APPROVAL"

	PartnerAccountAllowanceRequiredForBuy  PartnerAccountAllowanceRequiredFor = "BUY"
	PartnerAccountAllowanceRequiredForSell PartnerAccountAllowanceRequiredFor = "SELL"

	PartnerAccountAllowanceStatusConfirmed PartnerAccountAllowanceTargetStatus = "confirmed"
	PartnerAccountAllowanceStatusMissing   PartnerAccountAllowanceTargetStatus = "missing"
	PartnerAccountAllowanceStatusSubmitted PartnerAccountAllowanceTargetStatus = "submitted"
	PartnerAccountAllowanceStatusFailed    PartnerAccountAllowanceTargetStatus = "failed"

	PartnerAccountAllowanceErrorPrivySponsorshipUnavailable PartnerAccountAllowanceErrorCode = "PRIVY_SPONSORSHIP_UNAVAILABLE"
	PartnerAccountAllowanceErrorPrivySubmissionFailed       PartnerAccountAllowanceErrorCode = "PRIVY_SUBMISSION_FAILED"
	PartnerAccountAllowanceErrorRPCReadFailed               PartnerAccountAllowanceErrorCode = "RPC_READ_FAILED"
	PartnerAccountAllowanceErrorRequestBudgetExceeded       PartnerAccountAllowanceErrorCode = "REQUEST_BUDGET_EXCEEDED"
)

// PartnerAccountAllowanceSummary summarizes server-wallet approval readiness.
type PartnerAccountAllowanceSummary struct {
	Total     int `json:"total"`
	Confirmed int `json:"confirmed"`
	Missing   int `json:"missing"`
	Submitted int `json:"submitted"`
	Failed    int `json:"failed"`
}

// PartnerAccountAllowanceTarget describes one delegated-trading approval target.
type PartnerAccountAllowanceTarget struct {
	Type              PartnerAccountAllowanceTargetType   `json:"type"`
	TokenAddress      string                              `json:"tokenAddress"`
	SpenderOrOperator string                              `json:"spenderOrOperator"`
	Label             string                              `json:"label"`
	RequiredFor       PartnerAccountAllowanceRequiredFor  `json:"requiredFor"`
	Confirmed         bool                                `json:"confirmed"`
	Status            PartnerAccountAllowanceTargetStatus `json:"status"`
	TransactionID     *string                             `json:"transactionId,omitempty"`
	TxHash            *string                             `json:"txHash,omitempty"`
	UserOperationHash *string                             `json:"userOperationHash,omitempty"`
	Retryable         bool                                `json:"retryable"`
	ErrorCode         *PartnerAccountAllowanceErrorCode   `json:"errorCode,omitempty"`
	ErrorMessage      *string                             `json:"errorMessage,omitempty"`
}

// PartnerAccountAllowanceResponse is returned by partner allowance check and retry endpoints.
type PartnerAccountAllowanceResponse struct {
	ProfileID        int                             `json:"profileId"`
	PartnerProfileID int                             `json:"partnerProfileId"`
	ChainID          int                             `json:"chainId"`
	WalletAddress    string                          `json:"walletAddress"`
	Ready            bool                            `json:"ready"`
	Summary          PartnerAccountAllowanceSummary  `json:"summary"`
	Targets          []PartnerAccountAllowanceTarget `json:"targets"`
}
