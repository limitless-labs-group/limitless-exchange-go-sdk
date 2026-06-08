package limitless

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// stpTestServer returns an httptest server that mimics the profile/market/orders
// pipeline and forwards the raw POST /orders body to bodyCh for assertion.
func stpTestServer(t *testing.T, bodyCh chan<- []byte) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/profiles/"+testSignerAddress, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      42,
			"account": testSignerAddress,
			"rank": map[string]any{
				"id":         1,
				"name":       "Gold",
				"feeRateBps": 250,
			},
		})
	})
	mux.HandleFunc("/markets/test-market", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"slug":  "test-market",
			"title": "Test Market",
			"venue": map[string]any{
				"exchange": "0xa4409D988CA2218d956BeEFD3874100F444f0DC3",
				"adapter":  nil,
			},
		})
	})
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read order payload: %v", err)
			return
		}
		bodyCh <- body

		var payload NewOrderPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("failed to decode order payload: %v", err)
			return
		}

		// Echo a body that carries the always-present execution object with the
		// STP signal so the SDK pass-through can be asserted.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"order": map[string]any{
				"id":            "order-1",
				"createdAt":     "2026-03-17T00:00:00.000Z",
				"makerAmount":   "470154",
				"takerAmount":   "1234000",
				"expiration":    "0",
				"signatureType": 0,
				"salt":          "1742191300000000",
				"maker":         payload.Order.Maker,
				"signer":        payload.Order.Signer,
				"taker":         payload.Order.Taker,
				"tokenId":       payload.Order.TokenID,
				"side":          payload.Order.Side,
				"feeRateBps":    payload.Order.FeeRateBps,
				"nonce":         payload.Order.Nonce,
				"signature":     payload.Order.Signature,
				"orderType":     string(payload.OrderType),
				"price":         "0.381",
				"marketId":      7,
			},
			"makerMatches": []any{},
			"execution": map[string]any{
				"matched":          true,
				"settlementStatus": "CANCELED",
				"reason":           "STP_TAKER_REJECTED",
				"stpMakerCancels":  []string{"maker-uuid-1", "maker-uuid-2"},
				"feeRateBps":       300,
				"effectiveFeeBps":  0,
				"totalsRaw": map[string]any{
					"contractsGross": "10",
					"contractsFee":   "0",
					"contractsNet":   "10",
					"usdGross":       "5.00",
					"usdFee":         "0",
					"usdNet":         "5.00",
				},
			},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestOrderClient_CreateOrder_StpPolicyTopLevelAndExecutionPassThrough(t *testing.T) {
	t.Parallel()

	bodyCh := make(chan []byte, 1)
	srv := stpTestServer(t, bodyCh)

	httpClient := NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("test-key"))
	orderClient, err := NewOrderClient(httpClient, testPrivateKeyHex)
	if err != nil {
		t.Fatalf("NewOrderClient returned error: %v", err)
	}

	resp, err := orderClient.CreateOrder(context.Background(), CreateOrderParams{
		OrderType:  OrderTypeGTC,
		MarketSlug: "test-market",
		StpPolicy:  StpPolicyCancelMaker,
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

	body := <-bodyCh

	// stpPolicy must ride top-level on the request body...
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("failed to decode raw order payload: %v", err)
	}
	if got := string(raw["stpPolicy"]); got != `"cancel_maker"` {
		t.Fatalf("expected top-level stpPolicy \"cancel_maker\", got %s", got)
	}

	// ...and must never enter the signed order struct.
	var rawOrder map[string]json.RawMessage
	if err := json.Unmarshal(raw["order"], &rawOrder); err != nil {
		t.Fatalf("failed to decode raw order body: %v", err)
	}
	if _, ok := rawOrder["stpPolicy"]; ok {
		t.Fatal("expected stpPolicy to stay out of the signed order")
	}

	// Execution must be surfaced (previously dropped) with correct typing.
	exec := resp.Execution
	if !exec.Matched {
		t.Fatal("expected execution.matched to be true")
	}
	if exec.SettlementStatus != "CANCELED" {
		t.Fatalf("expected settlementStatus CANCELED, got %q", exec.SettlementStatus)
	}
	if exec.Reason != "STP_TAKER_REJECTED" {
		t.Fatalf("expected reason STP_TAKER_REJECTED, got %q", exec.Reason)
	}
	if len(exec.StpMakerCancels) != 2 || exec.StpMakerCancels[0] != "maker-uuid-1" {
		t.Fatalf("unexpected stpMakerCancels: %v", exec.StpMakerCancels)
	}
	if exec.FeeRateBps != 300 || exec.EffectiveFeeBps != 0 {
		t.Fatalf("unexpected fee bps numbers: %d / %d", exec.FeeRateBps, exec.EffectiveFeeBps)
	}
	if exec.TotalsRaw.UsdNet != "5.00" || exec.TotalsRaw.ContractsGross != "10" {
		t.Fatalf("unexpected totalsRaw strings: %+v", exec.TotalsRaw)
	}
}

