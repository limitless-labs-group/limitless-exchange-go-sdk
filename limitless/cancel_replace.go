package limitless

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (oc *OrderClient) CancelReplace(ctx context.Context, params CancelReplaceParams) (*CancelReplaceResult, error) {
	if err := oc.client.requireAuth("CancelReplace"); err != nil {
		return nil, err
	}
	request, err := oc.buildCancelReplaceRequest(ctx, params)
	if err != nil {
		return nil, err
	}
	return submitCancelReplace(ctx, oc.client, request)
}

func (oc *OrderClient) CancelReplaceBatch(ctx context.Context, operations []CancelReplaceParams) (*CancelReplaceBatchResponse, error) {
	if err := oc.client.requireAuth("CancelReplaceBatch"); err != nil {
		return nil, err
	}
	if len(operations) == 0 {
		return nil, fmt.Errorf("cancel-replace batch requires at least one operation")
	}
	requests := make([]CancelReplaceRequest, len(operations))
	for i, operation := range operations {
		request, err := oc.buildCancelReplaceRequest(ctx, operation)
		if err != nil {
			return nil, fmt.Errorf("operation %d: %w", i, err)
		}
		requests[i] = request
	}
	return submitCancelReplaceBatch(ctx, oc.client, CancelReplaceBatchRequest{Operations: requests})
}

func (oc *OrderClient) buildCancelReplaceRequest(ctx context.Context, params CancelReplaceParams) (CancelReplaceRequest, error) {
	userData, err := oc.ensureUserData(ctx)
	if err != nil {
		return CancelReplaceRequest{}, err
	}
	replacement, err := oc.buildDirectCancelReplaceOrder(ctx, params.Replacement, userData)
	if err != nil {
		return CancelReplaceRequest{}, err
	}
	return CancelReplaceRequest{Cancel: params.Cancel, Replacement: replacement, Mode: params.Mode}, nil
}

func (oc *OrderClient) buildDirectCancelReplaceOrder(ctx context.Context, params CancelReplaceOrderParams, userData *UserData) (CancelReplaceOrder, error) {
	receiveWindow, err := normalizeReceiveWindowOptions(params.ReceiveWindow, time.Now().UnixMilli)
	if err != nil {
		return CancelReplaceOrder{}, err
	}
	unsignedOrder, err := oc.builder.BuildOrder(params.Args)
	if err != nil {
		return CancelReplaceOrder{}, fmt.Errorf("failed to build replacement order: %w", err)
	}
	signingConfig, err := oc.resolveSigningConfigForMarket(ctx, params.MarketSlug)
	if err != nil {
		return CancelReplaceOrder{}, err
	}
	signature, err := oc.signer.SignOrder(unsignedOrder, signingConfig)
	if err != nil {
		return CancelReplaceOrder{}, fmt.Errorf("failed to sign replacement order: %w", err)
	}
	return cancelReplaceOrderFromUnsigned(unsignedOrder, signature, params, userData.UserID, receiveWindow), nil
}

func (s *DelegatedOrderService) CancelReplace(ctx context.Context, params DelegatedCancelReplaceParams) (*CancelReplaceResult, error) {
	if err := s.client.requireAuth("DelegatedCancelReplace"); err != nil {
		return nil, err
	}
	request, err := buildDelegatedCancelReplaceRequest(params)
	if err != nil {
		return nil, err
	}
	return submitCancelReplace(ctx, s.client, request)
}

func (s *DelegatedOrderService) CancelReplaceBatch(ctx context.Context, operations []DelegatedCancelReplaceParams) (*CancelReplaceBatchResponse, error) {
	if err := s.client.requireAuth("DelegatedCancelReplaceBatch"); err != nil {
		return nil, err
	}
	if len(operations) == 0 {
		return nil, fmt.Errorf("cancel-replace batch requires at least one operation")
	}
	requests := make([]CancelReplaceRequest, len(operations))
	for i, operation := range operations {
		request, err := buildDelegatedCancelReplaceRequest(operation)
		if err != nil {
			return nil, fmt.Errorf("operation %d: %w", i, err)
		}
		requests[i] = request
	}
	return submitCancelReplaceBatch(ctx, s.client, CancelReplaceBatchRequest{Operations: requests})
}

