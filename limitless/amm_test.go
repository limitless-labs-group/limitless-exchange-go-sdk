package limitless

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const ammTestHMACSecret = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="

func TestAMMService_AuthenticationModes(t *testing.T) {
	t.Run("HMAC signs every AMM request", func(t *testing.T) {
		var requests []*http.Request
		var bodies [][]byte
		transport := ammRoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			requests = append(requests, r.Clone(r.Context()))
			bodies = append(bodies, body)
			return ammHTTPResponse(r, http.StatusOK, `{}`), nil
		})

		client := NewHttpClient(
			WithBaseURL("https://amm.test"),
			WithAPIKey("legacy-key-must-not-be-used"),
			WithHMACCredentials(HMACCredentials{TokenID: "amm-token", Secret: ammTestHMACSecret}),
			WithTransport(transport),
		)
		service := NewAMMService(client)
		ctx := context.Background()

		if _, err := service.CheckAllowance(ctx, AMMAllowanceParams{Market: "market", Side: AMMAllowanceSideBuy}); err != nil {
			t.Fatalf("CheckAllowance returned error: %v", err)
		}
		if _, err := service.ApproveAllowance(ctx, AMMAllowanceParams{Market: "market", Side: AMMAllowanceSideSell}); err != nil {
			t.Fatalf("ApproveAllowance returned error: %v", err)
		}
		if _, err := service.Buy(ctx, validAMMBuyParams()); err != nil {
			t.Fatalf("Buy returned error: %v", err)
		}
		if _, err := service.Sell(ctx, validAMMSellParams()); err != nil {
			t.Fatalf("Sell returned error: %v", err)
		}

		wantPaths := []string{
			"/amm/allowances/check",
			"/amm/allowances/approve",
			"/amm/buy",
			"/amm/sell",
		}
		if len(requests) != len(wantPaths) {
			t.Fatalf("expected %d requests, got %d", len(wantPaths), len(requests))
		}
		for i, request := range requests {
			if request.Method != http.MethodPost || request.URL.RequestURI() != wantPaths[i] {
				t.Errorf("request %d: expected POST %s, got %s %s", i, wantPaths[i], request.Method, request.URL.RequestURI())
			}
			if got := request.Header.Get("lmts-api-key"); got != "amm-token" {
				t.Errorf("request %d: expected HMAC token header, got %q", i, got)
			}
			if got := request.Header.Get("Identity"); got != "" {
				t.Errorf("request %d: expected no identity header, got %q", i, got)
			}
			if got := request.Header.Get("X-API-Key"); got != "" {
				t.Errorf("request %d: expected legacy API key to be suppressed, got %q", i, got)
			}
			timestamp := request.Header.Get("lmts-timestamp")
			if timestamp == "" {
				t.Errorf("request %d: missing HMAC timestamp", i)
				continue
			}
			wantSignature, err := computeHMACSignature(
				ammTestHMACSecret,
				timestamp,
				request.Method,
				request.URL.RequestURI(),
				string(bodies[i]),
			)
			if err != nil {
				t.Fatalf("request %d: compute expected HMAC signature: %v", i, err)
			}
			if got := request.Header.Get("lmts-signature"); got != wantSignature {
				t.Errorf("request %d: expected signature %q, got %q", i, wantSignature, got)
			}
		}
	})

	t.Run("identity overrides configured HMAC auth", func(t *testing.T) {
		var requests []*http.Request
		transport := ammRoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			requests = append(requests, r.Clone(r.Context()))
			return ammHTTPResponse(r, http.StatusOK, `{}`), nil
		})

		client := NewHttpClient(
			WithBaseURL("https://amm.test"),
			WithHMACCredentials(HMACCredentials{TokenID: "unused-token", Secret: ammTestHMACSecret}),
			WithTransport(transport),
		)
		service := NewAMMService(client)
		ctx := context.Background()

		if _, err := service.CheckAllowanceWithIdentity(ctx, "  privy-token  ", AMMAllowanceParams{Market: "market", Side: AMMAllowanceSideBuy}); err != nil {
			t.Fatalf("CheckAllowanceWithIdentity returned error: %v", err)
		}
		if _, err := service.ApproveAllowanceWithIdentity(ctx, "privy-token", AMMAllowanceParams{Market: "market", Side: AMMAllowanceSideSell}); err != nil {
			t.Fatalf("ApproveAllowanceWithIdentity returned error: %v", err)
		}
		if _, err := service.BuyWithIdentity(ctx, "privy-token", validAMMBuyParams()); err != nil {
			t.Fatalf("BuyWithIdentity returned error: %v", err)
		}
		if _, err := service.SellWithIdentity(ctx, "privy-token", validAMMSellParams()); err != nil {
			t.Fatalf("SellWithIdentity returned error: %v", err)
		}

		if len(requests) != 4 {
			t.Fatalf("expected 4 requests, got %d", len(requests))
		}
		for i, request := range requests {
			if got := request.Header.Get("Identity"); got != "Bearer privy-token" {
				t.Errorf("request %d: expected trimmed Privy identity token, got %q", i, got)
			}
			for _, header := range []string{"lmts-api-key", "lmts-timestamp", "lmts-signature", "X-API-Key"} {
				if got := request.Header.Get(header); got != "" {
					t.Errorf("request %d: expected %s to be absent, got %q", i, header, got)
				}
			}
		}
	})
}

