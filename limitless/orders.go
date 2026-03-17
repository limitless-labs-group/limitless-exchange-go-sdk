package limitless

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
)

// OrderClient manages order creation, signing, and submission.
type OrderClient struct {
	client        *HttpClient
	signer        *OrderSigner
	signingConfig OrderSigningConfig
	marketFetcher *MarketFetcher
	logger        Logger

	userData *UserData
	builder  *OrderBuilder
	mu       sync.Mutex
}

// OrderClientOption configures an OrderClient.
type OrderClientOption func(*OrderClient)

// WithSigningConfig sets a custom signing configuration.
func WithSigningConfig(config OrderSigningConfig) OrderClientOption {
	return func(oc *OrderClient) {
		oc.signingConfig = config
	}
}

// WithOrderMarketFetcher sets a shared MarketFetcher for venue caching.
func WithOrderMarketFetcher(f *MarketFetcher) OrderClientOption {
	return func(oc *OrderClient) {
		oc.marketFetcher = f
	}
}

// WithOrderLogger sets the logger for the OrderClient.
func WithOrderLogger(l Logger) OrderClientOption {
	return func(oc *OrderClient) {
		oc.logger = l
	}
}

// NewOrderClient creates a new order client.
// privateKeyHex is the hex-encoded private key (with or without "0x" prefix).
func NewOrderClient(client *HttpClient, privateKeyHex string, opts ...OrderClientOption) (*OrderClient, error) {
	signer, err := NewOrderSigner(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to create order signer: %w", err)
	}

	chainID := DefaultChainID
	if env := os.Getenv("CHAIN_ID"); env != "" {
		if v, err := strconv.Atoi(env); err == nil {
			chainID = v
		}
	}

	oc := &OrderClient{
		client: client,
		signer: signer,
		signingConfig: OrderSigningConfig{
			ChainID:         chainID,
			ContractAddress: ZeroAddress,
		},
		logger: NewNoOpLogger(),
	}

	for _, opt := range opts {
		opt(oc)
	}

	// Set signer logger
	oc.signer.logger = oc.logger

	// Create market fetcher if not provided
	if oc.marketFetcher == nil {
		oc.marketFetcher = NewMarketFetcher(client, WithMarketLogger(oc.logger))
	}

	return oc, nil
}

// ensureUserData lazily fetches and caches user data from the profile API.
func (oc *OrderClient) ensureUserData(ctx context.Context) (*UserData, error) {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	if oc.userData != nil {
		return oc.userData, nil
	}

	oc.logger.Info("Fetching user profile for order client initialization...", map[string]any{
		"walletAddress": oc.signer.Address().Hex(),
	})

	pf := NewPortfolioFetcher(oc.client, WithPortfolioLogger(oc.logger))

	var profile UserProfile
	if err := pf.getProfile(ctx, oc.signer.Address().Hex(), &profile); err != nil {
		return nil, fmt.Errorf("failed to fetch user profile: %w", err)
	}

	feeRateBps := 300 // default
	if profile.Rank != nil {
		feeRateBps = profile.Rank.FeeRateBps
	}

	oc.userData = &UserData{
		UserID:     profile.ID,
		FeeRateBps: feeRateBps,
	}

	oc.builder = NewOrderBuilder(oc.signer.Address().Hex(), feeRateBps)

	oc.logger.Info("Order Client initialized", map[string]any{
		"walletAddress": profile.Account,
		"userId":        oc.userData.UserID,
		"feeRate":       fmt.Sprintf("%.2f%%", float64(feeRateBps)/100),
	})

	return oc.userData, nil
}

