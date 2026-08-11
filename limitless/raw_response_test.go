package limitless

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// hmacTestSecret is a valid base64-encoded secret reused across raw-response tests.
const hmacTestSecret = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="

// writeRawJSON writes a JSON body with a known status and an X-Request-Id header
// so tests can assert that raw responses surface status, headers, and body.
func writeRawJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("X-Request-Id", "req-123")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func newRawTestClient(t *testing.T, mux *http.ServeMux) *Client {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewClient(
		WithBaseURL(srv.URL),
		WithHMACCredentials(HMACCredentials{TokenID: "token-1", Secret: hmacTestSecret}),
	)
}

// TestWithRawResponse_AcrossServices exercises a representative ...WithRawResponse
// method per service and asserts Raw.Status, a custom response header, and Data.
func TestWithRawResponse_AcrossServices(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/markets/test-slug", func(w http.ResponseWriter, r *http.Request) {
		writeRawJSON(w, http.StatusOK, `{"slug":"test-slug","title":"Test Market"}`)
	})
	mux.HandleFunc("/navigation", func(w http.ResponseWriter, r *http.Request) {
		writeRawJSON(w, http.StatusOK, `[]`)
	})
	mux.HandleFunc("/profiles/me", func(w http.ResponseWriter, r *http.Request) {
		writeRawJSON(w, http.StatusOK, `{"id":42,"account":"0xabc"}`)
	})
	mux.HandleFunc("/auth/api-tokens", func(w http.ResponseWriter, r *http.Request) {
		writeRawJSON(w, http.StatusOK, `[{"tokenId":"t-1"}]`)
	})
	mux.HandleFunc("/profiles/partner-accounts/123/allowances", func(w http.ResponseWriter, r *http.Request) {
		writeRawJSON(w, http.StatusOK, `{"profileId":123,"ready":true}`)
	})
	mux.HandleFunc("/orders/order-1", func(w http.ResponseWriter, r *http.Request) {
		writeRawJSON(w, http.StatusOK, `{"message":"cancelled"}`)
	})
	mux.HandleFunc("/portfolio/split", func(w http.ResponseWriter, r *http.Request) {
		writeRawJSON(w, http.StatusCreated, `{"conditionId":"0xabc","operation":"split","hash":"0xdead"}`)
	})

	sdk := newRawTestClient(t, mux)
	ctx := context.Background()

	// MarketFetcher
	market, err := sdk.Markets.GetMarketWithRawResponse(ctx, "test-slug")
	if err != nil {
		t.Fatalf("GetMarketWithRawResponse error: %v", err)
	}
	if market.Raw.Status != http.StatusOK {
		t.Fatalf("expected market status 200, got %d", market.Raw.Status)
	}
	if got := market.Raw.Headers.Get("X-Request-Id"); got != "req-123" {
		t.Fatalf("expected X-Request-Id req-123, got %q", got)
	}
	if market.Data.Title != "Test Market" {
		t.Fatalf("expected market title, got %q", market.Data.Title)
	}

	// MarketPageFetcher
	nav, err := sdk.Pages.GetNavigationWithRawResponse(ctx)
	if err != nil {
		t.Fatalf("GetNavigationWithRawResponse error: %v", err)
	}
	if nav.Raw.Status != http.StatusOK || nav.Raw.Headers.Get("X-Request-Id") != "req-123" {
		t.Fatalf("unexpected navigation raw: status=%d header=%q", nav.Raw.Status, nav.Raw.Headers.Get("X-Request-Id"))
	}

	// PortfolioFetcher
	profile, err := sdk.Portfolio.GetCurrentProfileWithRawResponse(ctx)
	if err != nil {
		t.Fatalf("GetCurrentProfileWithRawResponse error: %v", err)
	}
	if profile.Raw.Status != http.StatusOK {
		t.Fatalf("expected profile status 200, got %d", profile.Raw.Status)
	}
	if profile.Data.ID != 42 || profile.Data.Account != "0xabc" {
		t.Fatalf("unexpected profile data: %+v", profile.Data)
	}

	// ApiTokenService
	tokens, err := sdk.ApiTokens.ListTokensWithRawResponse(ctx)
	if err != nil {
		t.Fatalf("ListTokensWithRawResponse error: %v", err)
	}
	if tokens.Raw.Status != http.StatusOK || len(tokens.Data) != 1 {
		t.Fatalf("unexpected tokens: status=%d len=%d", tokens.Raw.Status, len(tokens.Data))
	}

	// PartnerAccountService
	allowances, err := sdk.PartnerAccounts.CheckAllowancesWithRawResponse(ctx, 123)
	if err != nil {
		t.Fatalf("CheckAllowancesWithRawResponse error: %v", err)
	}
	if allowances.Raw.Status != http.StatusOK || !allowances.Data.Ready {
		t.Fatalf("unexpected allowances: status=%d ready=%v", allowances.Raw.Status, allowances.Data.Ready)
	}

	// DelegatedOrderService
	cancel, err := sdk.DelegatedOrders.CancelWithRawResponse(ctx, "order-1")
	if err != nil {
		t.Fatalf("CancelWithRawResponse error: %v", err)
	}
	if cancel.Raw.Status != http.StatusOK || cancel.Data.Message != "cancelled" {
		t.Fatalf("unexpected cancel: status=%d message=%q", cancel.Raw.Status, cancel.Data.Message)
	}

	// ServerWalletService (also preserves RawJSON on the decoded value)
	split, err := sdk.ServerWallets.SplitPositionsWithRawResponse(ctx, SplitServerWalletParams{
		ConditionID: "0x" + repeatHex(64),
		Amount:      "1000000",
		Venue:       &ServerWalletVenue{Exchange: "0x1111111111111111111111111111111111111111"},
		OnBehalfOf:  123,
	})
	if err != nil {
		t.Fatalf("SplitPositionsWithRawResponse error: %v", err)
	}
	if split.Raw.Status != http.StatusCreated {
		t.Fatalf("expected split status 201, got %d", split.Raw.Status)
	}
	if split.Data.Operation != "split" {
		t.Fatalf("expected operation split, got %q", split.Data.Operation)
	}
	if len(split.Data.RawJSON()) == 0 {
		t.Fatal("expected split RawJSON to be preserved on the decoded value")
	}
}