func TestAMMService_CheckAndApproveAllowanceMapping(t *testing.T) {
	checkCalls := 0
	approveCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/amm/allowances/check", func(w http.ResponseWriter, r *http.Request) {
		checkCalls++
		var request map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode allowance check: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if checkCalls == 1 {
			if got := string(request["side"]); got != `"BUY"` {
				t.Errorf("expected BUY check, got %s", got)
			}
			ammWriteJSON(t, w, http.StatusOK, map[string]any{
				"status":            "missing",
				"confirmed":         false,
				"market":            "market-slug",
				"marketAddress":     "0xfpmm",
				"side":              "BUY",
				"walletAddress":     "0xwallet",
				"tokenAddress":      "0xtoken",
				"spenderOrOperator": "0xfpmm",
				"currentAllowance":  "12345678901234567890",
			})
			return
		}
		if _, present := request["onBehalfOf"]; present {
			t.Errorf("expected onBehalfOf to be omitted for a direct profile")
		}
		ammWriteJSON(t, w, http.StatusOK, map[string]any{
			"status":            "confirmed",
			"confirmed":         true,
			"market":            "market-slug",
			"marketAddress":     "0xfpmm",
			"side":              "SELL",
			"walletAddress":     "0xwallet",
			"tokenAddress":      "0xconditionalTokens",
			"spenderOrOperator": "0xfpmm",
		})
	})
	mux.HandleFunc("/amm/allowances/approve", func(w http.ResponseWriter, r *http.Request) {
		approveCalls++
		if approveCalls == 1 {
			ammWriteJSON(t, w, http.StatusOK, map[string]any{
				"status":    "confirmed",
				"confirmed": true,
				"market":    "market-slug",
				"side":      "BUY",
			})
			return
		}
		ammWriteJSON(t, w, http.StatusAccepted, map[string]any{
			"status":            "submitted",
			"confirmed":         false,
			"market":            "market-slug",
			"marketAddress":     "0xfpmm",
			"side":              "SELL",
			"walletAddress":     "0xwallet",
			"tokenAddress":      "0xconditionalTokens",
			"spenderOrOperator": "0xfpmm",
			"transactionId":     "approval-transaction",
		})
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	service := newHMACAMMTestService(server.URL)
	ctx := context.Background()

	buyCheck, err := service.CheckAllowance(ctx, AMMAllowanceParams{
		Market:     "market-slug",
		Side:       AMMAllowanceSideBuy,
		OnBehalfOf: 436,
	})
	if err != nil {
		t.Fatalf("BUY CheckAllowance returned error: %v", err)
	}
	if buyCheck.Status != AMMAllowanceStatusMissing || buyCheck.Confirmed {
		t.Fatalf("unexpected BUY check state: %+v", buyCheck)
	}
	if buyCheck.CurrentAllowance == nil || *buyCheck.CurrentAllowance != "12345678901234567890" {
		t.Fatalf("expected string current allowance, got %v", buyCheck.CurrentAllowance)
	}

	sellCheck, err := service.CheckAllowance(ctx, AMMAllowanceParams{Market: "market-slug", Side: AMMAllowanceSideSell})
	if err != nil {
		t.Fatalf("SELL CheckAllowance returned error: %v", err)
	}
	if sellCheck.Status != AMMAllowanceStatusConfirmed || !sellCheck.Confirmed {
		t.Fatalf("unexpected SELL check state: %+v", sellCheck)
	}
	if sellCheck.CurrentAllowance != nil {
		t.Fatalf("expected SELL current allowance to be omitted, got %q", *sellCheck.CurrentAllowance)
	}

	alreadyApproved, err := service.ApproveAllowance(ctx, AMMAllowanceParams{Market: "market-slug", Side: AMMAllowanceSideBuy})
	if err != nil {
		t.Fatalf("confirmed ApproveAllowance returned error: %v", err)
	}
	if alreadyApproved.Status != AMMAllowanceStatusConfirmed || !alreadyApproved.Confirmed {
		t.Fatalf("unexpected HTTP 200 approve mapping: %+v", alreadyApproved)
	}

	submitted, err := service.ApproveAllowance(ctx, AMMAllowanceParams{Market: "market-slug", Side: AMMAllowanceSideSell})
	if err != nil {
		t.Fatalf("submitted ApproveAllowance returned error: %v", err)
	}
	if submitted.Status != AMMAllowanceStatusSubmitted || submitted.Confirmed {
		t.Fatalf("unexpected HTTP 202 approve mapping: %+v", submitted)
	}
	if submitted.TransactionID == nil || *submitted.TransactionID != "approval-transaction" {
		t.Fatalf("expected approval transaction identifier, got %v", submitted.TransactionID)
	}
}

func TestAMMService_EnsureAllowanceApprovesOnceAndPollsCheck(t *testing.T) {
	checkCalls := 0
	approveCalls := 0
	sequence := make([]string, 0, 4)
	mux := http.NewServeMux()
	mux.HandleFunc("/amm/allowances/check", func(w http.ResponseWriter, r *http.Request) {
		checkCalls++
		sequence = append(sequence, "check")
		confirmed := checkCalls == 3
		status := AMMAllowanceStatusMissing
		if confirmed {
			status = AMMAllowanceStatusConfirmed
		}
		ammWriteJSON(t, w, http.StatusOK, map[string]any{
			"status":    status,
			"confirmed": confirmed,
			"market":    "market-slug",
			"side":      "BUY",
		})
	})
	mux.HandleFunc("/amm/allowances/approve", func(w http.ResponseWriter, r *http.Request) {
		approveCalls++
		sequence = append(sequence, "approve")
		ammWriteJSON(t, w, http.StatusAccepted, map[string]any{
			"status":        "submitted",
			"confirmed":     false,
			"market":        "market-slug",
			"side":          "BUY",
			"transactionId": "approval-1",
		})
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	service := newHMACAMMTestService(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := service.EnsureAllowance(ctx, AMMAllowanceParams{
		Market:     "market-slug",
		Side:       AMMAllowanceSideBuy,
		OnBehalfOf: 436,
	}, AMMAllowancePollOptions{Interval: time.Millisecond})
	if err != nil {
		t.Fatalf("EnsureAllowance returned error: %v", err)
	}
	if !result.Confirmed || result.Status != AMMAllowanceStatusConfirmed {
		t.Fatalf("expected confirmed allowance, got %+v", result)
	}
	if approveCalls != 1 {
		t.Fatalf("expected exactly one approve call, got %d", approveCalls)
	}
	if checkCalls != 3 {
		t.Fatalf("expected initial check and two poll checks, got %d", checkCalls)
	}
	if got, want := strings.Join(sequence, ","), "check,approve,check,check"; got != want {
		t.Fatalf("expected request sequence %q, got %q", want, got)
	}
}

func TestAMMService_EnsureAllowanceWithIdentityReturnsConfirmedCheck(t *testing.T) {
	requests := 0
	transport := ammRoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.Path != "/amm/allowances/check" {
			t.Fatalf("expected only an allowance check, got %q", r.URL.Path)
		}
		if got := r.Header.Get("Identity"); got != "Bearer privy-token" {
			t.Fatalf("expected Privy identity header, got %q", got)
		}
		return ammHTTPResponse(r, http.StatusOK, `{"status":"confirmed","confirmed":true,"market":"market-slug","side":"SELL"}`), nil
	})
	service := NewAMMService(NewHttpClient(
		WithBaseURL("https://amm.test"),
		WithTransport(transport),
	))

	result, err := service.EnsureAllowanceWithIdentity(
		context.Background(),
		"privy-token",
		AMMAllowanceParams{Market: "market-slug", Side: AMMAllowanceSideSell},
	)
	if err != nil {
		t.Fatalf("EnsureAllowanceWithIdentity returned error: %v", err)
	}
	if result == nil || !result.Confirmed || result.Status != AMMAllowanceStatusConfirmed {
		t.Fatalf("expected confirmed allowance, got %+v", result)
	}
	if requests != 1 {
		t.Fatalf("expected exactly one check and no approval, got %d requests", requests)
	}
}

func TestAMMService_BuySellPayloadsDoNotPreflightAllowances(t *testing.T) {
	zeroSlippage := 0
	var paths []string
	var bodies [][]byte
	transport := ammRoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read AMM trade body: %v", err)
		}
		paths = append(paths, r.URL.Path)
		bodies = append(bodies, body)
		switch r.URL.Path {
		case "/amm/buy":
			return ammHTTPResponse(r, http.StatusCreated, `{"status":"SUBMITTED","market":"market-slug","outcomeIndex":1,"collateralAmount":"1000000","expectedShares":"1763995","minShares":"1746355"}`), nil
		case "/amm/sell":
			return ammHTTPResponse(r, http.StatusCreated, `{"status":"SUBMITTED","market":"market-slug","outcomeIndex":0,"collateralReturnAmount":"992015","expectedShares":"1959992","maxShares":"1979592"}`), nil
		default:
			t.Fatalf("unexpected allowance preflight or trade path %q", r.URL.Path)
			return nil, nil
		}
	})

	service := NewAMMService(NewHttpClient(
		WithBaseURL("https://amm.test"),
		WithHMACCredentials(HMACCredentials{TokenID: "amm-token", Secret: ammTestHMACSecret}),
		WithTransport(transport),
	))

	buy, err := service.Buy(context.Background(), AMMBuyParams{
		Market:           "market-slug",
		OutcomeIndex:     AMMOutcomeNo,
		CollateralAmount: "1000000",
		SlippageBps:      &zeroSlippage,
		IdempotencyKey:   "buy-key",
		OnBehalfOf:       436,
	})
	if err != nil {
		t.Fatalf("Buy returned error: %v", err)
	}
	if buy.CollateralAmount != "1000000" || buy.MinShares != "1746355" {
		t.Fatalf("unexpected buy mapping: %+v", buy)
	}

	sell, err := service.Sell(context.Background(), AMMSellParams{
		Market:                 "market-slug",
		OutcomeIndex:           AMMOutcomeYes,
		CollateralReturnAmount: "992015",
		SlippageBps:            nil,
		IdempotencyKey:         "sell-key",
		OnBehalfOf:             0,
	})
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}
	if sell.CollateralReturnAmount != "992015" || sell.MaxShares != "1979592" {
		t.Fatalf("unexpected sell mapping: %+v", sell)
	}

	if got, want := strings.Join(paths, ","), "/amm/buy,/amm/sell"; got != want {
		t.Fatalf("expected only direct buy and sell calls %q, got %q", want, got)
	}
	wantBuy := `{"market":"market-slug","outcomeIndex":1,"collateralAmount":"1000000","slippageBps":0,"idempotencyKey":"buy-key","onBehalfOf":436}`
	if got := string(bodies[0]); got != wantBuy {
		t.Errorf("unexpected buy body:\nwant %s\n got %s", wantBuy, got)
	}
	wantSell := `{"market":"market-slug","outcomeIndex":0,"collateralReturnAmount":"992015","idempotencyKey":"sell-key"}`
	if got := string(bodies[1]); got != wantSell {
		t.Errorf("unexpected sell body:\nwant %s\n got %s", wantSell, got)
	}
}

