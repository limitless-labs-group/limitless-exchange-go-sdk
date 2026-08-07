package limitless

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const cancelReplaceFailureResponse = `{"cancel":{"status":"FAILURE","error":{"code":"ORDER_NOT_FOUND","message":"not found"}},"replacement":{"status":"NOT_ATTEMPTED"}}`

func TestCancelTargetJSONAndResultValidation(t *testing.T) {
	t.Parallel()
	orderTarget, err := json.Marshal(CancelByOrderID("order-1"))
	if err != nil || string(orderTarget) != `{"orderId":"order-1"}` {
		t.Fatalf("unexpected order target: %s, %v", orderTarget, err)
	}
	clientTarget, err := json.Marshal(CancelByClientOrderID("client-1"))
	if err != nil || string(clientTarget) != `{"clientOrderId":"client-1"}` {
		t.Fatalf("unexpected client target: %s, %v", clientTarget, err)
	}
	if _, err := json.Marshal(CancelTarget{}); err == nil {
		t.Fatal("expected empty cancel target to fail")
	}

	invalid := []string{
		`{"cancel":{"status":"SUCCESS","orderId":"x","error":{"code":"x","message":"x"}},"replacement":{"status":"NOT_ATTEMPTED"}}`,
		`{"cancel":{"status":"FAILURE","error":{"code":"x","message":"x"}},"replacement":{"status":"SUCCESS","data":{"order":{}}}}`,
		`{"cancel":{"status":"UNKNOWN"},"replacement":{"status":"NOT_ATTEMPTED"}}`,
		`{"cancel":{"status":"FAILURE","error":{"code":"x","message":"x"}},"replacement":{"status":"NOT_ATTEMPTED","error":{"code":"x","message":"x"}}}`,
	}
	for _, body := range invalid {
		var result CancelReplaceResult
		if err := json.Unmarshal([]byte(body), &result); err == nil {
			t.Fatalf("expected invalid response to fail: %s", body)
		}
	}
	var result CancelReplaceResult
	if err := json.Unmarshal([]byte(cancelReplaceFailureResponse), &result); err != nil {
		t.Fatalf("valid response failed: %v", err)
	}
	if result.Cancel.Status() != CancelReplaceCancelFailure || result.Replacement.Status() != CancelReplaceReplacementNotAttempted {
		t.Fatalf("unexpected statuses")
	}
	if failure, ok := result.Cancel.Error(); !ok || failure.Code != "ORDER_NOT_FOUND" {
		t.Fatalf("unexpected failure: %+v", failure)
	}
}

func TestDelegatedCancelReplace_UnsignedTopLevelOnBehalfOfAnd409(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/orders/cancel-replace", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatal(err)
		}
		if string(raw["onBehalfOf"]) != "42" {
			t.Fatalf("missing top-level onBehalfOf: %s", body)
		}
		var replacement map[string]json.RawMessage
		_ = json.Unmarshal(raw["replacement"], &replacement)
		if _, ok := replacement["onBehalfOf"]; ok {
			t.Fatalf("onBehalfOf nested in replacement: %s", body)
		}
		if string(replacement["clientOrderId"]) != `"new-client"` || string(replacement["stpPolicy"]) != `"cancel_taker"` {
			t.Fatalf("missing replacement fields: %s", body)
		}
		var order map[string]json.RawMessage
		_ = json.Unmarshal(replacement["order"], &order)
		if _, ok := order["signature"]; ok {
			t.Fatalf("delegated signature must be omitted: %s", body)
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(cancelReplaceFailureResponse))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	service := NewDelegatedOrderService(NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("key")))
	result, err := service.CancelReplace(context.Background(), DelegatedCancelReplaceParams{
		Cancel: CancelByClientOrderID("old-client"), Mode: CancelReplaceStopOnFailure, OnBehalfOf: 42,
		Replacement: CancelReplaceOrderParams{MarketSlug: "market", OrderType: OrderTypeGTC, ClientOrderID: "new-client", STPPolicy: STPCancelTaker,
			Args: GTCOrderArgs{TokenID: "123", Side: SideBuy, Price: .5, Size: 1}},
	})
	if err != nil {
		t.Fatalf("CancelReplace returned error: %v", err)
	}
	if result.Replacement.Status() != CancelReplaceReplacementNotAttempted {
		t.Fatalf("unexpected replacement status")
	}
}

