package limitless

import "encoding/json"

// UserRank contains rank information for a user.
type UserRank struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	FeeRateBps int    `json:"feeRateBps"`
}

// ReferralData contains referral information.
type ReferralData struct {
	CreatedAt         string  `json:"createdAt"`
	ID                int     `json:"id"`
	ReferredProfileID int     `json:"referredProfileId"`
	PfpURL            *string `json:"pfpUrl"`
	DisplayName       string  `json:"displayName"`
}

// UserProfile represents a user profile from the API (1:1 with API response).
type UserProfile struct {
	// Core fields
	ID      int       `json:"id"`
	Account string    `json:"account"`
	Rank    *UserRank `json:"rank,omitempty"`

	// Profile information
	CreatedAt         *string `json:"createdAt,omitempty"`
	Username          *string `json:"username,omitempty"`
	DisplayName       *string `json:"displayName,omitempty"`
	PfpURL            *string `json:"pfpUrl,omitempty"`
	Bio               *string `json:"bio,omitempty"`
	SocialURL         *string `json:"socialUrl,omitempty"`
	TradeWalletOption *string `json:"tradeWalletOption,omitempty"`
	EmbeddedAccount   *string `json:"embeddedAccount,omitempty"`

	// Points and gamification
	Points                  *float64 `json:"points,omitempty"`
	AccumulativePoints      *float64 `json:"accumulativePoints,omitempty"`
	EnrolledInPointsProgram *bool    `json:"enrolledInPointsProgram,omitempty"`
	LeaderboardPosition     *int     `json:"leaderboardPosition,omitempty"`
	IsTop100                *bool    `json:"isTop100,omitempty"`
	IsCaptain               *bool    `json:"isCaptain,omitempty"`

	// Referral data
	ReferralData       []ReferralData `json:"referralData,omitempty"`
	ReferredUsersCount *int           `json:"referredUsersCount,omitempty"`
}

// PositionMarketGroup contains group information for a position's market.
type PositionMarketGroup struct {
	Slug  *string `json:"slug,omitempty"`
	Title *string `json:"title,omitempty"`
}

// PositionMarket contains market information for a position.
type PositionMarket struct {
	ID                  json.RawMessage      `json:"id"` // Can be int or string
	Slug                string               `json:"slug"`
	Title               string               `json:"title"`
	Status              *string              `json:"status,omitempty"`
	Closed              bool                 `json:"closed"`
	Deadline            string               `json:"deadline"`
	ConditionID         *string              `json:"conditionId,omitempty"`
	WinningOutcomeIndex *int                 `json:"winningOutcomeIndex,omitempty"`
	Group               *PositionMarketGroup `json:"group,omitempty"`
}

// PositionSide contains position details for one side (YES or NO).
type PositionSide struct {
	Cost          string `json:"cost"`
	FillPrice     string `json:"fillPrice"`
	MarketValue   string `json:"marketValue"`
	RealisedPnl   string `json:"realisedPnl"`
	UnrealizedPnl string `json:"unrealizedPnl"`
}

// TokenBalance contains YES and NO token balances.
type TokenBalance struct {
	Yes string `json:"yes"`
	No  string `json:"no"`
}

// LatestTrade contains latest trade price information.
type LatestTrade struct {
	LatestYesPrice    float64 `json:"latestYesPrice"`
	LatestNoPrice     float64 `json:"latestNoPrice"`
	OutcomeTokenPrice float64 `json:"outcomeTokenPrice"`
}

// CLOBPositionOrders contains active orders information.
type CLOBPositionOrders struct {
	LiveOrders            []json.RawMessage `json:"liveOrders"`
	TotalCollateralLocked string            `json:"totalCollateralLocked"`
}

// CLOBPositionRewards contains rewards information.
type CLOBPositionRewards struct {
	Epochs    []json.RawMessage `json:"epochs"`
	IsEarning bool              `json:"isEarning"`
}

// CLOBPositionSides contains YES and NO position sides.
type CLOBPositionSides struct {
	Yes PositionSide `json:"yes"`
	No  PositionSide `json:"no"`
}

// CLOBPosition represents a CLOB (Central Limit Order Book) position.
type CLOBPosition struct {
	Market        PositionMarket       `json:"market"`
	MakerAddress  string               `json:"makerAddress"`
	Positions     CLOBPositionSides    `json:"positions"`
	TokensBalance TokenBalance         `json:"tokensBalance"`
	LatestTrade   LatestTrade          `json:"latestTrade"`
	Orders        *CLOBPositionOrders  `json:"orders,omitempty"`
	Rewards       *CLOBPositionRewards `json:"rewards,omitempty"`
}

// AMMLatestTrade contains latest trade price for AMM positions.
type AMMLatestTrade struct {
	OutcomeTokenPrice string `json:"outcomeTokenPrice"`
}