func TestAMMService_OptionalTransactionIdentifiers(t *testing.T) {
	responses := []string{
		`{"status":"missing","confirmed":false,"transactionId":"transaction-only"}`,
		`{"status":"SUBMITTED","transactionId":"transaction-only"}`,
		`{"status":"SUBMITTED","userOperationHash":"user-operation-only"}`,
		`{"status":"SUBMITTED","txHash":"tx-only"}`,
		`{"status":"SUBMITTED"}`,
	}
	call := 0
	transport := ammRoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		response := responses[call]
		call++
		return ammHTTPResponse(r, http.StatusOK, response), nil
	})
	service := NewAMMService(NewHttpClient(
		WithBaseURL("https://amm.test"),
		WithHMACCredentials(HMACCredentials{TokenID: "amm-token", Secret: ammTestHMACSecret}),
		WithTransport(transport),
	))

	allowance, err := service.CheckAllowance(context.Background(), AMMAllowanceParams{Market: "market", Side: AMMAllowanceSideBuy})
	if err != nil {
		t.Fatalf("CheckAllowance returned error: %v", err)
	}
	assertAMMIdentifiers(t, allowance.AMMTransactionIdentifiers, "transaction-only", "", "")

	buy, err := service.Buy(context.Background(), validAMMBuyParams())
	if err != nil {
		t.Fatalf("Buy with transactionId returned error: %v", err)
	}
	assertAMMIdentifiers(t, buy.AMMTransactionIdentifiers, "transaction-only", "", "")

	buy, err = service.Buy(context.Background(), validAMMBuyParams())
	if err != nil {
		t.Fatalf("Buy with userOperationHash returned error: %v", err)
	}
	assertAMMIdentifiers(t, buy.AMMTransactionIdentifiers, "", "user-operation-only", "")

	sell, err := service.Sell(context.Background(), validAMMSellParams())
	if err != nil {
		t.Fatalf("Sell with txHash returned error: %v", err)
	}
	assertAMMIdentifiers(t, sell.AMMTransactionIdentifiers, "", "", "tx-only")

	sell, err = service.Sell(context.Background(), validAMMSellParams())
	if err != nil {
		t.Fatalf("Sell without identifiers returned error: %v", err)
	}
	assertAMMIdentifiers(t, sell.AMMTransactionIdentifiers, "", "", "")
}