// CreateOrder creates, signs, and submits a new order.
func (oc *OrderClient) CreateOrder(ctx context.Context, params CreateOrderParams) (*OrderResponse, error) {
	userData, err := oc.ensureUserData(ctx)
	if err != nil {
		return nil, err
	}

	oc.logger.Info("Creating order", map[string]any{
		"orderType":  params.OrderType,
		"marketSlug": params.MarketSlug,
	})

	// Resolve venue
	venue, ok := oc.marketFetcher.GetVenue(params.MarketSlug)
	if !ok {
		oc.logger.Warn("Venue not cached, fetching market details. "+
			"For better performance, call marketFetcher.GetMarket() before CreateOrder().",
			map[string]any{"marketSlug": params.MarketSlug})

		market, err := oc.marketFetcher.GetMarket(ctx, params.MarketSlug)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch market: %w", err)
		}
		if market.Venue == nil {
			return nil, fmt.Errorf("market %s does not have venue information", params.MarketSlug)
		}
		venue = *market.Venue
	}

	signingConfig := OrderSigningConfig{
		ChainID:         oc.signingConfig.ChainID,
		ContractAddress: venue.Exchange,
	}

	// Build unsigned order
	unsignedOrder, err := oc.builder.BuildOrder(params.Args)
	if err != nil {
		return nil, fmt.Errorf("failed to build order: %w", err)
	}

	// Sign the order
	signature, err := oc.signer.SignOrder(unsignedOrder, signingConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to sign order: %w", err)
	}

	// Prepare payload
	payload := NewOrderPayload{
		Order: SignedOrder{
			Salt:          unsignedOrder.Salt,
			Maker:         unsignedOrder.Maker,
			Signer:        unsignedOrder.Signer,
			Taker:         unsignedOrder.Taker,
			TokenID:       unsignedOrder.TokenID,
			MakerAmount:   unsignedOrder.MakerAmount,
			TakerAmount:   unsignedOrder.TakerAmount,
			Expiration:    unsignedOrder.Expiration,
			Nonce:         unsignedOrder.Nonce,
			FeeRateBps:    unsignedOrder.FeeRateBps,
			Side:          unsignedOrder.Side,
			SignatureType: unsignedOrder.SignatureType,
			Price:         unsignedOrder.Price,
			Signature:     signature,
		},
		OrderType:  params.OrderType,
		MarketSlug: params.MarketSlug,
		OwnerID:    userData.UserID,
	}

	// Submit to API
	var resp OrderResponse
	if err := oc.client.Post(ctx, "/orders", payload, &resp); err != nil {
		return nil, err
	}

	oc.logger.Info("Order created successfully", map[string]any{
		"orderId": resp.Order.ID,
	})

	return &resp, nil
}

// CancelResponse is the response from a cancel operation.
type CancelResponse struct {
	Message string `json:"message"`
}

// Cancel cancels an order by ID and returns the API message.
func (oc *OrderClient) Cancel(ctx context.Context, orderID string) (string, error) {
	oc.logger.Info("Cancelling order", map[string]any{"orderId": orderID})

	var resp CancelResponse
	if err := oc.client.Delete(ctx, "/orders/"+orderID, &resp); err != nil {
		return "", err
	}
	return resp.Message, nil
}

// CancelAll cancels all orders for a market and returns the API message.
func (oc *OrderClient) CancelAll(ctx context.Context, marketSlug string) (string, error) {
	oc.logger.Info("Cancelling all orders for market", map[string]any{"marketSlug": marketSlug})

	var resp CancelResponse
	if err := oc.client.Delete(ctx, "/orders/all/"+marketSlug, &resp); err != nil {
		return "", err
	}
	return resp.Message, nil
}

// BuildUnsignedOrder builds an unsigned order without signing or submitting.
func (oc *OrderClient) BuildUnsignedOrder(ctx context.Context, args OrderArgs) (*UnsignedOrder, error) {
	if _, err := oc.ensureUserData(ctx); err != nil {
		return nil, err
	}
	return oc.builder.BuildOrder(args)
}

// SignOrder signs an unsigned order without submitting.
func (oc *OrderClient) SignOrder(order *UnsignedOrder) (string, error) {
	return oc.signer.SignOrder(order, oc.signingConfig)
}

// WalletAddress returns the wallet address.
func (oc *OrderClient) WalletAddress() string {
	return oc.signer.Address().Hex()
}

// OwnerID returns the user ID from the profile, or nil if not yet loaded.
func (oc *OrderClient) OwnerID() *int {
	oc.mu.Lock()
	defer oc.mu.Unlock()
	if oc.userData == nil {
		return nil
	}
	id := oc.userData.UserID
	return &id
}
