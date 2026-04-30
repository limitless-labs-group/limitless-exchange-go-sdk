package limitless

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPartnerAccountService_CheckAllowances(t *testing.T) {
	t.Parallel()

	const secret = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="

	mux := http.NewServeMux()
	mux.HandleFunc("/profiles/partner-accounts/12345/allowances", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET method, got %s", r.Method)
		}
		if got := r.Header.Get("X-API-Key"); got != "" {
			t.Fatalf("expected X-API-Key to be suppressed when HMAC is configured, got %q", got)
		}
		if got := r.Header.Get("lmts-api-key"); got != "token-1" {
			t.Fatalf("expected lmts-api-key token-1, got %q", got)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"profileId":        12345,
			"partnerProfileId": 999,
			"chainId":          8453,
			"walletAddress":    "0x1111111111111111111111111111111111111111",
			"ready":            false,
			"summary": map[string]int{
				"total":     4,
				"confirmed": 3,
				"missing":   1,
				"submitted": 0,
				"failed":    0,
			},
			"targets": []map[string]any{
				{
					"type":              "USDC_ALLOWANCE",
					"tokenAddress":      "0x2222222222222222222222222222222222222222",
					"spenderOrOperator": "0x3333333333333333333333333333333333333333",
					"label":             "ctf-exchange",
					"requiredFor":       "BUY",
					"confirmed":         false,
					"status":            "missing",
					"retryable":         true,
				},
			},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	service := NewPartnerAccountService(NewHttpClient(
		WithBaseURL(srv.URL),
		WithHMACCredentials(HMACCredentials{
			TokenID: "token-1",
			Secret:  secret,
		}),
	))

	resp, err := service.CheckAllowances(context.Background(), 12345)
	if err != nil {
		t.Fatalf("CheckAllowances returned error: %v", err)
	}
	if resp.ProfileID != 12345 || resp.PartnerProfileID != 999 || resp.ChainID != 8453 {
		t.Fatalf("unexpected allowance response: %+v", resp)
	}
	if resp.Ready {
		t.Fatal("expected response to be not ready")
	}
	if resp.Summary.Total != 4 || resp.Summary.Missing != 1 {
		t.Fatalf("unexpected allowance summary: %+v", resp.Summary)
	}
	if len(resp.Targets) != 1 {
		t.Fatalf("expected one target, got %d", len(resp.Targets))
	}
	target := resp.Targets[0]
	if target.Type != PartnerAccountAllowanceTypeUSDCAllowance {
		t.Fatalf("unexpected target type: %s", target.Type)
	}
	if target.RequiredFor != PartnerAccountAllowanceRequiredForBuy {
		t.Fatalf("unexpected requiredFor: %s", target.RequiredFor)
	}
	if target.Status != PartnerAccountAllowanceStatusMissing || !target.Retryable {
		t.Fatalf("unexpected target status: %+v", target)
	}
}

func TestPartnerAccountService_RetryAllowances(t *testing.T) {
	t.Parallel()

	const secret = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="

	mux := http.NewServeMux()
	mux.HandleFunc("/profiles/partner-accounts/12345/allowances/retry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
		}
		if got := r.Header.Get("lmts-api-key"); got != "token-1" {
			t.Fatalf("expected lmts-api-key token-1, got %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if string(body) != "{}" {
			t.Fatalf("expected empty object payload, got %q", string(body))
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"profileId":        12345,
			"partnerProfileId": 999,
			"chainId":          8453,
			"walletAddress":    "0x1111111111111111111111111111111111111111",
			"ready":            false,
			"summary": map[string]int{
				"total":     4,
				"confirmed": 3,
				"missing":   0,
				"submitted": 1,
				"failed":    0,
			},
			"targets": []map[string]any{
				{
					"type":              "USDC_ALLOWANCE",
					"tokenAddress":      "0x2222222222222222222222222222222222222222",
					"spenderOrOperator": "0x3333333333333333333333333333333333333333",
					"label":             "ctf-exchange",
					"requiredFor":       "BUY",
					"confirmed":         false,
					"status":            "submitted",
					"transactionId":     "privy-transaction-id",
					"txHash":            "0xabc",
					"userOperationHash": "0xdef",
					"retryable":         false,
				},
			},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	service := NewPartnerAccountService(NewHttpClient(
		WithBaseURL(srv.URL),
		WithHMACCredentials(HMACCredentials{
			TokenID: "token-1",
			Secret:  secret,
		}),
	))

	resp, err := service.RetryAllowances(context.Background(), 12345)
	if err != nil {
		t.Fatalf("RetryAllowances returned error: %v", err)
	}
	if resp.Summary.Submitted != 1 {
		t.Fatalf("expected one submitted target, got %+v", resp.Summary)
	}
	if len(resp.Targets) != 1 {
		t.Fatalf("expected one target, got %d", len(resp.Targets))
	}
	target := resp.Targets[0]
	if target.Status != PartnerAccountAllowanceStatusSubmitted {
		t.Fatalf("unexpected target status: %s", target.Status)
	}
	if target.TransactionID == nil || *target.TransactionID != "privy-transaction-id" {
		t.Fatalf("unexpected transactionId: %+v", target.TransactionID)
	}
	if target.TxHash == nil || *target.TxHash != "0xabc" {
		t.Fatalf("unexpected txHash: %+v", target.TxHash)
	}
	if target.UserOperationHash == nil || *target.UserOperationHash != "0xdef" {
		t.Fatalf("unexpected userOperationHash: %+v", target.UserOperationHash)
	}
}

func TestPartnerAccountService_RetryAllowancesErrors(t *testing.T) {
	t.Parallel()

	const secret = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="

	mux := http.NewServeMux()
	mux.HandleFunc("/profiles/partner-accounts/12345/allowances/retry", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message":           "rate limited",
			"retryAfterSeconds": 42,
		})
	})
	mux.HandleFunc("/profiles/partner-accounts/67890/allowances/retry", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "allowance retry already running",
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	service := NewPartnerAccountService(NewHttpClient(
		WithBaseURL(srv.URL),
		WithHMACCredentials(HMACCredentials{
			TokenID: "token-1",
			Secret:  secret,
		}),
	))

	_, err := service.RetryAllowances(context.Background(), 12345)
	var rateErr *RateLimitError
	if !errors.As(err, &rateErr) {
		t.Fatalf("expected RateLimitError, got %T (%v)", err, err)
	}
	var rateBody struct {
		RetryAfterSeconds int `json:"retryAfterSeconds"`
	}
	if err := json.Unmarshal(rateErr.Data, &rateBody); err != nil {
		t.Fatalf("failed to decode rate-limit body: %v", err)
	}
	if rateBody.RetryAfterSeconds != 42 {
		t.Fatalf("expected retryAfterSeconds=42, got %d", rateBody.RetryAfterSeconds)
	}

	_, err = service.RetryAllowances(context.Background(), 67890)
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected ConflictError, got %T (%v)", err, err)
	}
}

