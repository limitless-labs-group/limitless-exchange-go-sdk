package limitless

// PackLeg describes a catalogue, quote, or position leg. Amounts remain decimal strings.
type PackLeg struct {
	ID               string   `json:"id,omitempty"`
	LegIndex         int      `json:"legIndex,omitempty"`
	PriceAtCreation  string   `json:"priceAtCreation,omitempty"`
	ResolutionStatus string   `json:"resolutionStatus,omitempty"`
	ResolvedAt       *string  `json:"resolvedAt,omitempty"`
	MarketSlug       string   `json:"marketSlug"`
	MarketID         int      `json:"marketId"`
	ConditionID      string   `json:"conditionId"`
	MarketType       string   `json:"marketType"`
	Outcome          string   `json:"outcome"`
	Price            *float64 `json:"price,omitempty"`
	BookPrice        *float64 `json:"bookPrice,omitempty"`
	LivePrice        *float64 `json:"livePrice,omitempty"`
}

type Pack struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Eligible   bool      `json:"eligible"`
	ParlayType string    `json:"parlayType"`
	Legs       []PackLeg `json:"legs"`
}

type PackQuoteParams struct {
	PackID      string `json:"packId"`
	StakeAmount string `json:"stakeAmount"`
}

type PackQuote struct {
	Eligible           bool      `json:"eligible"`
	ParlayType         string    `json:"parlayType"`
	RawMultiplier      float64   `json:"rawMultiplier"`
	Multiplier         float64   `json:"multiplier"`
	PotentialPayout    string    `json:"potentialPayout"`
	CollateralRequired string    `json:"collateralRequired"`
	VigPercent         float64   `json:"vigPercent"`
	Legs               []PackLeg `json:"legs"`
	IneligibleReasons  []string  `json:"ineligibleReasons"`
}

// PackBuyParams must be durably saved before Buy. Replay every field unchanged,
// including an expired deadline. Never generate a new key after an ambiguous result.
type PackBuyParams struct {
	OnBehalfOf      int    `json:"onBehalfOf"`
	PackID          string `json:"packId"`
	StakeAmount     string `json:"stakeAmount"`
	MinPayoutAmount string `json:"minPayoutAmount"`
	Deadline        string `json:"deadline"`
	IdempotencyKey  string `json:"idempotencyKey"`
}

type PackSource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PackPosition struct {
	ID                  string      `json:"id"`
	Status              string      `json:"status"` // PENDING, ACTIVE, WON, LOST, FAILED, VAULT_REVERTED
	StakeAmount         string      `json:"stakeAmount"`
	CollateralDeposited string      `json:"collateralDeposited"`
	MultiplierOffered   string      `json:"multiplierOffered"`
	VigPercent          string      `json:"vigPercent"`
	TxHash              *string     `json:"txHash"`
	ClaimTxHash         *string     `json:"claimTxHash"`
	Wallet              string      `json:"wallet"`
	CreatedAt           string      `json:"createdAt"`
	Legs                []PackLeg   `json:"legs"`
	SourcePack          *PackSource `json:"sourcePack"`
	PayoutAmount        *string     `json:"payoutAmount"`
}

type PackBuyResponse struct {
	Status   string       `json:"status"` // COMPLETED, PENDING, RECOVERING, FAILED
	Position PackPosition `json:"position"`
}

type PackPositions struct {
	Data   []PackPosition `json:"data"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type PackClaimResponse struct {
	ParlayID          string  `json:"parlayId"`
	Status            string  `json:"status"` // APPROVAL_PENDING, CLAIM_PENDING, CLAIMED
	TxHash            *string `json:"txHash,omitempty"`
	TransactionID     *string `json:"transactionId,omitempty"`
	UserOperationHash *string `json:"userOperationHash,omitempty"`
}
