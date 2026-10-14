package limitless

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

const defaultDelegatedFeeRateBps = 300

// DelegatedOrderService creates orders that the API signs on behalf of a target profile.
type DelegatedOrderService struct {
	client        *HttpClient
	logger        Logger
	marketFetcher *MarketFetcher
}

// DelegatedOrderServiceOption configures a DelegatedOrderService.
type DelegatedOrderServiceOption func(*DelegatedOrderService)

// WithDelegatedOrderLogger sets the logger for the DelegatedOrderService.
func WithDelegatedOrderLogger(l Logger) DelegatedOrderServiceOption {
	return func(s *DelegatedOrderService) {
		s.logger = l
	}
}

// WithDelegatedOrderMarketFetcher sets a shared MarketFetcher so cached venue
// data is reused when building delegated orders.
func WithDelegatedOrderMarketFetcher(f *MarketFetcher) DelegatedOrderServiceOption {
	return func(s *DelegatedOrderService) {
		s.marketFetcher = f
	}
}

// NewDelegatedOrderService creates a new delegated-order service.
func NewDelegatedOrderService(client *HttpClient, opts ...DelegatedOrderServiceOption) *DelegatedOrderService {
	s := &DelegatedOrderService{
		client: client,
		logger: NewNoOpLogger(),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.marketFetcher == nil {
		s.marketFetcher = NewMarketFetcher(client, WithMarketLogger(s.logger))
	}
	return s
}

// resolveFeeRateBps returns the fee rate to put on a delegated order. Collateral
// fee venues require 0; legacy venues keep the requested rate or the default.
func (s *DelegatedOrderService) resolveFeeRateBps(ctx context.Context, marketSlug string, requested int) (int, error) {
	venue, err := resolveMarketVenue(ctx, s.marketFetcher, s.logger, marketSlug)
	if err != nil {
		return 0, err
	}
	if venue.IsCollateralFeeVenue() {
		return 0, nil
	}
	if requested <= 0 {
		return defaultDelegatedFeeRateBps, nil
	}
	return requested, nil
}

// CreateOrder builds an unsigned order and lets the API sign it on behalf of a target profile.
func (s *DelegatedOrderService) CreateOrder(ctx context.Context, params CreateDelegatedOrderParams) (*OrderResponse, error) {
	result, err := s.CreateOrderWithRawResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// CreateOrderWithRawResponse is the raw-response variant of CreateOrder.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (s *DelegatedOrderService) CreateOrderWithRawResponse(ctx context.Context, params CreateDelegatedOrderParams) (*RawResult[OrderResponse], error) {
	if err := s.client.requireAuth("CreateDelegatedOrder"); err != nil {
		return nil, err
	}
	if params.OnBehalfOf <= 0 {
		return nil, fmt.Errorf("OnBehalfOf must be a positive integer")
	}
	receiveWindow, err := normalizeReceiveWindowOptions(params.ReceiveWindow, time.Now().UnixMilli)
	if err != nil {
		return nil, err
	}
	feeRateBps, err := s.resolveFeeRateBps(ctx, params.MarketSlug, params.FeeRateBps)
	if err != nil {
		return nil, err
	}

	builder := NewOrderBuilder(ZeroAddress, feeRateBps)
	unsignedOrder, err := builder.BuildOrder(params.Args)
	if err != nil {
		return nil, fmt.Errorf("failed to build delegated order: %w", err)
	}

	payload := CreateOrderRequest{
		Order: OrderSubmission{
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
			SignatureType: SignatureTypeEOA,
			Price:         unsignedOrder.Price,
		},
		OrderType:  params.OrderType,
		MarketSlug: params.MarketSlug,
		OwnerID:    params.OnBehalfOf,
		OnBehalfOf: &params.OnBehalfOf,
		PostOnly:   postOnlyFromArgs(params.Args),
		Timestamp:  receiveWindow.Timestamp,
		RecvWindow: receiveWindow.RecvWindow,
	}

	raw, err := s.client.PostRaw(ctx, "/orders", payload)
	return decodeRawResult[OrderResponse](raw, err)
}

// Cancel cancels a delegated order by ID and returns the API message.
func (s *DelegatedOrderService) Cancel(ctx context.Context, orderID string) (string, error) {
	result, err := s.CancelWithRawResponse(ctx, orderID)
	if err != nil {
		return "", err
	}
	return result.Data.Message, nil
}

// CancelWithRawResponse is the raw-response variant of Cancel.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (s *DelegatedOrderService) CancelWithRawResponse(ctx context.Context, orderID string) (*RawResult[CancelResponse], error) {
	if err := s.client.requireAuth("CancelDelegatedOrder"); err != nil {
		return nil, err
	}

	raw, err := s.client.DeleteRaw(ctx, "/orders/"+url.PathEscape(orderID))
	return decodeRawResult[CancelResponse](raw, err)
}

