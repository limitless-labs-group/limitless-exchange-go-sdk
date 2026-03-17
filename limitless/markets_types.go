package limitless

// CollateralToken contains token contract information.
type CollateralToken struct {
	Address  string `json:"address"`
	Decimals int    `json:"decimals"`
	Symbol   string `json:"symbol"`
}

// MarketCreator contains creator information.
type MarketCreator struct {
	Name     string  `json:"name"`
	ImageURI *string `json:"imageURI,omitempty"`
	Link     *string `json:"link,omitempty"`
}

// MarketMetadata contains market metadata flags.
type MarketMetadata struct {
	Fee              bool    `json:"fee"`
	IsBannered       *bool   `json:"isBannered,omitempty"`
	IsPolyArbitrage  *bool   `json:"isPolyArbitrage,omitempty"`
	ShouldMarketMake *bool   `json:"shouldMarketMake,omitempty"`
	OpenPrice        *string `json:"openPrice,omitempty"`
}

// MarketSettings contains CLOB market settings.
type MarketSettings struct {
	MinSize      string  `json:"minSize"`
	MaxSpread    float64 `json:"maxSpread"`
	DailyReward  string  `json:"dailyReward"`
	RewardsEpoch any     `json:"rewardsEpoch"`
	C            any     `json:"c"`
	RebateRate   *float64 `json:"rebateRate,omitempty"`
}

// TradePrices contains buy/sell price data.
type TradePrices struct {
	Buy struct {
		Market [2]float64 `json:"market"`
		Limit  [2]float64 `json:"limit"`
	} `json:"buy"`
	Sell struct {
		Market [2]float64 `json:"market"`
		Limit  [2]float64 `json:"limit"`
	} `json:"sell"`
}

// PriceOracleMetadata contains oracle price feed information.
type PriceOracleMetadata struct {
	Ticker      string `json:"ticker"`
	AssetType   string `json:"assetType"`
	PythAddress string `json:"pythAddress"`
	Symbol      string `json:"symbol"`
	Name        string `json:"name"`
	Logo        string `json:"logo"`
}

// MarketOutcome represents a market outcome (legacy field).
type MarketOutcome struct {
	ID      int      `json:"id"`
	Title   string   `json:"title"`
	TokenID string   `json:"tokenId"`
	Price   *float64 `json:"price,omitempty"`
}

// Venue contains exchange and adapter contract addresses for CLOB markets.
type Venue struct {
	Exchange string  `json:"exchange"`
	Adapter  *string `json:"adapter"`
}

// MarketTokens contains YES/NO token IDs.
type MarketTokens struct {
	Yes string `json:"yes"`
	No  string `json:"no"`
}

// OrderBookEntry represents a single bid or ask.
type OrderBookEntry struct {
	Price float64 `json:"price"`
	Size  float64 `json:"size"`
	Side  string  `json:"side"`
}

// OrderBook represents the full orderbook for a market.
type OrderBook struct {
	Bids             []OrderBookEntry `json:"bids"`
	Asks             []OrderBookEntry `json:"asks"`
	TokenID          string           `json:"tokenId"`
	AdjustedMidpoint float64          `json:"adjustedMidpoint"`
	MaxSpread        string           `json:"maxSpread"`
	MinSize          string           `json:"minSize"`
	LastTradePrice   float64          `json:"lastTradePrice"`
}

