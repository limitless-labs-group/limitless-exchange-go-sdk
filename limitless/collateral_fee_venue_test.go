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
)

const (
	testVenueExchange  = "0xa4409D988CA2218d956BeEFD3874100F444f0DC3"
	testVenueFeeModule = "0x1111111111111111111111111111111111111111"

	legacyTestVenueJSON     = `{"exchange":"` + testVenueExchange + `","adapter":null}`
	collateralTestVenueJSON = `{"exchange":"` + testVenueExchange + `","adapter":null,"feeModule":"` + testVenueFeeModule + `","feeModuleType":"COLLATERAL"}`
)

// handleTestMarket serves GET /markets/{slug} with the given venue JSON.
func handleTestMarket(t *testing.T, mux *http.ServeMux, slug, venueJSON string) *int32 {
	t.Helper()
	var hits int32
	mux.HandleFunc("/markets/"+slug, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{"slug":"` + slug + `","venue":` + venueJSON + `}`))
	})
	return &hits
}

func handleTestProfile(mux *http.ServeMux) {
	mux.HandleFunc("/profiles/"+testSignerAddress, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":42,"account":"` + testSignerAddress + `","rank":{"feeRateBps":250}}`))
	})
}

func writeTestOrderResponse(w http.ResponseWriter, order OrderSubmission, orderType OrderType) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"order": map[string]any{
			"id":            "order-1",
			"createdAt":     "2026-03-17T00:00:00.000Z",
			"makerAmount":   "500000",
			"takerAmount":   "1000000",
			"expiration":    "0",
			"signatureType": 0,
			"salt":          "1742191300000000",
			"maker":         order.Maker,
			"signer":        order.Signer,
			"taker":         order.Taker,
			"tokenId":       order.TokenID,
			"side":          order.Side,
			"feeRateBps":    order.FeeRateBps,
			"nonce":         order.Nonce,
			"signature":     order.Signature,
			"orderType":     string(orderType),
			"price":         "0.5",
			"marketId":      7,
		},
		"makerMatches": []any{},
	})
}