// CancelOnBehalfOf cancels a delegated order by ID for a target profile and returns the API message.
func (s *DelegatedOrderService) CancelOnBehalfOf(ctx context.Context, orderID string, onBehalfOf int) (string, error) {
	result, err := s.CancelOnBehalfOfWithRawResponse(ctx, orderID, onBehalfOf)
	if err != nil {
		return "", err
	}
	return result.Data.Message, nil
}

// CancelOnBehalfOfWithRawResponse is the raw-response variant of CancelOnBehalfOf.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (s *DelegatedOrderService) CancelOnBehalfOfWithRawResponse(ctx context.Context, orderID string, onBehalfOf int) (*RawResult[CancelResponse], error) {
	if err := s.client.requireAuth("CancelDelegatedOrder"); err != nil {
		return nil, err
	}
	if onBehalfOf <= 0 {
		return nil, fmt.Errorf("OnBehalfOf must be a positive integer")
	}

	path := fmt.Sprintf("/orders/%s?onBehalfOf=%d", url.PathEscape(orderID), onBehalfOf)
	raw, err := s.client.DeleteRaw(ctx, path)
	return decodeRawResult[CancelResponse](raw, err)
}

// CancelAll cancels all delegated orders for a market and returns the API message.
func (s *DelegatedOrderService) CancelAll(ctx context.Context, marketSlug string) (string, error) {
	result, err := s.CancelAllWithRawResponse(ctx, marketSlug)
	if err != nil {
		return "", err
	}
	return result.Data.Message, nil
}

// CancelAllWithRawResponse is the raw-response variant of CancelAll.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (s *DelegatedOrderService) CancelAllWithRawResponse(ctx context.Context, marketSlug string) (*RawResult[CancelResponse], error) {
	if err := s.client.requireAuth("CancelAllDelegatedOrders"); err != nil {
		return nil, err
	}

	raw, err := s.client.DeleteRaw(ctx, "/orders/all/"+url.PathEscape(marketSlug))
	return decodeRawResult[CancelResponse](raw, err)
}

// CancelAllOnBehalfOf cancels all delegated orders for a market for a target profile and returns the API message.
func (s *DelegatedOrderService) CancelAllOnBehalfOf(ctx context.Context, marketSlug string, onBehalfOf int) (string, error) {
	result, err := s.CancelAllOnBehalfOfWithRawResponse(ctx, marketSlug, onBehalfOf)
	if err != nil {
		return "", err
	}
	return result.Data.Message, nil
}

// CancelAllOnBehalfOfWithRawResponse is the raw-response variant of CancelAllOnBehalfOf.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (s *DelegatedOrderService) CancelAllOnBehalfOfWithRawResponse(ctx context.Context, marketSlug string, onBehalfOf int) (*RawResult[CancelResponse], error) {
	if err := s.client.requireAuth("CancelAllDelegatedOrders"); err != nil {
		return nil, err
	}
	if onBehalfOf <= 0 {
		return nil, fmt.Errorf("OnBehalfOf must be a positive integer")
	}

	path := fmt.Sprintf("/orders/all/%s?onBehalfOf=%d", url.PathEscape(marketSlug), onBehalfOf)
	raw, err := s.client.DeleteRaw(ctx, path)
	return decodeRawResult[CancelResponse](raw, err)
}