// Market represents complete market information (1:1 with API response).
type Market struct {
	// Common fields
	ID                  int              `json:"id"`
	Slug                string           `json:"slug"`
	Title               string           `json:"title"`
	ProxyTitle          *string          `json:"proxyTitle"`
	Description         string           `json:"description,omitempty"`
	CollateralToken     CollateralToken  `json:"collateralToken"`
	ExpirationDate      string           `json:"expirationDate"`
	ExpirationTimestamp int64            `json:"expirationTimestamp"`
	Expired             *bool            `json:"expired,omitempty"`
	CreatedAt           string           `json:"createdAt"`
	UpdatedAt           string           `json:"updatedAt"`
	Categories          []string         `json:"categories"`
	Status              string           `json:"status"`
	Creator             MarketCreator    `json:"creator"`
	Tags                []string         `json:"tags"`
	TradeType           string           `json:"tradeType"`
	MarketType          string           `json:"marketType"`
	PriorityIndex       int              `json:"priorityIndex"`
	Metadata            MarketMetadata   `json:"metadata"`
	Volume              *string          `json:"volume,omitempty"`
	VolumeFormatted     *string          `json:"volumeFormatted,omitempty"`
	AutomationType      *string          `json:"automationType,omitempty"`
	ImageURL            *string          `json:"imageUrl,omitempty"`
	Trends              map[string]any   `json:"trends,omitempty"`
	OpenInterest        *string          `json:"openInterest,omitempty"`
	OpenInterestFmt     *string          `json:"openInterestFormatted,omitempty"`
	Liquidity           *string          `json:"liquidity,omitempty"`
	LiquidityFormatted  *string          `json:"liquidityFormatted,omitempty"`
	PositionIDs         []string         `json:"positionIds,omitempty"`

	// CLOB single market fields
	ConditionID        *string              `json:"conditionId,omitempty"`
	NegRiskRequestID   *string              `json:"negRiskRequestId,omitempty"`
	Tokens             *MarketTokens        `json:"tokens,omitempty"`
	Prices             []float64            `json:"prices,omitempty"`
	TradePrices        *TradePrices         `json:"tradePrices,omitempty"`
	IsRewardable       *bool                `json:"isRewardable,omitempty"`
	Settings           *MarketSettings      `json:"settings,omitempty"`
	Venue              *Venue               `json:"venue,omitempty"`
	Logo               *string              `json:"logo,omitempty"`
	PriceOracleData    *PriceOracleMetadata `json:"priceOracleMetadata,omitempty"`
	OrderInGroup       *int                 `json:"orderInGroup,omitempty"`
	WinningOutcomeIdx  *int                 `json:"winningOutcomeIndex,omitempty"`

	// NegRisk group market fields
	OutcomeTokens   []string  `json:"outcomeTokens,omitempty"`
	OgImageURI      *string   `json:"ogImageURI,omitempty"`
	NegRiskMarketID *string   `json:"negRiskMarketId,omitempty"`
	Markets         []Market  `json:"markets,omitempty"`
	DailyReward     *string   `json:"dailyReward,omitempty"`

	// Legacy fields
	Address        *string         `json:"address,omitempty"`
	Type           *string         `json:"type,omitempty"`
	Outcomes       []MarketOutcome `json:"outcomes,omitempty"`
	ResolutionDate *string         `json:"resolutionDate,omitempty"`
}

// ActiveMarketsSortBy constrains sort values for active markets.
// Valid values: "lp_rewards", "ending_soon", "newest", "high_value", "liquidity"
type ActiveMarketsSortBy string

const (
	SortByLPRewards  ActiveMarketsSortBy = "lp_rewards"
	SortByEndingSoon ActiveMarketsSortBy = "ending_soon"
	SortByNewest     ActiveMarketsSortBy = "newest"
	SortByHighValue  ActiveMarketsSortBy = "high_value"
	SortByLiquidity  ActiveMarketsSortBy = "liquidity"
)

// ActiveMarketsParams are query parameters for the active markets endpoint.
type ActiveMarketsParams struct {
	Limit  int                 `url:"limit,omitempty"`
	Page   int                 `url:"page,omitempty"`
	SortBy ActiveMarketsSortBy `url:"sortBy,omitempty"`
}

// ActiveMarketsResponse is the response from the active markets endpoint.
type ActiveMarketsResponse struct {
	Data              []Market `json:"data"`
	TotalMarketsCount int      `json:"totalMarketsCount"`
}

// MarketsResponse is a generic markets list response.
type MarketsResponse struct {
	Markets []Market `json:"markets"`
	Total   *int     `json:"total,omitempty"`
	Offset  *int     `json:"offset,omitempty"`
	Limit   *int     `json:"limit,omitempty"`
}
