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
	client *HttpClient
	logger Logger
}

// DelegatedOrderServiceOption configures a DelegatedOrderService.
type DelegatedOrderServiceOption func(*DelegatedOrderService)

// WithDelegatedOrderLogger sets the logger for the DelegatedOrderService.
func WithDelegatedOrderLogger(l Logger) DelegatedOrderServiceOption {
	return func(s *DelegatedOrderService) {
		s.logger = l
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
	return s
}

// CreateOrder builds an unsigned order and lets the API sign it on behalf of a target profile.
func (s *DelegatedOrderService) CreateOrder(ctx context.Context, params CreateDelegatedOrderParams) (*OrderResponse, error) {
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
	if params.FeeRateBps <= 0 {
		params.FeeRateBps = defaultDelegatedFeeRateBps
	}

	builder := NewOrderBuilder(ZeroAddress, params.FeeRateBps)
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
		// stpPolicy is sent top-level; do NOT add it to the signed order (would change the signature).
		StpPolicy:  params.StpPolicy,
		PostOnly:   postOnlyFromArgs(params.Args),
		Timestamp:  receiveWindow.Timestamp,
		RecvWindow: receiveWindow.RecvWindow,
	}

	var resp OrderResponse
	if err := s.client.Post(ctx, "/orders", payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Cancel cancels a delegated order by ID and returns the API message.
func (s *DelegatedOrderService) Cancel(ctx context.Context, orderID string) (string, error) {
	if err := s.client.requireAuth("CancelDelegatedOrder"); err != nil {
		return "", err
	}

	var resp CancelResponse
	if err := s.client.Delete(ctx, "/orders/"+url.PathEscape(orderID), &resp); err != nil {
		return "", err
	}
	return resp.Message, nil
}

// CancelOnBehalfOf cancels a delegated order by ID for a target profile and returns the API message.
func (s *DelegatedOrderService) CancelOnBehalfOf(ctx context.Context, orderID string, onBehalfOf int) (string, error) {
	if err := s.client.requireAuth("CancelDelegatedOrder"); err != nil {
		return "", err
	}
	if onBehalfOf <= 0 {
		return "", fmt.Errorf("OnBehalfOf must be a positive integer")
	}

	var resp CancelResponse
	path := fmt.Sprintf("/orders/%s?onBehalfOf=%d", url.PathEscape(orderID), onBehalfOf)
	if err := s.client.Delete(ctx, path, &resp); err != nil {
		return "", err
	}
	return resp.Message, nil
}

// CancelAll cancels all delegated orders for a market and returns the API message.
func (s *DelegatedOrderService) CancelAll(ctx context.Context, marketSlug string) (string, error) {
	if err := s.client.requireAuth("CancelAllDelegatedOrders"); err != nil {
		return "", err
	}

	var resp CancelResponse
	if err := s.client.Delete(ctx, "/orders/all/"+url.PathEscape(marketSlug), &resp); err != nil {
		return "", err
	}
	return resp.Message, nil
}

// CancelAllOnBehalfOf cancels all delegated orders for a market for a target profile and returns the API message.
func (s *DelegatedOrderService) CancelAllOnBehalfOf(ctx context.Context, marketSlug string, onBehalfOf int) (string, error) {
	if err := s.client.requireAuth("CancelAllDelegatedOrders"); err != nil {
		return "", err
	}
	if onBehalfOf <= 0 {
		return "", fmt.Errorf("OnBehalfOf must be a positive integer")
	}

	var resp CancelResponse
	path := fmt.Sprintf("/orders/all/%s?onBehalfOf=%d", url.PathEscape(marketSlug), onBehalfOf)
	if err := s.client.Delete(ctx, path, &resp); err != nil {
		return "", err
	}
	return resp.Message, nil
}