// TestOrderClientCancelWithRawResponse covers the OrderClient raw surface without
// requiring signing or profile bootstrapping.
func TestOrderClientCancelWithRawResponse(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/orders/all/market-1", func(w http.ResponseWriter, r *http.Request) {
		writeRawJSON(w, http.StatusOK, `{"message":"all cancelled"}`)
	})

	sdk := newRawTestClient(t, mux)
	oc, err := sdk.NewOrderClient("0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d")
	if err != nil {
		t.Fatalf("NewOrderClient error: %v", err)
	}

	result, err := oc.CancelAllWithRawResponse(context.Background(), "market-1")
	if err != nil {
		t.Fatalf("CancelAllWithRawResponse error: %v", err)
	}
	if result.Raw.Status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", result.Raw.Status)
	}
	if result.Data.Message != "all cancelled" {
		t.Fatalf("expected message, got %q", result.Data.Message)
	}
}

// TestAMMRawResponses asserts AMM buy/sell land as 201, allowance check as 200,
// and approve as 202 in Raw.Status, that Data decodes, and that the base
// (non-raw) method returns the same *T value unchanged (regression).
func TestAMMRawResponses(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/amm/allowances/check", func(w http.ResponseWriter, r *http.Request) {
		writeRawJSON(w, http.StatusOK, `{"status":"confirmed","confirmed":true,"market":"m","side":"BUY"}`)
	})
	mux.HandleFunc("/amm/allowances/approve", func(w http.ResponseWriter, r *http.Request) {
		writeRawJSON(w, http.StatusAccepted, `{"status":"submitted","confirmed":false,"market":"m","side":"BUY"}`)
	})
	mux.HandleFunc("/amm/buy", func(w http.ResponseWriter, r *http.Request) {
		writeRawJSON(w, http.StatusCreated, `{"status":"SUBMITTED","market":"m","outcomeIndex":0,"collateralAmount":"1000000","expectedShares":"900000","minShares":"800000"}`)
	})
	mux.HandleFunc("/amm/sell", func(w http.ResponseWriter, r *http.Request) {
		writeRawJSON(w, http.StatusCreated, `{"status":"SUBMITTED","market":"m","outcomeIndex":1,"collateralReturnAmount":"1000000","expectedShares":"900000","maxShares":"1100000"}`)
	})

	sdk := newRawTestClient(t, mux)
	ctx := context.Background()

	allowanceParams := AMMAllowanceParams{Market: "m", Side: AMMAllowanceSideBuy, OnBehalfOf: 123}
	check, err := sdk.AMM.CheckAllowanceWithRawResponse(ctx, allowanceParams)
	if err != nil {
		t.Fatalf("CheckAllowanceWithRawResponse error: %v", err)
	}
	if check.Raw.Status != http.StatusOK {
		t.Fatalf("expected check status 200, got %d", check.Raw.Status)
	}
	if check.Raw.Headers.Get("X-Request-Id") != "req-123" {
		t.Fatalf("expected X-Request-Id on check raw")
	}
	if !check.Data.Confirmed {
		t.Fatalf("expected confirmed allowance")
	}

	approve, err := sdk.AMM.ApproveAllowanceWithRawResponse(ctx, allowanceParams)
	if err != nil {
		t.Fatalf("ApproveAllowanceWithRawResponse error: %v", err)
	}
	if approve.Raw.Status != http.StatusAccepted {
		t.Fatalf("expected approve status 202, got %d", approve.Raw.Status)
	}

	buyParams := AMMBuyParams{
		Market:           "m",
		OutcomeIndex:     AMMOutcomeYes,
		CollateralAmount: "1000000",
		IdempotencyKey:   "buy-key-1",
		OnBehalfOf:       123,
	}
	buy, err := sdk.AMM.BuyWithRawResponse(ctx, buyParams)
	if err != nil {
		t.Fatalf("BuyWithRawResponse error: %v", err)
	}
	if buy.Raw.Status != http.StatusCreated {
		t.Fatalf("expected buy status 201, got %d", buy.Raw.Status)
	}
	if buy.Data.Status != AMMTradeStatusSubmitted || buy.Data.ExpectedShares != "900000" {
		t.Fatalf("unexpected buy data: %+v", buy.Data)
	}

	sellParams := AMMSellParams{
		Market:                 "m",
		OutcomeIndex:           AMMOutcomeNo,
		CollateralReturnAmount: "1000000",
		IdempotencyKey:         "sell-key-1",
		OnBehalfOf:             123,
	}
	sell, err := sdk.AMM.SellWithRawResponse(ctx, sellParams)
	if err != nil {
		t.Fatalf("SellWithRawResponse error: %v", err)
	}
	if sell.Raw.Status != http.StatusCreated {
		t.Fatalf("expected sell status 201, got %d", sell.Raw.Status)
	}

	// Regression: the base (non-raw) method returns *T unchanged.
	baseBuy, err := sdk.AMM.Buy(ctx, buyParams)
	if err != nil {
		t.Fatalf("Buy error: %v", err)
	}
	if baseBuy == nil {
		t.Fatal("expected base Buy to return a non-nil *AMMBuyResponse")
	}
	if baseBuy.Status != buy.Data.Status ||
		baseBuy.ExpectedShares != buy.Data.ExpectedShares ||
		baseBuy.MinShares != buy.Data.MinShares {
		t.Fatalf("base Buy result diverged from raw variant: base=%+v raw=%+v", *baseBuy, buy.Data)
	}
}

// TestDecodeRawResult_EmptyAndNil covers helper edge cases.
func TestDecodeRawResult_EmptyAndNil(t *testing.T) {
	t.Parallel()

	if _, err := decodeRawResult[map[string]any](nil, nil); err == nil {
		t.Fatal("expected error for nil raw response")
	}

	sentinel := &APIError{Status: 500}
	if _, err := decodeRawResult[map[string]any](nil, sentinel); err != sentinel {
		t.Fatalf("expected passthrough error, got %v", err)
	}

	result, err := decodeRawResult[map[string]any](&RawResponse{Status: 204, Body: json.RawMessage(nil)}, nil)
	if err != nil {
		t.Fatalf("unexpected error for empty body: %v", err)
	}
	if result.Raw.Status != 204 {
		t.Fatalf("expected status 204, got %d", result.Raw.Status)
	}
}

// repeatHex returns a string of n hex characters for building test condition IDs.
func repeatHex(n int) string {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = 'a'
	}
	return string(buf)
}