func TestVenue_IsCollateralFeeVenue(t *testing.T) {
	t.Parallel()

	var nilVenue *Venue
	cases := []struct {
		name  string
		venue *Venue
		want  bool
	}{
		{name: "nil venue", venue: nilVenue, want: false},
		{name: "legacy venue", venue: &Venue{Exchange: testVenueExchange}, want: false},
		{name: "collateral fee venue", venue: &Venue{Exchange: testVenueExchange, FeeModule: testVenueFeeModule, FeeModuleType: FeeModuleTypeCollateral}, want: true},
		{name: "unknown fee module type", venue: &Venue{Exchange: testVenueExchange, FeeModule: testVenueFeeModule, FeeModuleType: "OTHER"}, want: false},
	}
	for _, tc := range cases {
		if got := tc.venue.IsCollateralFeeVenue(); got != tc.want {
			t.Errorf("%s: IsCollateralFeeVenue() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestVenue_JSONFeeModuleFields(t *testing.T) {
	t.Parallel()

	var collateral Venue
	if err := json.Unmarshal([]byte(collateralTestVenueJSON), &collateral); err != nil {
		t.Fatalf("failed to decode collateral venue: %v", err)
	}
	if collateral.FeeModule != testVenueFeeModule || collateral.FeeModuleType != FeeModuleTypeCollateral {
		t.Fatalf("unexpected collateral venue: %+v", collateral)
	}
	if !collateral.IsCollateralFeeVenue() {
		t.Fatal("expected decoded venue to be a collateral fee venue")
	}

	var legacy Venue
	if err := json.Unmarshal([]byte(legacyTestVenueJSON), &legacy); err != nil {
		t.Fatalf("failed to decode legacy venue: %v", err)
	}
	if legacy.FeeModule != "" || legacy.FeeModuleType != "" || legacy.IsCollateralFeeVenue() {
		t.Fatalf("unexpected legacy venue: %+v", legacy)
	}

	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("failed to encode legacy venue: %v", err)
	}
	if strings.Contains(string(encoded), "feeModule") {
		t.Fatalf("expected legacy venue to omit fee module fields, got %s", encoded)
	}
}

func TestOrderClient_CreateOrder_CollateralFeeVenueSignsZeroFeeRate(t *testing.T) {
	t.Parallel()

	var orderClient *OrderClient
	mux := http.NewServeMux()
	handleTestProfile(mux)
	handleTestMarket(t, mux, "fee-market", collateralTestVenueJSON)
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		var payload NewOrderPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode order payload: %v", err)
		}
		if payload.Order.FeeRateBps != 0 {
			t.Fatalf("expected feeRateBps 0 on collateral fee venue, got %d", payload.Order.FeeRateBps)
		}
		assertSignedWithPayloadFields(t, orderClient, payload.Order)
		writeTestOrderResponse(w, OrderSubmission{
			Maker: payload.Order.Maker, Signer: payload.Order.Signer, Taker: payload.Order.Taker,
			TokenID: payload.Order.TokenID, Side: payload.Order.Side, FeeRateBps: payload.Order.FeeRateBps,
			Nonce: payload.Order.Nonce, Signature: payload.Order.Signature,
		}, payload.OrderType)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	var err error
	orderClient, err = NewOrderClient(NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("key")), testPrivateKeyHex)
	if err != nil {
		t.Fatalf("NewOrderClient returned error: %v", err)
	}
	resp, err := orderClient.CreateOrder(context.Background(), CreateOrderParams{
		OrderType:  OrderTypeGTC,
		MarketSlug: "fee-market",
		Args:       GTCOrderArgs{TokenID: "123", Side: SideBuy, Price: 0.5, Size: 1},
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if resp.Order.FeeRateBps != 0 {
		t.Fatalf("expected response feeRateBps 0, got %d", resp.Order.FeeRateBps)
	}
}

// assertSignedWithPayloadFields re-signs the submitted order fields and checks the
// signature matches, proving the fee rate was overridden before signing.
func assertSignedWithPayloadFields(t *testing.T, oc *OrderClient, order SignedOrder) {
	t.Helper()
	expected, err := oc.SignOrderWithConfig(&UnsignedOrder{
		Salt: order.Salt, Maker: order.Maker, Signer: order.Signer, Taker: order.Taker,
		TokenID: order.TokenID, MakerAmount: order.MakerAmount, TakerAmount: order.TakerAmount,
		Expiration: order.Expiration, Nonce: order.Nonce, FeeRateBps: order.FeeRateBps,
		Side: order.Side, SignatureType: order.SignatureType, Price: order.Price,
	}, OrderSigningConfig{ChainID: oc.signingConfig.ChainID, ContractAddress: testVenueExchange})
	if err != nil {
		t.Fatalf("failed to re-sign order: %v", err)
	}
	if !strings.EqualFold(expected, order.Signature) {
		t.Fatalf("signature does not cover submitted fields: got %s, want %s", order.Signature, expected)
	}
}

func TestOrderClient_CancelReplace_FeeRateByVenue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		venueJSON string
		want      int
	}{
		{name: "collateral fee venue", venueJSON: collateralTestVenueJSON, want: 0},
		{name: "legacy venue", venueJSON: legacyTestVenueJSON, want: 250},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var orderClient *OrderClient
			mux := http.NewServeMux()
			handleTestProfile(mux)
			handleTestMarket(t, mux, "market", tc.venueJSON)
			mux.HandleFunc("/orders/cancel-replace", func(w http.ResponseWriter, r *http.Request) {
				var request CancelReplaceRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatalf("failed to decode cancel-replace request: %v", err)
				}
				order := request.Replacement.Order
				if order.FeeRateBps != tc.want {
					t.Fatalf("expected feeRateBps %d, got %d", tc.want, order.FeeRateBps)
				}
				assertSignedWithPayloadFields(t, orderClient, SignedOrder{
					Salt: order.Salt, Maker: order.Maker, Signer: order.Signer, Taker: order.Taker,
					TokenID: order.TokenID, MakerAmount: order.MakerAmount, TakerAmount: order.TakerAmount,
					Expiration: order.Expiration, Nonce: order.Nonce, FeeRateBps: order.FeeRateBps,
					Side: order.Side, SignatureType: order.SignatureType, Price: order.Price, Signature: order.Signature,
				})
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(cancelReplaceFailureResponse))
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			var err error
			orderClient, err = NewOrderClient(NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("key")), testPrivateKeyHex)
			if err != nil {
				t.Fatalf("NewOrderClient returned error: %v", err)
			}
			if _, err := orderClient.CancelReplace(context.Background(), CancelReplaceParams{
				Cancel: CancelByOrderID("old"), Mode: CancelReplaceStopOnFailure,
				Replacement: CancelReplaceOrderParams{MarketSlug: "market", OrderType: OrderTypeGTC,
					Args: GTCOrderArgs{TokenID: "123", Side: SideBuy, Price: .5, Size: 1}},
			}); err != nil {
				t.Fatalf("CancelReplace returned error: %v", err)
			}
		})
	}
}