func TestAMMValidation_AmountStrings(t *testing.T) {
	const maxUint256 = "115792089237316195423570985008687907853269984665640564039457584007913129639935"
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "one", value: "1", valid: true},
		{name: "ordinary base unit amount", value: "1000000", valid: true},
		{name: "uint256 maximum", value: maxUint256, valid: true},
		{name: "zero", value: "0"},
		{name: "negative", value: "-1"},
		{name: "leading zero", value: "01"},
		{name: "decimal", value: "1.5"},
		{name: "scientific notation", value: "1e6"},
		{name: "surrounding whitespace", value: " 1 "},
		{name: "uint256 overflow", value: "115792089237316195423570985008687907853269984665640564039457584007913129639936"},
		{name: "too long", value: strings.Repeat("9", 79)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := validAMMBuyParams()
			params.CollateralAmount = test.value
			request, err := buildAMMBuyRequest(params)
			if test.valid {
				if err != nil {
					t.Fatalf("expected %q to be valid: %v", test.value, err)
				}
				if request.CollateralAmount != test.value {
					t.Fatalf("amount string changed: want %q, got %q", test.value, request.CollateralAmount)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected %q to be rejected", test.value)
			}
			if !strings.Contains(err.Error(), "CollateralAmount") {
				t.Fatalf("expected CollateralAmount validation error, got %v", err)
			}
		})
	}

	sell := validAMMSellParams()
	sell.CollateralReturnAmount = maxUint256
	request, err := buildAMMSellRequest(sell)
	if err != nil {
		t.Fatalf("expected uint256 maximum sell return amount to be valid: %v", err)
	}
	if request.CollateralReturnAmount != maxUint256 {
		t.Fatalf("sell amount string changed: want %q, got %q", maxUint256, request.CollateralReturnAmount)
	}
	sell.CollateralReturnAmount = "01"
	if _, err := buildAMMSellRequest(sell); err == nil || !strings.Contains(err.Error(), "CollateralReturnAmount") {
		t.Fatalf("expected leading-zero CollateralReturnAmount validation error, got %v", err)
	}
}

