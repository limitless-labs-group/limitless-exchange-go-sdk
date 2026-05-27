package limitless

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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
		if strings.Contains(raw, `"timestamp"`) {
			t.Fatalf("expected timestamp to be omitted by default, got %s", raw)
		}
		if strings.Contains(raw, `"recvWindow"`) {
			t.Fatalf("expected recvWindow to be omitted by default, got %s", raw)
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

func TestDelegatedOrderService_CreateOrder_ReceiveWindowTopLevelOnly(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("failed to decode raw delegated order payload: %v", err)
		}
		if string(raw["timestamp"]) != "1770000000000" {
			t.Fatalf("expected top-level timestamp 1770000000000, got %s", string(raw["timestamp"]))
		}
		if string(raw["recvWindow"]) != "1500" {
			t.Fatalf("expected top-level recvWindow 1500, got %s", string(raw["recvWindow"]))
		}

		var rawOrder map[string]json.RawMessage
		if err := json.Unmarshal(raw["order"], &rawOrder); err != nil {
			t.Fatalf("failed to decode raw delegated order body: %v", err)
		}
		if _, ok := rawOrder["timestamp"]; ok {
			t.Fatal("expected timestamp to stay out of delegated order body")
		}
		if _, ok := rawOrder["recvWindow"]; ok {
			t.Fatal("expected recvWindow to stay out of delegated order body")
		}

		var payload CreateOrderRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode delegated order payload: %v", err)
		}
		if payload.Timestamp == nil || *payload.Timestamp != 1_770_000_000_000 {
			t.Fatalf("unexpected timestamp pointer: %v", payload.Timestamp)
		}
		if payload.RecvWindow == nil || *payload.RecvWindow != 1500 {
			t.Fatalf("unexpected recvWindow pointer: %v", payload.RecvWindow)
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
		ReceiveWindow: ReceiveWindowOptions{
			Timestamp:  int64Ptr(1_770_000_000_000),
			RecvWindow: int64Ptr(1500),
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if resp.Order.ID != "delegated-1" {
		t.Fatalf("expected delegated order id delegated-1, got %s", resp.Order.ID)
	}
}

func TestDelegatedOrderService_CreateOrder_AutoStampsTimestampForRecvWindow(t *testing.T) {
	t.Parallel()

	values := make(chan [2]int64, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		var payload CreateOrderRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode delegated order payload: %v", err)
		}
		if payload.Timestamp == nil {
			t.Fatal("expected timestamp to be auto-stamped")
		}
		if payload.RecvWindow == nil {
			t.Fatal("expected recvWindow to be present")
		}
		values <- [2]int64{*payload.Timestamp, *payload.RecvWindow}

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
	before := time.Now().UnixMilli()
	_, err := service.CreateOrder(context.Background(), CreateDelegatedOrderParams{
		MarketSlug: "test-market",
		OrderType:  OrderTypeGTC,
		OnBehalfOf: 42,
		Args: GTCOrderArgs{
			TokenID: "12345",
			Side:    SideBuy,
			Price:   0.381,
			Size:    1.234,
		},
		ReceiveWindow: ReceiveWindowOptions{
			RecvWindow: int64Ptr(1500),
		},
	})
	after := time.Now().UnixMilli()
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}

	got := <-values
	if got[1] != 1500 {
		t.Fatalf("expected recvWindow 1500, got %d", got[1])
	}
	if got[0] < before || got[0] > after {
		t.Fatalf("expected auto timestamp between %d and %d, got %d", before, after, got[0])
	}
}

func TestDelegatedOrderService_CreateOrder_RejectsInvalidReceiveWindowBeforeNetwork(t *testing.T) {
	t.Parallel()

	var hits int32
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	service := NewDelegatedOrderService(NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("api-key")))
	_, err := service.CreateOrder(context.Background(), CreateDelegatedOrderParams{
		MarketSlug: "test-market",
		OrderType:  OrderTypeGTC,
		OnBehalfOf: 42,
		Args: GTCOrderArgs{
			TokenID: "12345",
			Side:    SideBuy,
			Price:   0.381,
			Size:    1.234,
		},
		ReceiveWindow: ReceiveWindowOptions{
			RecvWindow: int64Ptr(0),
		},
	})
	if err == nil {
		t.Fatal("expected CreateOrder to reject invalid receive-window options")
	}
	if !strings.Contains(err.Error(), "recvWindow") {
		t.Fatalf("expected error to mention recvWindow, got %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Fatalf("expected invalid receive-window values to fail before network, got %d request(s)", got)
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
