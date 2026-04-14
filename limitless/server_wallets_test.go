package limitless

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerWalletService_RedeemPositions(t *testing.T) {
	t.Parallel()

	const secret = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="
	const conditionID = "0x1111111111111111111111111111111111111111111111111111111111111111"

	mux := http.NewServeMux()
	mux.HandleFunc("/portfolio/redeem", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
		}
		if got := r.Header.Get("X-API-Key"); got != "" {
			t.Fatalf("expected X-API-Key to be suppressed when HMAC is configured, got %q", got)
		}
		if got := r.Header.Get("lmts-api-key"); got != "token-1" {
			t.Fatalf("expected lmts-api-key token-1, got %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload redeemServerWalletRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode redeem payload: %v", err)
		}
		if payload.ConditionID != conditionID || payload.OnBehalfOf != 42 {
			t.Fatalf("unexpected redeem payload: %+v", payload)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"hash":              "0xredeem",
			"userOperationHash": "0xuserop",
			"transactionId":     "tx-123",
			"walletAddress":     "0x1111111111111111111111111111111111111111",
			"conditionId":       payload.ConditionID,
			"marketId":          99,
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	service := NewServerWalletService(NewHttpClient(
		WithBaseURL(srv.URL),
		WithHMACCredentials(HMACCredentials{
			TokenID: "token-1",
			Secret:  secret,
		}),
	))

	resp, err := service.RedeemPositions(context.Background(), RedeemServerWalletParams{
		ConditionID: conditionID,
		OnBehalfOf:  42,
	})
	if err != nil {
		t.Fatalf("RedeemPositions returned error: %v", err)
	}
	if resp.TransactionID != "tx-123" || resp.ConditionID != conditionID || resp.MarketID != 99 {
		t.Fatalf("unexpected redeem response: %+v", resp)
	}
}

func TestServerWalletService_Withdraw(t *testing.T) {
	t.Parallel()

	const secret = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="

	mux := http.NewServeMux()
	mux.HandleFunc("/portfolio/withdraw", func(w http.ResponseWriter, r *http.Request) {
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

		var payload withdrawServerWalletRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode withdraw payload: %v", err)
		}
		if payload.Amount != "1000000" || payload.OnBehalfOf != 42 {
			t.Fatalf("unexpected withdraw payload: %+v", payload)
		}
		if payload.Token == nil || *payload.Token != "0x2222222222222222222222222222222222222222" {
			t.Fatalf("expected token in payload, got %+v", payload.Token)
		}
		if payload.Destination == nil || *payload.Destination != "0x3333333333333333333333333333333333333333" {
			t.Fatalf("expected destination in payload, got %+v", payload.Destination)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"hash":              "0xwithdraw",
			"userOperationHash": "0xuserop",
			"transactionId":     "tx-456",
			"walletAddress":     "0x1111111111111111111111111111111111111111",
			"token":             *payload.Token,
			"destination":       *payload.Destination,
			"amount":            payload.Amount,
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	service := NewServerWalletService(NewHttpClient(
		WithBaseURL(srv.URL),
		WithHMACCredentials(HMACCredentials{
			TokenID: "token-1",
			Secret:  secret,
		}),
	))

	resp, err := service.Withdraw(context.Background(), WithdrawServerWalletParams{
		Amount:      "1000000",
		OnBehalfOf:  42,
		Token:       "0x2222222222222222222222222222222222222222",
		Destination: "0x3333333333333333333333333333333333333333",
	})
	if err != nil {
		t.Fatalf("Withdraw returned error: %v", err)
	}
	if resp.TransactionID != "tx-456" || resp.Amount != "1000000" {
		t.Fatalf("unexpected withdraw response: %+v", resp)
	}
}

func TestServerWalletService_Withdraw_OmitsOptionalFields(t *testing.T) {
	t.Parallel()

	const secret = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="

	mux := http.NewServeMux()
	mux.HandleFunc("/portfolio/withdraw", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode withdraw payload: %v", err)
		}
		if _, ok := payload["token"]; ok {
			t.Fatalf("expected token to be omitted, got %+v", payload)
		}
		if _, ok := payload["destination"]; ok {
			t.Fatalf("expected destination to be omitted, got %+v", payload)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"hash":              "0xwithdraw",
			"userOperationHash": "0xuserop",
			"transactionId":     "tx-789",
			"walletAddress":     "0x1111111111111111111111111111111111111111",
			"token":             "0x0000000000000000000000000000000000000000",
			"destination":       "0x4444444444444444444444444444444444444444",
			"amount":            "1",
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	service := NewServerWalletService(NewHttpClient(
		WithBaseURL(srv.URL),
		WithHMACCredentials(HMACCredentials{
			TokenID: "token-1",
			Secret:  secret,
		}),
	))

	if _, err := service.Withdraw(context.Background(), WithdrawServerWalletParams{
		Amount:     "1",
		OnBehalfOf: 42,
	}); err != nil {
		t.Fatalf("Withdraw returned error: %v", err)
	}
}

func TestServerWalletService_RejectsInvalidRedeemParamsBeforeNetwork(t *testing.T) {
	t.Parallel()

	service := NewServerWalletService(NewHttpClient(WithHMACCredentials(HMACCredentials{
		TokenID: "token-1",
		Secret:  "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=",
	})))

	tests := []struct {
		name   string
		params RedeemServerWalletParams
		want   string
	}{
		{
			name: "invalid condition id",
			params: RedeemServerWalletParams{
				ConditionID: "bad",
				OnBehalfOf:  42,
			},
			want: "ConditionID must be a 0x-prefixed 32-byte hex string",
		},
		{
			name: "invalid on behalf of",
			params: RedeemServerWalletParams{
				ConditionID: "0x1111111111111111111111111111111111111111111111111111111111111111",
				OnBehalfOf:  0,
			},
			want: "OnBehalfOf must be a positive integer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.RedeemPositions(context.Background(), tc.params)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("expected error %q, got %v", tc.want, err)
			}
		})
	}
}

func TestServerWalletService_RejectsInvalidWithdrawParamsBeforeNetwork(t *testing.T) {
	t.Parallel()

	service := NewServerWalletService(NewHttpClient(WithHMACCredentials(HMACCredentials{
		TokenID: "token-1",
		Secret:  "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=",
	})))

	tests := []struct {
		name   string
		params WithdrawServerWalletParams
		want   string
	}{
		{
			name: "invalid amount",
			params: WithdrawServerWalletParams{
				Amount:     "0",
				OnBehalfOf: 42,
			},
			want: "Amount must be a positive integer string in the token smallest unit",
		},
		{
			name: "invalid on behalf of",
			params: WithdrawServerWalletParams{
				Amount:     "1",
				OnBehalfOf: 0,
			},
			want: "OnBehalfOf must be a positive integer",
		},
		{
			name: "invalid token",
			params: WithdrawServerWalletParams{
				Amount:     "1",
				OnBehalfOf: 42,
				Token:      "bad",
			},
			want: "Token must be a valid EVM address",
		},
		{
			name: "invalid destination",
			params: WithdrawServerWalletParams{
				Amount:      "1",
				OnBehalfOf:  42,
				Destination: "bad",
			},
			want: "Destination must be a valid EVM address",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.Withdraw(context.Background(), tc.params)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("expected error %q, got %v", tc.want, err)
			}
		})
	}
}

func TestServerWalletService_RejectsLegacyAPIKeyOnlyAuth(t *testing.T) {
	t.Parallel()

	service := NewServerWalletService(NewHttpClient(WithAPIKey("api-key")))

	_, err := service.RedeemPositions(context.Background(), RedeemServerWalletParams{
		ConditionID: "0x1111111111111111111111111111111111111111111111111111111111111111",
		OnBehalfOf:  42,
	})
	if err == nil || err.Error() != serverWalletHMACOnlyError {
		t.Fatalf("expected error %q, got %v", serverWalletHMACOnlyError, err)
	}
}