func TestDelegatedOrderService_CreateOrder_FeeRateByVenue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		venueJSON string
		requested int
		want      int
	}{
		{name: "collateral fee venue ignores requested rate", venueJSON: collateralTestVenueJSON, requested: 250, want: 0},
		{name: "collateral fee venue ignores default rate", venueJSON: collateralTestVenueJSON, requested: 0, want: 0},
		{name: "legacy venue keeps requested rate", venueJSON: legacyTestVenueJSON, requested: 250, want: 250},
		{name: "legacy venue keeps default rate", venueJSON: legacyTestVenueJSON, requested: 0, want: defaultDelegatedFeeRateBps},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mux := http.NewServeMux()
			handleTestMarket(t, mux, "test-market", tc.venueJSON)
			mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				var payload CreateOrderRequest
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("failed to decode delegated order payload: %v", err)
				}
				if payload.Order.FeeRateBps != tc.want {
					t.Fatalf("expected feeRateBps %d, got %d", tc.want, payload.Order.FeeRateBps)
				}
				writeTestOrderResponse(w, payload.Order, payload.OrderType)
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			service := NewDelegatedOrderService(NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("api-key")))
			if _, err := service.CreateOrder(context.Background(), CreateDelegatedOrderParams{
				MarketSlug: "test-market",
				OrderType:  OrderTypeGTC,
				OnBehalfOf: 42,
				FeeRateBps: tc.requested,
				Args:       GTCOrderArgs{TokenID: "123", Side: SideBuy, Price: 0.5, Size: 1},
			}); err != nil {
				t.Fatalf("CreateOrder returned error: %v", err)
			}
		})
	}
}

func TestDelegatedOrderService_CancelReplace_CollateralFeeVenueZeroFeeRate(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handleTestMarket(t, mux, "market", collateralTestVenueJSON)
	mux.HandleFunc("/orders/cancel-replace", func(w http.ResponseWriter, r *http.Request) {
		var request CancelReplaceRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("failed to decode cancel-replace request: %v", err)
		}
		if request.Replacement.Order.FeeRateBps != 0 {
			t.Fatalf("expected feeRateBps 0 on collateral fee venue, got %d", request.Replacement.Order.FeeRateBps)
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(cancelReplaceFailureResponse))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	service := NewDelegatedOrderService(NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("key")))
	if _, err := service.CancelReplace(context.Background(), DelegatedCancelReplaceParams{
		Cancel: CancelByOrderID("old"), Mode: CancelReplaceStopOnFailure, OnBehalfOf: 42, FeeRateBps: 250,
		Replacement: CancelReplaceOrderParams{MarketSlug: "market", OrderType: OrderTypeGTC,
			Args: GTCOrderArgs{TokenID: "123", Side: SideBuy, Price: .5, Size: 1}},
	}); err != nil {
		t.Fatalf("CancelReplace returned error: %v", err)
	}
}

func TestClient_DelegatedOrdersReuseCachedVenue(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	marketHits := handleTestMarket(t, mux, "test-market", collateralTestVenueJSON)
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		var payload CreateOrderRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode delegated order payload: %v", err)
		}
		if payload.Order.FeeRateBps != 0 {
			t.Fatalf("expected feeRateBps 0 on collateral fee venue, got %d", payload.Order.FeeRateBps)
		}
		writeTestOrderResponse(w, payload.Order, payload.OrderType)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewClient(WithBaseURL(srv.URL), WithAPIKey("api-key"))
	if _, err := client.Markets.GetMarket(context.Background(), "test-market"); err != nil {
		t.Fatalf("GetMarket returned error: %v", err)
	}
	if _, err := client.DelegatedOrders.CreateOrder(context.Background(), CreateDelegatedOrderParams{
		MarketSlug: "test-market",
		OrderType:  OrderTypeGTC,
		OnBehalfOf: 42,
		Args:       GTCOrderArgs{TokenID: "123", Side: SideBuy, Price: 0.5, Size: 1},
	}); err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if got := atomic.LoadInt32(marketHits); got != 1 {
		t.Fatalf("expected the delegated order to reuse the cached venue, got %d market request(s)", got)
	}
}
