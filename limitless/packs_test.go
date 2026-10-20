package limitless

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testPackID = "22222222-2222-4222-8222-222222222222"

func TestPacksIneligibleQuote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"eligible":false,"ineligibleReasons":["Insufficient depth"]}`)
	}))
	defer server.Close()
	s := NewClient(WithBaseURL(server.URL), WithHMACCredentials(HMACCredentials{TokenID: "test", Secret: "c2VjcmV0"})).Packs
	q, err := s.Quote(context.Background(), PackQuoteParams{PackID: testPackID, StakeAmount: "1000000"})
	if err != nil || q.Eligible || len(q.IneligibleReasons) != 1 || q.IneligibleReasons[0] != "Insufficient depth" {
		t.Fatalf("minimal quote: %+v %v", q, err)
	}
}

func TestPacksMintRecoveryAndVaultRevert(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := "FAILED"
		switch calls.Add(1) {
		case 2:
			status = "ACTIVE"
		case 3:
			status = "VAULT_REVERTED"
		}
		fmt.Fprintf(w, `{"id":%q,"status":%q,"txHash":"0xbroadcast"}`, testPackID, status)
	}))
	defer server.Close()
	s := NewClient(WithBaseURL(server.URL), WithHMACCredentials(HMACCredentials{TokenID: "test", Secret: "c2VjcmV0"})).Packs
	s.spacing = 0
	p, err := s.WaitForPosition(context.Background(), 123, testPackID)
	if err != nil || p.Status != "ACTIVE" || calls.Load() != 2 {
		t.Fatalf("recovery: %+v %v", p, err)
	}
	_, err = s.WaitForPosition(context.Background(), 123, testPackID)
	if err == nil || !strings.Contains(err.Error(), "VAULT_REVERTED") || calls.Load() != 3 {
		t.Fatalf("revert: %v", err)
	}
}

func TestPacksRateLimitDelaysNextCall(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(429)
		io.WriteString(w, `{"message":"rate limited","retryAfterSeconds":10}`)
	}))
	defer server.Close()
	s := NewClient(WithBaseURL(server.URL), WithHMACCredentials(HMACCredentials{TokenID: "test", Secret: "c2VjcmV0"})).Packs
	_, err := s.List(context.Background())
	var limit *RateLimitError
	if !errors.As(err, &limit) {
		t.Fatalf("want rate error: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = s.List(ctx)
	if !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 1 {
		t.Fatalf("retried before backoff: calls=%d error=%v", calls.Load(), err)
	}
}

func TestPacksErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		kind   PackErrorKind
	}{
		{409, `{"code":"PACK_ALLOWANCE_REQUIRED","tokenAddress":"0xtoken","spender":"0xvault","requiredAllowance":"1000000"}`, PackErrorAllowanceRequired},
		{400, `{"code":"BAD_INTENT_SIGNATURE","submissionState":"NOT_SUBMITTED","parlayId":"position","recovery":"Fix cause"}`, PackErrorNotSubmitted},
		{409, `{"code":"PRICE_DRIFTED"}`, PackErrorPriceDrifted},
		{401, `{"message":"Intent expired — please retry"}`, PackErrorIntentExpired},
		{401, `{"message":"Invalid signature"}`, PackErrorOther},
		{409, `{"message":"Another Pack operation is in progress for this wallet; retry the same request"}`, PackErrorWalletBusy},
		{429, `{}`, PackErrorRateLimited},
		{409, `{"code":"PACK_TRANSACTION_REVERTED"}`, PackErrorTransactionReverted},
		{409, `{"code":"PACK_SUBMISSION_UNKNOWN","submissionState":"NOT_SUBMITTED"}`, PackErrorSubmissionUnknown},
		{500, `{"submissionState":"NOT_SUBMITTED"}`, PackErrorOther},
		{400, `{"submissionState":true}`, PackErrorOther},
	} {
		err := parseAPIError(tc.status, []byte(tc.body), "/parlay/partner/buy", "POST")
		result := ClassifyPackError(fmt.Errorf("wrapped: %w", err))
		if result.Kind != tc.kind {
			t.Fatalf("body=%s got=%s want=%s", tc.body, result.Kind, tc.kind)
		}
		if result.Kind == PackErrorAllowanceRequired && (result.Spender != "0xvault" || result.TokenAddress != "0xtoken" || result.RequiredAllowance != "1000000") {
			t.Fatalf("lost allowance details: %+v", result)
		}
		if result.Kind == PackErrorNotSubmitted && (result.ParlayID != "position" || result.Recovery != "Fix cause") {
			t.Fatalf("lost failure details: %+v", result)
		}
	}
	if ClassifyPackError(context.DeadlineExceeded).Kind != PackErrorOther {
		t.Fatal("classified transport error as safe")
	}
}

func testPackBuy() PackBuyParams {
	return PackBuyParams{OnBehalfOf: 123, PackID: testPackID, StakeAmount: "1000000", MinPayoutAmount: "2000000", Deadline: "1", IdempotencyKey: "order-1"}
}
func TestPacksWireSigningReplayAndHeaders(t *testing.T) {
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mac := hmac.New(sha256.New, []byte("secret"))
		mac.Write([]byte(r.Header.Get("lmts-timestamp") + "\n" + r.Method + "\n" + r.URL.RequestURI() + "\n" + string(raw)))
		if r.Header.Get("lmts-signature") != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
			t.Error("signature does not cover actual body/query")
		}
		if r.Method == "POST" {
			if r.Header.Get("x-on-behalf-of") != "" {
				t.Error("write has delegation header")
			}
			bodies = append(bodies, string(raw))
			io.WriteString(w, `{"status":"RECOVERING","position":{"id":"`+testPackID+`","status":"PENDING"}}`)
		} else {
			if r.Header.Get("x-on-behalf-of") != "123" {
				t.Error("missing read delegation")
			}
			if r.URL.RawQuery != "limit=50&offset=0" {
				t.Error(r.URL.RawQuery)
			}
			io.WriteString(w, `{"data":[],"total":0,"limit":50,"offset":0}`)
		}
	}))
	defer server.Close()
	s := NewClient(WithBaseURL(server.URL), WithHMACCredentials(HMACCredentials{TokenID: "test", Secret: base64.StdEncoding.EncodeToString([]byte("secret"))})).Packs
	s.spacing = 0
	for i := 0; i < 2; i++ {
		out, err := s.Buy(context.Background(), testPackBuy())
		if err != nil || out.Status != "RECOVERING" {
			t.Fatalf("buy: %v %v", out, err)
		}
	}
	if bodies[0] != bodies[1] {
		t.Fatal("retry changed expired request")
	}
	if _, err := s.ListPositions(context.Background(), 123, 100, -1); err != nil {
		t.Fatal(err)
	}
}
func TestPacksValidationAndIntegerSlippage(t *testing.T) {
	for _, key := range []string{"", "with space", "é", strings.Repeat("a", 129), "a\n"} {
		p := testPackBuy()
		p.IdempotencyKey = key
		if validatePackBuy(p) == nil {
			t.Fatalf("accepted key %q", key)
		}
	}
	for _, amount := range []string{"0", "01", "1.5", "1e6", strings.Repeat("9", 21)} {
		p := testPackBuy()
		p.StakeAmount = amount
		if validatePackBuy(p) == nil {
			t.Fatalf("accepted amount %s", amount)
		}
	}
	got, err := PackMinPayout("99999999999999999999", 100)
	if err != nil || got != "98999999999999999999" {
		t.Fatalf("lost precision: %s %v", got, err)
	}
	if validatePackBuy(testPackBuy()) != nil {
		t.Fatal("expired replay must be accepted")
	}
}
func TestPacksClaimUnknownStopsAndErrorsPreserved(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(409)
		io.WriteString(w, `{"code":"PACK_SUBMISSION_UNKNOWN","message":"Find receipt"}`)
	}))
	defer srv.Close()
	s := NewClient(WithBaseURL(srv.URL), WithHMACCredentials(HMACCredentials{TokenID: "test", Secret: "c2VjcmV0"})).Packs
	_, err := s.Claim(context.Background(), 123, testPackID)
	conflict, ok := err.(*ConflictError)
	if !ok || !strings.Contains(string(conflict.Data), "PACK_SUBMISSION_UNKNOWN") || calls != 1 {
		t.Fatalf("claim must stop: %v calls=%d", err, calls)
	}
}
func TestPacksCancellationAndAuth(t *testing.T) {
	s := NewClient(WithAPIKey("legacy")).Packs
	if _, err := s.Buy(context.Background(), testPackBuy()); err == nil {
		t.Fatal("accepted legacy auth")
	}
	s = NewClient(WithHMACCredentials(HMACCredentials{TokenID: "test", Secret: "c2VjcmV0"})).Packs
	s.next = time.Now().Add(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Buy(ctx, testPackBuy()); err != context.Canceled {
		t.Fatalf("want cancellation, got %v", err)
	}
	if packLogBody("/parlay/partner/buy", []byte(`{"prepared":"secret"}`)) != "[Pack response omitted]" {
		t.Fatal("leaked prepared response")
	}
}

func TestPacksPositionUsesDecimalStrings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":"`+testPackID+`","status":"ACTIVE","stakeAmount":"1000000","collateralDeposited":"2000000","multiplierOffered":"2.000000","vigPercent":"10.00","wallet":"0xwallet","createdAt":"2026-10-18T00:00:00Z","legs":[{"marketId":1,"outcome":"YES","priceAtCreation":"0.54","resolutionStatus":"PENDING"}]}`)
	}))
	defer server.Close()
	s := NewClient(WithBaseURL(server.URL), WithHMACCredentials(HMACCredentials{TokenID: "test", Secret: "c2VjcmV0"})).Packs
	result, err := s.WaitForPosition(context.Background(), 123, testPackID)
	if err != nil {
		t.Fatal(err)
	}
	if result.MultiplierOffered != "2.000000" || result.VigPercent != "10.00" || result.Legs[0].PriceAtCreation != "0.54" {
		t.Fatalf("position decimal fields changed: %+v", result)
	}
}