func TestAMMValidation_TradeBoundaries(t *testing.T) {
	t.Run("outcome index", func(t *testing.T) {
		for _, test := range []struct {
			name    string
			outcome AMMOutcomeIndex
			valid   bool
		}{
			{name: "YES lower boundary", outcome: AMMOutcomeYes, valid: true},
			{name: "NO upper boundary", outcome: AMMOutcomeNo, valid: true},
			{name: "negative", outcome: -1},
			{name: "above binary range", outcome: 2},
		} {
			t.Run(test.name, func(t *testing.T) {
				params := validAMMBuyParams()
				params.OutcomeIndex = test.outcome
				_, err := buildAMMBuyRequest(params)
				if (err == nil) != test.valid {
					t.Fatalf("outcome %d validity: expected valid=%v, got err=%v", test.outcome, test.valid, err)
				}
			})
		}
	})

	t.Run("slippage", func(t *testing.T) {
		for _, test := range []struct {
			name     string
			slippage *int
			valid    bool
		}{
			{name: "nil uses API default", slippage: nil, valid: true},
			{name: "zero lower boundary", slippage: ammIntPtr(0), valid: true},
			{name: "upper boundary", slippage: ammIntPtr(1000), valid: true},
			{name: "negative", slippage: ammIntPtr(-1)},
			{name: "above range", slippage: ammIntPtr(1001)},
		} {
			t.Run(test.name, func(t *testing.T) {
				params := validAMMBuyParams()
				params.SlippageBps = test.slippage
				request, err := buildAMMBuyRequest(params)
				if (err == nil) != test.valid {
					t.Fatalf("expected valid=%v, got err=%v", test.valid, err)
				}
				if err == nil {
					if test.slippage == nil && request.SlippageBps != nil {
						t.Fatal("expected nil slippage to remain omitted")
					}
					if test.slippage != nil && (request.SlippageBps == nil || *request.SlippageBps != *test.slippage) {
						t.Fatalf("expected explicit slippage %d, got %v", *test.slippage, request.SlippageBps)
					}
				}
			})
		}
	})

	t.Run("idempotency key", func(t *testing.T) {
		for _, test := range []struct {
			name  string
			key   string
			valid bool
		}{
			{name: "one character", key: "k", valid: true},
			{name: "128 ASCII characters", key: strings.Repeat("a", 128), valid: true},
			{name: "128 Unicode characters", key: strings.Repeat("é", 128), valid: true},
			{name: "empty", key: ""},
			{name: "whitespace", key: " \t\n "},
			{name: "129 characters", key: strings.Repeat("a", 129)},
		} {
			t.Run(test.name, func(t *testing.T) {
				params := validAMMBuyParams()
				params.IdempotencyKey = test.key
				_, err := buildAMMBuyRequest(params)
				if (err == nil) != test.valid {
					t.Fatalf("expected valid=%v, got err=%v", test.valid, err)
				}
			})
		}
	})

	t.Run("on behalf of", func(t *testing.T) {
		for _, test := range []struct {
			name       string
			onBehalfOf int
			valid      bool
		}{
			{name: "omitted zero", onBehalfOf: 0, valid: true},
			{name: "positive", onBehalfOf: 436, valid: true},
			{name: "signed 32 bit maximum", onBehalfOf: ammMaxOnBehalfOf, valid: true},
			{name: "negative", onBehalfOf: -1},
			{name: "above signed 32 bit maximum", onBehalfOf: ammMaxOnBehalfOf + 1},
		} {
			t.Run(test.name, func(t *testing.T) {
				params := validAMMBuyParams()
				params.OnBehalfOf = test.onBehalfOf
				_, err := buildAMMBuyRequest(params)
				if (err == nil) != test.valid {
					t.Fatalf("expected valid=%v, got err=%v", test.valid, err)
				}
			})
		}
	})

	t.Run("market", func(t *testing.T) {
		for _, test := range []struct {
			name   string
			market string
			valid  bool
			want   string
		}{
			{name: "trimmed", market: "  market-slug  ", valid: true, want: "market-slug"},
			{name: "255 characters", market: strings.Repeat("m", 255), valid: true, want: strings.Repeat("m", 255)},
			{name: "blank", market: " \t "},
			{name: "256 characters", market: strings.Repeat("m", 256)},
		} {
			t.Run(test.name, func(t *testing.T) {
				params := validAMMBuyParams()
				params.Market = test.market
				request, err := buildAMMBuyRequest(params)
				if (err == nil) != test.valid {
					t.Fatalf("expected valid=%v, got err=%v", test.valid, err)
				}
				if err == nil && request.Market != test.want {
					t.Fatalf("expected normalized market %q, got %q", test.want, request.Market)
				}
			})
		}
	})

	t.Run("allowance side", func(t *testing.T) {
		for _, test := range []struct {
			side  AMMAllowanceSide
			valid bool
		}{
			{side: AMMAllowanceSideBuy, valid: true},
			{side: AMMAllowanceSideSell, valid: true},
			{side: "buy"},
			{side: ""},
		} {
			_, err := buildAMMAllowanceRequest(AMMAllowanceParams{Market: "market", Side: test.side})
			if (err == nil) != test.valid {
				t.Errorf("side %q: expected valid=%v, got err=%v", test.side, test.valid, err)
			}
		}
	})
}