// AMMPosition represents an AMM (Automated Market Maker) position.
type AMMPosition struct {
	Market             PositionMarket  `json:"market"`
	Account            string          `json:"account"`
	OutcomeIndex       int             `json:"outcomeIndex"`
	CollateralAmount   string          `json:"collateralAmount"`
	OutcomeTokenAmount string          `json:"outcomeTokenAmount"`
	AverageFillPrice   string          `json:"averageFillPrice"`
	TotalBuysCost      string          `json:"totalBuysCost"`
	TotalSellsCost     string          `json:"totalSellsCost"`
	RealizedPnl        string          `json:"realizedPnl"`
	UnrealizedPnl      string          `json:"unrealizedPnl"`
	LatestTrade        *AMMLatestTrade `json:"latestTrade,omitempty"`
}

// PortfolioRewards contains rewards information in portfolio response.
type PortfolioRewards struct {
	TodaysRewards             string            `json:"todaysRewards"`
	RewardsByEpoch            []json.RawMessage `json:"rewardsByEpoch"`
	RewardsChartData          []json.RawMessage `json:"rewardsChartData"`
	TotalUnpaidRewards        string            `json:"totalUnpaidRewards"`
	TotalUserRewardsLastEpoch string            `json:"totalUserRewardsLastEpoch"`
}

// PortfolioPositionsResponse is the API response for /portfolio/positions.
type PortfolioPositionsResponse struct {
	AMM                []AMMPosition     `json:"amm"`
	CLOB               []CLOBPosition    `json:"clob"`
	Group              []json.RawMessage `json:"group"`
	Points             *string           `json:"points,omitempty"`
	AccumulativePoints *string           `json:"accumulativePoints,omitempty"`
	Rewards            *PortfolioRewards `json:"rewards,omitempty"`
}

// Position is a simplified unified position view covering both CLOB and AMM.
type Position struct {
	Type          string         `json:"type"` // "CLOB" or "AMM"
	Market        PositionMarket `json:"market"`
	Side          string         `json:"side"` // "YES" or "NO"
	CostBasis     float64        `json:"costBasis"`
	MarketValue   float64        `json:"marketValue"`
	UnrealizedPnl float64        `json:"unrealizedPnl"`
	RealizedPnl   float64        `json:"realizedPnl"`
	CurrentPrice  float64        `json:"currentPrice"`
	AvgPrice      float64        `json:"avgPrice"`
	TokenBalance  float64        `json:"tokenBalance"`
}

// PortfolioBreakdownEntry contains breakdown data for a position type.
type PortfolioBreakdownEntry struct {
	Positions int     `json:"positions"`
	Value     float64 `json:"value"`
	Pnl       float64 `json:"pnl"`
}

// PortfolioBreakdown contains CLOB and AMM position breakdowns.
type PortfolioBreakdown struct {
	CLOB PortfolioBreakdownEntry `json:"clob"`
	AMM  PortfolioBreakdownEntry `json:"amm"`
}

// PortfolioSummary provides portfolio statistics.
type PortfolioSummary struct {
	TotalValue            float64            `json:"totalValue"`
	TotalCostBasis        float64            `json:"totalCostBasis"`
	TotalUnrealizedPnl    float64            `json:"totalUnrealizedPnl"`
	TotalRealizedPnl      float64            `json:"totalRealizedPnl"`
	TotalUnrealizedPnlPct float64            `json:"totalUnrealizedPnlPercent"`
	PositionCount         int                `json:"positionCount"`
	MarketCount           int                `json:"marketCount"`
	Breakdown             PortfolioBreakdown `json:"breakdown"`
}

// HistoryMarketCollateral is the collateral token info inside a history market.
type HistoryMarketCollateral struct {
	Symbol   string `json:"symbol"`
	ID       string `json:"id"`
	Decimals int    `json:"decimals"`
}

// HistoryMarket is the market snapshot embedded in a history entry.
type HistoryMarket struct {
	Closed         bool                     `json:"closed"`
	Collateral     *HistoryMarketCollateral `json:"collateral,omitempty"`
	Group          interface{}              `json:"group,omitempty"`
	ConditionID    string                   `json:"conditionId,omitempty"`
	Funding        string                   `json:"funding,omitempty"`
	ID             string                   `json:"id"`
	Slug           string                   `json:"slug"`
	Title          string                   `json:"title"`
	ExpirationDate string                   `json:"expirationDate,omitempty"`
}

// HistoryEntry represents a user history entry (cursor-based).
type HistoryEntry struct {
	BlockTimestamp      int64          `json:"blockTimestamp"`
	CollateralAmount    string         `json:"collateralAmount,omitempty"`
	Market              *HistoryMarket `json:"market,omitempty"`
	OutcomeIndex        *int           `json:"outcomeIndex,omitempty"`
	OutcomeTokenAmount  string         `json:"outcomeTokenAmount,omitempty"`
	OutcomeTokenAmounts []string       `json:"outcomeTokenAmounts,omitempty"`
	OutcomeTokenPrice   interface{}    `json:"outcomeTokenPrice,omitempty"`
	Strategy            string         `json:"strategy,omitempty"`
	TransactionHash     string         `json:"transactionHash,omitempty"`
}

// HistoryResponse is the cursor-based response for /portfolio/history.
type HistoryResponse struct {
	Data       []HistoryEntry `json:"data"`
	NextCursor *string        `json:"nextCursor"`
}
