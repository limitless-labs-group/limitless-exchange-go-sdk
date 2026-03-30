package limitless

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDelegatedOrderService_CreateOrder(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-Key"); got != "api-key" {
			t.Fatalf("expected X-API-Key api-key, got %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		raw := string(body)
		if strings.Contains(raw, `"signature"`) {
			t.Fatalf("expected signature to be omitted for delegated order, got %s", raw)
		}

		var payload CreateOrderRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode delegated order payload: %v", err)
		}
		if payload.OwnerID != 42 || payload.OnBehalfOf == nil || *payload.OnBehalfOf != 42 {
			t.Fatalf("unexpected ownership fields: %+v", payload)
		}
		if payload.Order.FeeRateBps != 250 {
			t.Fatalf("expected feeRateBps 250, got %d", payload.Order.FeeRateBps)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"order": map[string]any{
				"id":            "delegated-1",
				"createdAt":     "2026-03-23T12:00:00.000Z",
				"makerAmount":   "470154",
				"takerAmount":   "1234000",
				"expiration":    "0",
				"signatureType": 0,
				"salt":          "1742191300000000",
				"maker":         "0xserver",
				"signer":        "0xserver",
				"taker":         payload.Order.Taker,
				"tokenId":       payload.Order.TokenID,
				"side":          payload.Order.Side,
				"feeRateBps":    payload.Order.FeeRateBps,
				"nonce":         payload.Order.Nonce,
				"signature":     "0xsigned",
				"orderType":     string(payload.OrderType),
				"price":         "0.381",
				"marketId":      7,
			},
			"makerMatches": []any{},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	service := NewDelegatedOrderService(NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("api-key")))
	resp, err := service.CreateOrder(context.Background(), CreateDelegatedOrderParams{
		MarketSlug: "test-market",
		OrderType:  OrderTypeGTC,
		OnBehalfOf: 42,
		FeeRateBps: 250,
		Args: GTCOrderArgs{
			TokenID: "12345",
			Side:    SideBuy,
			Price:   0.381,
			Size:    1.234,
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if resp.Order.ID != "delegated-1" {
		t.Fatalf("expected delegated order id delegated-1, got %s", resp.Order.ID)
	}
}

func TestDelegatedOrderService_CreateOrder_DefaultFeeRate(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload CreateOrderRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode delegated order payload: %v", err)
		}
		if payload.Order.FeeRateBps != defaultDelegatedFeeRateBps {
			t.Fatalf("expected default feeRateBps %d, got %d", defaultDelegatedFeeRateBps, payload.Order.FeeRateBps)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"order": map[string]any{
				"id":            "delegated-default-fee",
				"createdAt":     "2026-03-23T12:00:00.000Z",
				"makerAmount":   "50000",
				"takerAmount":   "1000000",
				"expiration":    "0",
				"signatureType": 0,
				"salt":          "1742191300000001",
				"maker":         "0xserver",
				"signer":        "0xserver",
				"taker":         payload.Order.Taker,
				"tokenId":       payload.Order.TokenID,
				"side":          payload.Order.Side,
				"feeRateBps":    payload.Order.FeeRateBps,
				"nonce":         payload.Order.Nonce,
				"signature":     "0xsigned",
				"orderType":     string(payload.OrderType),
				"price":         "0.050",
				"marketId":      7,
			},
			"makerMatches": []any{},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	service := NewDelegatedOrderService(NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("api-key")))
	resp, err := service.CreateOrder(context.Background(), CreateDelegatedOrderParams{
		MarketSlug: "test-market",
		OrderType:  OrderTypeGTC,
		OnBehalfOf: 42,
		Args: GTCOrderArgs{
			TokenID: "12345",
			Side:    SideBuy,
			Price:   0.05,
			Size:    1.0,
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if resp.Order.FeeRateBps != defaultDelegatedFeeRateBps {
		t.Fatalf("expected response feeRateBps %d, got %d", defaultDelegatedFeeRateBps, resp.Order.FeeRateBps)
	}
}
