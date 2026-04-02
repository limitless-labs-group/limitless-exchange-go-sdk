package limitless

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestOrderClient_CreateOrder_PipelineAndCaching(t *testing.T) {
	t.Parallel()

	var profileHits int32
	var marketHits int32
	var orderHits int32

	mux := http.NewServeMux()
	mux.HandleFunc("/profiles/"+testSignerAddress, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&profileHits, 1)
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
		atomic.AddInt32(&marketHits, 1)
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
		atomic.AddInt32(&orderHits, 1)
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST /orders, got %s", r.Method)
		}

		var payload NewOrderPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode order payload: %v", err)
		}
		if payload.OwnerID != 42 {
			t.Fatalf("expected ownerId 42, got %d", payload.OwnerID)
		}
		if payload.OrderType != OrderTypeGTC {
			t.Fatalf("expected orderType GTC, got %s", payload.OrderType)
		}
		if payload.MarketSlug != "test-market" {
			t.Fatalf("expected market slug test-market, got %s", payload.MarketSlug)
		}
		if payload.Order.FeeRateBps != 250 {
			t.Fatalf("expected feeRateBps 250 from user rank, got %d", payload.Order.FeeRateBps)
		}
		if len(payload.Order.Signature) != 132 || !strings.HasPrefix(payload.Order.Signature, "0x") {
			t.Fatalf("expected 132-char hex signature with 0x prefix, got %q", payload.Order.Signature)
		}

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
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	httpClient := NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("test-key"))
	orderClient, err := NewOrderClient(httpClient, testPrivateKeyHex)
	if err != nil {
		t.Fatalf("NewOrderClient returned error: %v", err)
	}

	params := CreateOrderParams{
		OrderType:  OrderTypeGTC,
		MarketSlug: "test-market",
		Args: GTCOrderArgs{
			TokenID: "12345",
			Side:    SideBuy,
			Price:   0.381,
			Size:    1.234,
		},
	}

	resp1, err := orderClient.CreateOrder(context.Background(), params)
	if err != nil {
		t.Fatalf("CreateOrder #1 returned error: %v", err)
	}
	if resp1.Order.ID != "order-1" {
		t.Fatalf("expected response order id order-1, got %s", resp1.Order.ID)
	}

	resp2, err := orderClient.CreateOrder(context.Background(), params)
	if err != nil {
		t.Fatalf("CreateOrder #2 returned error: %v", err)
	}
	if resp2.Order.ID != "order-1" {
		t.Fatalf("expected response order id order-1, got %s", resp2.Order.ID)
	}

	if got := atomic.LoadInt32(&profileHits); got != 1 {
		t.Fatalf("expected profile to be fetched once (cached), got %d", got)
	}
	if got := atomic.LoadInt32(&marketHits); got != 1 {
		t.Fatalf("expected market venue to be fetched once (cached), got %d", got)
	}
	if got := atomic.LoadInt32(&orderHits); got != 2 {
		t.Fatalf("expected two order submissions, got %d", got)
	}
}

func TestOrderClient_BuildUnsignedOrderAndSignOrderForMarket(t *testing.T) {
	t.Parallel()

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

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	httpClient := NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("test-key"))
	orderClient, err := NewOrderClient(httpClient, testPrivateKeyHex)
	if err != nil {
		t.Fatalf("NewOrderClient returned error: %v", err)
	}

	unsignedOrder, err := orderClient.BuildUnsignedOrder(context.Background(), GTCOrderArgs{
		TokenID: "12345",
		Side:    SideBuy,
		Price:   0.381,
		Size:    1.234,
	})
	if err != nil {
		t.Fatalf("BuildUnsignedOrder returned error: %v", err)
	}

	if _, err := orderClient.SignOrder(unsignedOrder); err == nil {
		t.Fatal("expected SignOrder without explicit contract address to fail")
	}

	signature, err := orderClient.SignOrderForMarket(context.Background(), "test-market", unsignedOrder)
	if err != nil {
		t.Fatalf("SignOrderForMarket returned error: %v", err)
	}
	if len(signature) != 132 || !strings.HasPrefix(signature, "0x") {
		t.Fatalf("expected 132-char hex signature with 0x prefix, got %q", signature)
	}
}

func TestOrderClient_CreateOrder_UsesFallbackSigningContractWhenVenueMissing(t *testing.T) {
	t.Parallel()

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
		})
	})
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		var payload NewOrderPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode order payload: %v", err)
		}
		if payload.Order.Signature == "" {
			t.Fatal("expected signature to be present")
		}
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
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	httpClient := NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("test-key"))
	orderClient, err := NewOrderClient(
		httpClient,
		testPrivateKeyHex,
		WithSigningConfig(OrderSigningConfig{
			ChainID:         DefaultChainID,
			ContractAddress: "0xa4409D988CA2218d956BeEFD3874100F444f0DC3",
		}),
	)
	if err != nil {
		t.Fatalf("NewOrderClient returned error: %v", err)
	}

	resp, err := orderClient.CreateOrder(context.Background(), CreateOrderParams{
		OrderType:  OrderTypeGTC,
		MarketSlug: "test-market",
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
	if resp.Order.ID != "order-1" {
		t.Fatalf("expected order id order-1, got %s", resp.Order.ID)
	}
}

func TestOrderClient_CreateOrder_RequiresAPIKey(t *testing.T) {
	t.Parallel()

	httpClient := NewHttpClient(WithBaseURL("https://example.com"))
	orderClient, err := NewOrderClient(httpClient, testPrivateKeyHex)
	if err != nil {
		t.Fatalf("NewOrderClient returned error: %v", err)
	}

	_, err = orderClient.CreateOrder(context.Background(), CreateOrderParams{
		OrderType:  OrderTypeFOK,
		MarketSlug: "test-market",
		Args: FOKOrderArgs{
			TokenID:     "12345",
			Side:        SideBuy,
			MakerAmount: 5.0,
		},
	})
	if err == nil {
		t.Fatal("expected CreateOrder to fail without API key")
	}
	if !strings.Contains(err.Error(), "WithAPIKey") {
		t.Fatalf("expected error to mention WithAPIKey, got: %v", err)
	}
}