func TestAMMService_RejectsLegacyAPIKeyAndBlankIdentity(t *testing.T) {
	transportCalls := 0
	transport := ammRoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		transportCalls++
		return ammHTTPResponse(r, http.StatusOK, `{}`), nil
	})
	service := NewAMMService(NewHttpClient(
		WithBaseURL("https://amm.test"),
		WithAPIKey("legacy-key"),
		WithTransport(transport),
	))
	ctx := context.Background()

	legacyCalls := []struct {
		name string
		call func() error
	}{
		{name: "check", call: func() error {
			_, err := service.CheckAllowance(ctx, AMMAllowanceParams{Market: "market", Side: AMMAllowanceSideBuy})
			return err
		}},
		{name: "approve", call: func() error {
			_, err := service.ApproveAllowance(ctx, AMMAllowanceParams{Market: "market", Side: AMMAllowanceSideBuy})
			return err
		}},
		{name: "ensure", call: func() error {
			_, err := service.EnsureAllowance(ctx, AMMAllowanceParams{Market: "market", Side: AMMAllowanceSideBuy})
			return err
		}},
		{name: "buy", call: func() error {
			_, err := service.Buy(ctx, validAMMBuyParams())
			return err
		}},
		{name: "sell", call: func() error {
			_, err := service.Sell(ctx, validAMMSellParams())
			return err
		}},
	}
	for _, test := range legacyCalls {
		t.Run("legacy "+test.name, func(t *testing.T) {
			err := test.call()
			if err == nil || err.Error() != ammHMACOnlyError {
				t.Fatalf("expected legacy API key rejection %q, got %v", ammHMACOnlyError, err)
			}
		})
	}

	blankIdentityCalls := []struct {
		name string
		call func() error
	}{
		{name: "check", call: func() error {
			_, err := service.CheckAllowanceWithIdentity(ctx, " \t ", AMMAllowanceParams{Market: "market", Side: AMMAllowanceSideBuy})
			return err
		}},
		{name: "approve", call: func() error {
			_, err := service.ApproveAllowanceWithIdentity(ctx, "", AMMAllowanceParams{Market: "market", Side: AMMAllowanceSideBuy})
			return err
		}},
		{name: "ensure", call: func() error {
			_, err := service.EnsureAllowanceWithIdentity(ctx, " ", AMMAllowanceParams{Market: "market", Side: AMMAllowanceSideBuy})
			return err
		}},
		{name: "buy", call: func() error {
			_, err := service.BuyWithIdentity(ctx, "", validAMMBuyParams())
			return err
		}},
		{name: "sell", call: func() error {
			_, err := service.SellWithIdentity(ctx, "\n", validAMMSellParams())
			return err
		}},
	}
	for _, test := range blankIdentityCalls {
		t.Run("blank identity "+test.name, func(t *testing.T) {
			err := test.call()
			if err == nil || !strings.Contains(err.Error(), "identity token is required") {
				t.Fatalf("expected blank identity rejection, got %v", err)
			}
		})
	}

	if transportCalls != 0 {
		t.Fatalf("expected rejected auth to perform no HTTP calls, got %d", transportCalls)
	}
}