func buildDelegatedCancelReplaceRequest(params DelegatedCancelReplaceParams) (CancelReplaceRequest, error) {
	if params.OnBehalfOf <= 0 {
		return CancelReplaceRequest{}, fmt.Errorf("OnBehalfOf must be a positive integer")
	}
	feeRateBps := params.FeeRateBps
	if feeRateBps <= 0 {
		feeRateBps = defaultDelegatedFeeRateBps
	}
	receiveWindow, err := normalizeReceiveWindowOptions(params.Replacement.ReceiveWindow, time.Now().UnixMilli)
	if err != nil {
		return CancelReplaceRequest{}, err
	}
	builder := NewOrderBuilder(ZeroAddress, feeRateBps)
	unsignedOrder, err := builder.BuildOrder(params.Replacement.Args)
	if err != nil {
		return CancelReplaceRequest{}, fmt.Errorf("failed to build delegated replacement order: %w", err)
	}
	onBehalfOf := params.OnBehalfOf
	return CancelReplaceRequest{
		Cancel: params.Cancel,
		Replacement: cancelReplaceOrderFromUnsigned(
			unsignedOrder, "", params.Replacement, params.OnBehalfOf, receiveWindow,
		),
		Mode:       params.Mode,
		OnBehalfOf: &onBehalfOf,
	}, nil
}

func cancelReplaceOrderFromUnsigned(unsignedOrder *UnsignedOrder, signature string, params CancelReplaceOrderParams, ownerID int, receiveWindow ReceiveWindowOptions) CancelReplaceOrder {
	return CancelReplaceOrder{
		Order: OrderSubmission{
			Salt: unsignedOrder.Salt, Maker: unsignedOrder.Maker, Signer: unsignedOrder.Signer,
			Taker: unsignedOrder.Taker, TokenID: unsignedOrder.TokenID, MakerAmount: unsignedOrder.MakerAmount,
			TakerAmount: unsignedOrder.TakerAmount, Expiration: unsignedOrder.Expiration, Nonce: unsignedOrder.Nonce,
			FeeRateBps: unsignedOrder.FeeRateBps, Side: unsignedOrder.Side, SignatureType: unsignedOrder.SignatureType,
			Price: unsignedOrder.Price, Signature: signature,
		},
		OwnerID: ownerID, OrderType: params.OrderType, MarketSlug: params.MarketSlug,
		PostOnly: postOnlyFromArgs(params.Args), ClientOrderID: params.ClientOrderID,
		Timestamp: receiveWindow.Timestamp, RecvWindow: receiveWindow.RecvWindow, STPPolicy: params.STPPolicy,
	}
}

func submitCancelReplace(ctx context.Context, client *HttpClient, request CancelReplaceRequest) (*CancelReplaceResult, error) {
	raw, err := client.PostRaw(ctx, "/orders/cancel-replace", request, AllowStatus(http.StatusConflict))
	if err != nil {
		return nil, err
	}
	var result CancelReplaceResult
	if err := json.Unmarshal(raw.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode cancel-replace response: %w", err)
	}
	return &result, nil
}

func submitCancelReplaceBatch(ctx context.Context, client *HttpClient, request CancelReplaceBatchRequest) (*CancelReplaceBatchResponse, error) {
	raw, err := client.PostRaw(ctx, "/orders/cancel-replace/batch", request)
	if err != nil {
		return nil, err
	}
	var result CancelReplaceBatchResponse
	if err := json.Unmarshal(raw.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode cancel-replace batch response: %w", err)
	}
	for i, item := range result.Results {
		if item.Index < 0 {
			return nil, fmt.Errorf("failed to decode cancel-replace batch response: result %d has negative index", i)
		}
	}
	return &result, nil
}