func TestOrderClient_CreateOrder_StpPolicyOmittedWhenUnset(t *testing.T) {
	t.Parallel()

	bodyCh := make(chan []byte, 1)
	srv := stpTestServer(t, bodyCh)

	httpClient := NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("test-key"))
	orderClient, err := NewOrderClient(httpClient, testPrivateKeyHex)
	if err != nil {
		t.Fatalf("NewOrderClient returned error: %v", err)
	}

	if _, err := orderClient.CreateOrder(context.Background(), CreateOrderParams{
		OrderType:  OrderTypeGTC,
		MarketSlug: "test-market",
		Args: GTCOrderArgs{
			TokenID: "12345",
			Side:    SideBuy,
			Price:   0.381,
			Size:    1.234,
		},
	}); err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}

	body := <-bodyCh
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("failed to decode raw order payload: %v", err)
	}
	if _, ok := raw["stpPolicy"]; ok {
		t.Fatal("expected stpPolicy to be omitted when unset")
	}
}

func TestDelegatedOrderService_CreateOrder_StpPolicyTopLevelOnly(t *testing.T) {
	t.Parallel()

	bodyCh := make(chan []byte, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
			return
		}
		bodyCh <- body

		var payload CreateOrderRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("failed to decode delegated order payload: %v", err)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"order": map[string]any{
				"id":          "delegated-1",
				"makerAmount": "470154",
				"takerAmount": "1234000",
				"salt":        "1742191300000000",
				"orderType":   string(payload.OrderType),
				"marketId":    7,
			},
			"makerMatches": []any{},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	service := NewDelegatedOrderService(NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("api-key")))
	if _, err := service.CreateOrder(context.Background(), CreateDelegatedOrderParams{
		MarketSlug: "test-market",
		OrderType:  OrderTypeGTC,
		OnBehalfOf: 42,
		FeeRateBps: 250,
		StpPolicy:  StpPolicyCancelBoth,
		Args: GTCOrderArgs{
			TokenID: "12345",
			Side:    SideBuy,
			Price:   0.381,
			Size:    1.234,
		},
	}); err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}

	body := <-bodyCh
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("failed to decode raw payload: %v", err)
	}
	if got := string(raw["stpPolicy"]); got != `"cancel_both"` {
		t.Fatalf("expected top-level stpPolicy \"cancel_both\", got %s", got)
	}
	var rawOrder map[string]json.RawMessage
	if err := json.Unmarshal(raw["order"], &rawOrder); err != nil {
		t.Fatalf("failed to decode raw order body: %v", err)
	}
	if _, ok := rawOrder["stpPolicy"]; ok {
		t.Fatal("expected stpPolicy to stay out of the delegated order struct")
	}
}

func TestOrderResponse_UnmarshalWithoutExecution(t *testing.T) {
	t.Parallel()

	// A body without execution must still decode cleanly to a zero-value
	// execution (tolerant, never a hard deser failure).
	body := []byte(`{"order":{"id":"order-1","makerAmount":"1","takerAmount":"1","salt":"1"},"makerMatches":[]}`)

	var resp OrderResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("expected tolerant unmarshal, got error: %v", err)
	}
	if resp.Order.ID != "order-1" {
		t.Fatalf("expected order id order-1, got %q", resp.Order.ID)
	}
	if resp.Execution.Matched || resp.Execution.SettlementStatus != "" {
		t.Fatalf("expected zero-value execution, got %+v", resp.Execution)
	}
}