func TestAMMService_TimeoutRetryUsesByteIdenticalBody(t *testing.T) {
	var bodies [][]byte
	transport := ammRoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read retry request body: %v", err)
		}
		bodies = append(bodies, body)
		if len(bodies) == 1 {
			return nil, ammTimeoutError{}
		}
		return ammHTTPResponse(r, http.StatusCreated, `{"status":"SUBMITTED","market":"market-slug","outcomeIndex":0,"collateralAmount":"1000000","expectedShares":"1763995","minShares":"1746355","transactionId":"same-submission"}`), nil
	})

	service := NewAMMService(NewHttpClient(
		WithBaseURL("https://amm.test"),
		WithHMACCredentials(HMACCredentials{TokenID: "amm-token", Secret: ammTestHMACSecret}),
		WithTransport(transport),
	))
	params := AMMBuyParams{
		Market:           "market-slug",
		OutcomeIndex:     AMMOutcomeYes,
		CollateralAmount: "1000000",
		SlippageBps:      ammIntPtr(100),
		IdempotencyKey:   "stable-buy-key",
		OnBehalfOf:       436,
	}

	result, err := WithRetry(context.Background(), func() (*AMMBuyResponse, error) {
		return service.Buy(context.Background(), params)
	}, RetryConfig{
		MaxRetries: 1,
		Delays:     []time.Duration{0},
	})
	if err != nil {
		t.Fatalf("retrying Buy returned error: %v", err)
	}
	if result.TransactionID == nil || *result.TransactionID != "same-submission" {
		t.Fatalf("unexpected retry result: %+v", result)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected one timed-out request and one retry, got %d", len(bodies))
	}
	if !bytes.Equal(bodies[0], bodies[1]) {
		t.Fatalf("retry body changed:\nfirst  %s\nsecond %s", bodies[0], bodies[1])
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(bodies[1], &payload); err != nil {
		t.Fatalf("decode retried body: %v", err)
	}
	if got := string(payload["idempotencyKey"]); got != `"stable-buy-key"` {
		t.Fatalf("expected stable idempotency key, got %s", got)
	}
}

func TestAMMService_IdempotentReplayAndConflictMapping(t *testing.T) {
	var bodies [][]byte
	transport := ammRoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read idempotent request body: %v", err)
		}
		bodies = append(bodies, body)
		if len(bodies) < 3 {
			return ammHTTPResponse(r, http.StatusCreated, `{"status":"SUBMITTED","transactionId":"original-submission"}`), nil
		}
		return ammHTTPResponse(r, http.StatusConflict, `{"message":"idempotency key was already used with different parameters"}`), nil
	})
	service := NewAMMService(NewHttpClient(
		WithBaseURL("https://amm.test"),
		WithHMACCredentials(HMACCredentials{TokenID: "amm-token", Secret: ammTestHMACSecret}),
		WithTransport(transport),
	))
	params := validAMMBuyParams()
	params.IdempotencyKey = "replay-key"

	first, err := service.Buy(context.Background(), params)
	if err != nil {
		t.Fatalf("first Buy returned error: %v", err)
	}
	second, err := service.Buy(context.Background(), params)
	if err != nil {
		t.Fatalf("replayed Buy returned error: %v", err)
	}
	if first.TransactionID == nil || second.TransactionID == nil || *first.TransactionID != *second.TransactionID {
		t.Fatalf("expected replay to retain identifiers, got first=%+v second=%+v", first, second)
	}
	if !bytes.Equal(bodies[0], bodies[1]) {
		t.Fatalf("same params produced different bodies:\nfirst  %s\nsecond %s", bodies[0], bodies[1])
	}

	params.CollateralAmount = "2"
	_, err = service.Buy(context.Background(), params)
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected changed params with the same key to map to ConflictError, got %T (%v)", err, err)
	}
	if bytes.Equal(bodies[1], bodies[2]) {
		t.Fatalf("expected changed trade params to produce a different body, got %s", bodies[2])
	}
}