func TestPartnerAccountService_AllowanceValidation(t *testing.T) {
	t.Parallel()

	service := NewPartnerAccountService(NewHttpClient(WithHMACCredentials(HMACCredentials{
		TokenID: "token-1",
		Secret:  "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=",
	})))

	_, err := service.CheckAllowances(context.Background(), 0)
	if err == nil || err.Error() != "ProfileID must be a positive integer" {
		t.Fatalf("expected invalid profileID error, got %v", err)
	}

	_, err = service.RetryAllowances(context.Background(), -1)
	if err == nil || err.Error() != "ProfileID must be a positive integer" {
		t.Fatalf("expected invalid profileID error, got %v", err)
	}
}

func TestPartnerAccountService_AllowancesRejectLegacyAPIKeyOnlyAuth(t *testing.T) {
	t.Parallel()

	service := NewPartnerAccountService(NewHttpClient(WithAPIKey("api-key")))

	_, err := service.CheckAllowances(context.Background(), 12345)
	if err == nil || err.Error() != partnerAccountAllowanceHMACOnlyError {
		t.Fatalf("expected error %q, got %v", partnerAccountAllowanceHMACOnlyError, err)
	}

	_, err = service.RetryAllowances(context.Background(), 12345)
	if err == nil || err.Error() != partnerAccountAllowanceHMACOnlyError {
		t.Fatalf("expected error %q, got %v", partnerAccountAllowanceHMACOnlyError, err)
	}
}