func TestOrderClientCancelReplace_SignsForVenueAnd400Errors(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/profiles/"+testSignerAddress, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":42,"account":"` + testSignerAddress + `","rank":{"feeRateBps":250}}`))
	})
	mux.HandleFunc("/markets/market", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"slug":"market","venue":{"exchange":"0xa4409D988CA2218d956BeEFD3874100F444f0DC3"}}`))
	})
	mux.HandleFunc("/orders/cancel-replace", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"signature":"0x`) {
			t.Fatalf("direct replacement was not signed: %s", body)
		}
		if strings.Contains(string(body), `"onBehalfOf"`) {
			t.Fatalf("direct request contains onBehalfOf: %s", body)
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"bad request"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client, err := NewOrderClient(NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("key")), testPrivateKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CancelReplace(context.Background(), CancelReplaceParams{Cancel: CancelByOrderID("old"), Mode: CancelReplaceAllowFailure,
		Replacement: CancelReplaceOrderParams{MarketSlug: "market", OrderType: OrderTypeGTC, Args: GTCOrderArgs{TokenID: "123", Side: SideBuy, Price: .5, Size: 1}}})
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError for 400, got %T %v", err, err)
	}
}

func TestDelegatedCancelReplaceBatch_Accepts207AndDoesNotLimitFour(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/orders/cancel-replace/batch", func(w http.ResponseWriter, r *http.Request) {
		var request CancelReplaceBatchRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if len(request.Operations) != 5 {
			t.Fatalf("expected five operations, got %d", len(request.Operations))
		}
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`{"results":[{"index":0,"cancel":{"status":"FAILURE","error":{"code":"x","message":"x"}},"replacement":{"status":"NOT_ATTEMPTED"}}]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	service := NewDelegatedOrderService(NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("key")))
	operation := DelegatedCancelReplaceParams{Cancel: CancelByOrderID("old"), Mode: CancelReplaceStopOnFailure, OnBehalfOf: 42,
		Replacement: CancelReplaceOrderParams{MarketSlug: "market", OrderType: OrderTypeGTC, Args: GTCOrderArgs{TokenID: "123", Side: SideBuy, Price: .5, Size: 1}}}
	result, err := service.CancelReplaceBatch(context.Background(), []DelegatedCancelReplaceParams{operation, operation, operation, operation, operation})
	if err != nil {
		t.Fatalf("batch returned error: %v", err)
	}
	if len(result.Results) != 1 || result.Results[0].Index != 0 {
		t.Fatalf("unexpected batch result: %+v", result.Results)
	}
}

func TestHttpClientPostRaw_HMACUsesExactBodyAndPath(t *testing.T) {
	t.Parallel()
	secret := "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="
	mux := http.NewServeMux()
	mux.HandleFunc("/orders/cancel-replace", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		expected, err := computeHMACSignature(secret, r.Header.Get("lmts-timestamp"), http.MethodPost, "/orders/cancel-replace", string(body))
		if err != nil {
			t.Fatal(err)
		}
		if r.Header.Get("lmts-signature") != expected {
			t.Fatalf("signature does not match transmitted bytes")
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(cancelReplaceFailureResponse))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := NewHttpClient(WithBaseURL(srv.URL), WithHMACCredentials(HMACCredentials{TokenID: "token", Secret: secret}))
	_, err := submitCancelReplace(context.Background(), client, CancelReplaceRequest{Cancel: CancelByOrderID("old"), Mode: CancelReplaceStopOnFailure})
	if err != nil {
		t.Fatalf("submit returned error: %v", err)
	}
}