func TestAMMService_MapsEndpointErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		isType func(error) bool
	}{
		{name: "authentication", status: http.StatusForbidden, isType: func(err error) bool {
			var target *AuthenticationError
			return errors.As(err, &target)
		}},
		{name: "unprocessable entity", status: http.StatusUnprocessableEntity, isType: func(err error) bool {
			var target *UnprocessableEntityError
			return errors.As(err, &target)
		}},
		{name: "maintenance", status: http.StatusTooEarly, isType: func(err error) bool {
			var target *TooEarlyError
			return errors.As(err, &target)
		}},
		{name: "rate limit", status: http.StatusTooManyRequests, isType: func(err error) bool {
			var target *RateLimitError
			return errors.As(err, &target)
		}},
		{name: "bad gateway", status: http.StatusBadGateway, isType: func(err error) bool {
			var target *UpstreamUnavailableError
			return errors.As(err, &target)
		}},
		{name: "service unavailable", status: http.StatusServiceUnavailable, isType: func(err error) bool {
			var target *UpstreamUnavailableError
			return errors.As(err, &target)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transport := ammRoundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return ammHTTPResponse(r, test.status, `{"message":"AMM request rejected"}`), nil
			})
			service := NewAMMService(NewHttpClient(
				WithBaseURL("https://amm.test"),
				WithHMACCredentials(HMACCredentials{TokenID: "amm-token", Secret: ammTestHMACSecret}),
				WithTransport(transport),
			))

			_, err := service.Buy(context.Background(), validAMMBuyParams())
			if err == nil || !test.isType(err) {
				t.Fatalf("expected typed error for HTTP %d, got %T (%v)", test.status, err, err)
			}
		})
	}
}

func newHMACAMMTestService(baseURL string) *AMMService {
	return NewAMMService(NewHttpClient(
		WithBaseURL(baseURL),
		WithHMACCredentials(HMACCredentials{TokenID: "amm-token", Secret: ammTestHMACSecret}),
	))
}

func validAMMBuyParams() AMMBuyParams {
	return AMMBuyParams{
		Market:           "market",
		OutcomeIndex:     AMMOutcomeYes,
		CollateralAmount: "1",
		IdempotencyKey:   "buy-key",
	}
}

func validAMMSellParams() AMMSellParams {
	return AMMSellParams{
		Market:                 "market",
		OutcomeIndex:           AMMOutcomeNo,
		CollateralReturnAmount: "1",
		IdempotencyKey:         "sell-key",
	}
}

func ammIntPtr(value int) *int {
	return &value
}

func ammWriteJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode AMM test response: %v", err)
	}
}

func assertAMMIdentifiers(t *testing.T, identifiers AMMTransactionIdentifiers, transactionID, userOperationHash, txHash string) {
	t.Helper()
	assertOptionalAMMIdentifier(t, "transactionId", identifiers.TransactionID, transactionID)
	assertOptionalAMMIdentifier(t, "userOperationHash", identifiers.UserOperationHash, userOperationHash)
	assertOptionalAMMIdentifier(t, "txHash", identifiers.TxHash, txHash)
}

func assertOptionalAMMIdentifier(t *testing.T, name string, got *string, want string) {
	t.Helper()
	if want == "" {
		if got != nil {
			t.Errorf("expected %s to be omitted, got %q", name, *got)
		}
		return
	}
	if got == nil || *got != want {
		t.Errorf("expected %s %q, got %v", name, want, got)
	}
}

type ammRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f ammRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func ammHTTPResponse(request *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}

type ammTimeoutError struct{}

func (ammTimeoutError) Error() string   { return "AMM request timed out" }
func (ammTimeoutError) Timeout() bool   { return true }
func (ammTimeoutError) Temporary() bool { return true }
