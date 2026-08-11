package limitless

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestServerWalletService_SplitAndMergePositions(t *testing.T) {
	t.Parallel()

	const secret = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="
	const conditionID = "0x1111111111111111111111111111111111111111111111111111111111111111"
	const exchange = "0x3333333333333333333333333333333333333333"

	mux := http.NewServeMux()
	mux.HandleFunc("/portfolio/split", func(w http.ResponseWriter, r *http.Request) {
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
			t.Fatalf("failed to read split request body: %v", err)
		}

		var payload splitMergeServerWalletRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode split payload: %v", err)
		}
		if payload.ConditionID != conditionID || payload.Amount != "1000000" || payload.OnBehalfOf != 42 {
			t.Fatalf("unexpected split payload: %+v", payload)
		}
		var rawPayload map[string]any
		if err := json.Unmarshal(body, &rawPayload); err != nil {
			t.Fatalf("failed to decode split raw payload: %v", err)
		}
		if _, ok := rawPayload["marketId"]; ok {
			t.Fatalf("split payload must not include marketId: %+v", rawPayload)
		}
		venuePayload, ok := rawPayload["venue"].(map[string]any)
		if !ok {
			t.Fatalf("CLOB split payload must include venue.exchange: %+v", rawPayload)
		}
		if venuePayload["exchange"] != exchange {
			t.Fatalf("expected CLOB split venue.exchange %s, got %+v", exchange, venuePayload)
		}
		if _, ok := venuePayload["adapter"]; ok {
			t.Fatalf("CLOB split payload must not include venue.adapter: %+v", venuePayload)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"userOperationHash": "0xsplituserop",
			"transactionId":     "tx-split",
			"walletAddress":     "0x1111111111111111111111111111111111111111",
			"conditionId":       payload.ConditionID,
			"marketId":          99,
			"operation":         "SPLIT",
		})
	})
	mux.HandleFunc("/portfolio/merge", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
		}
		if got := r.Header.Get("lmts-api-key"); got != "token-1" {
			t.Fatalf("expected lmts-api-key token-1, got %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read merge request body: %v", err)
		}

		var payload splitMergeServerWalletRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode merge payload: %v", err)
		}
		if payload.ConditionID != conditionID || payload.Amount != "1000000" || payload.OnBehalfOf != 42 {
			t.Fatalf("unexpected merge payload: %+v", payload)
		}
		var rawPayload map[string]any
		if err := json.Unmarshal(body, &rawPayload); err != nil {
			t.Fatalf("failed to decode merge raw payload: %v", err)
		}
		if _, ok := rawPayload["marketId"]; ok {
			t.Fatalf("merge payload must not include marketId: %+v", rawPayload)
		}
		venuePayload, ok := rawPayload["venue"].(map[string]any)
		if !ok {
			t.Fatalf("CLOB merge payload must include venue.exchange: %+v", rawPayload)
		}
		if venuePayload["exchange"] != exchange {
			t.Fatalf("expected CLOB merge venue.exchange %s, got %+v", exchange, venuePayload)
		}
		if _, ok := venuePayload["adapter"]; ok {
			t.Fatalf("CLOB merge payload must not include venue.adapter: %+v", venuePayload)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"userOperationHash": "0xmergeuserop",
			"transactionId":     "tx-merge",
			"walletAddress":     "0x1111111111111111111111111111111111111111",
			"conditionId":       payload.ConditionID,
			"marketId":          99,
			"operation":         "MERGE",
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

	splitResp, err := service.SplitPositions(context.Background(), SplitServerWalletParams{
		ConditionID: conditionID,
		Amount:      "1000000",
		Venue:       &ServerWalletVenue{Exchange: "  " + exchange + "  "},
		OnBehalfOf:  42,
	})
	if err != nil {
		t.Fatalf("SplitPositions returned error: %v", err)
	}
	if splitResp.TransactionID != "tx-split" || splitResp.MarketID != 99 || splitResp.ConditionID != conditionID || splitResp.Operation != "SPLIT" {
		t.Fatalf("unexpected split response: %+v", splitResp)
	}
	if len(splitResp.RawJSON()) == 0 {
		t.Fatal("expected split response to preserve raw API JSON")
	}

	mergeResp, err := service.MergePositions(context.Background(), MergeServerWalletParams{
		ConditionID: conditionID,
		Amount:      "1000000",
		Venue:       &ServerWalletVenue{Exchange: "  " + exchange + "  "},
		OnBehalfOf:  42,
	})
	if err != nil {
		t.Fatalf("MergePositions returned error: %v", err)
	}
	if mergeResp.TransactionID != "tx-merge" || mergeResp.ConditionID != conditionID || mergeResp.MarketID != 99 || mergeResp.Operation != "MERGE" {
		t.Fatalf("unexpected merge response: %+v", mergeResp)
	}
	if len(mergeResp.RawJSON()) == 0 {
		t.Fatal("expected merge response to preserve raw API JSON")
	}
}

func TestServerWalletService_SplitAndMergePositions_WithNegRiskVenue(t *testing.T) {
	t.Parallel()

	const secret = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="
	const conditionID = "0x1111111111111111111111111111111111111111111111111111111111111111"
	const adapter = "0x2222222222222222222222222222222222222222"
	const exchange = "0x3333333333333333333333333333333333333333"

	mux := http.NewServeMux()
	checkRequest := func(w http.ResponseWriter, r *http.Request, operation string) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
		}
		if got := r.Header.Get("lmts-api-key"); got != "token-1" {
			t.Fatalf("expected lmts-api-key token-1, got %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read %s request body: %v", strings.ToLower(operation), err)
		}

		var payload splitMergeServerWalletRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode %s payload: %v", strings.ToLower(operation), err)
		}
		if payload.ConditionID != conditionID || payload.Amount != "1000000" || payload.OnBehalfOf != 42 {
			t.Fatalf("unexpected %s payload: %+v", strings.ToLower(operation), payload)
		}
		if payload.Venue == nil || payload.Venue.Adapter != adapter {
			t.Fatalf("expected %s venue adapter %s, got %+v", strings.ToLower(operation), adapter, payload.Venue)
		}
		if payload.Venue.Exchange != exchange {
			t.Fatalf("expected %s venue exchange %s, got %+v", strings.ToLower(operation), exchange, payload.Venue)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"userOperationHash": "0x" + strings.ToLower(operation) + "userop",
			"transactionId":     "tx-" + strings.ToLower(operation),
			"walletAddress":     "0x1111111111111111111111111111111111111111",
			"conditionId":       payload.ConditionID,
			"marketId":          99,
			"operation":         operation,
			"route":             "NEGRISK",
		})
	}
	mux.HandleFunc("/portfolio/split", func(w http.ResponseWriter, r *http.Request) {
		checkRequest(w, r, "SPLIT")
	})
	mux.HandleFunc("/portfolio/merge", func(w http.ResponseWriter, r *http.Request) {
		checkRequest(w, r, "MERGE")
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

	venue := &ServerWalletVenue{
		Exchange: "  " + exchange + "  ",
		Adapter:  "  " + adapter + "  ",
	}
	splitResp, err := service.SplitPositions(context.Background(), SplitServerWalletParams{
		ConditionID: conditionID,
		Amount:      "1000000",
		Venue:       venue,
		OnBehalfOf:  42,
	})
	if err != nil {
		t.Fatalf("SplitPositions returned error: %v", err)
	}
	if splitResp.TransactionID != "tx-split" || splitResp.Operation != "SPLIT" || splitResp.Route != "NEGRISK" {
		t.Fatalf("unexpected split response: %+v", splitResp)
	}

	mergeResp, err := service.MergePositions(context.Background(), MergeServerWalletParams{
		ConditionID: conditionID,
		Amount:      "1000000",
		Venue:       venue,
		OnBehalfOf:  42,
	})
	if err != nil {
		t.Fatalf("MergePositions returned error: %v", err)
	}
	if mergeResp.TransactionID != "tx-merge" || mergeResp.Operation != "MERGE" || mergeResp.Route != "NEGRISK" {
		t.Fatalf("unexpected merge response: %+v", mergeResp)
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

func TestServerWalletService_Withdraw_DestinationOnlyOwnWallet(t *testing.T) {
	t.Parallel()

	const secret = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="
	const destination = "0x3333333333333333333333333333333333333333"

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
		if _, ok := payload["onBehalfOf"]; ok {
			t.Fatalf("expected onBehalfOf to be omitted, got %+v", payload)
		}
		if payload["amount"] != "1000000" || payload["destination"] != destination {
			t.Fatalf("unexpected destination-only withdraw payload: %+v", payload)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"hash":              "0xwithdraw",
			"userOperationHash": "0xuserop",
			"transactionId":     "tx-own-wallet",
			"walletAddress":     "0x1111111111111111111111111111111111111111",
			"token":             "0x0000000000000000000000000000000000000000",
			"destination":       destination,
			"amount":            "1000000",
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
		Destination: destination,
	})
	if err != nil {
		t.Fatalf("Withdraw returned error: %v", err)
	}
	if resp.TransactionID != "tx-own-wallet" || resp.Destination != destination {
		t.Fatalf("unexpected withdraw response: %+v", resp)
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
				Amount:      "1",
				OnBehalfOf:  -1,
				Destination: "0x3333333333333333333333333333333333333333",
			},
			want: "OnBehalfOf must be a positive integer",
		},
		{
			name: "missing on behalf of and destination",
			params: WithdrawServerWalletParams{
				Amount: "1",
			},
			want: "OnBehalfOf or Destination is required for withdraw",
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

func TestServerWalletService_RejectsInvalidSplitMergeParamsBeforeNetwork(t *testing.T) {
	t.Parallel()

	service := NewServerWalletService(NewHttpClient(WithHMACCredentials(HMACCredentials{
		TokenID: "token-1",
		Secret:  "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=",
	})))

	tests := []struct {
		name   string
		params SplitServerWalletParams
		want   string
	}{
		{
			name: "missing condition id",
			params: SplitServerWalletParams{
				Amount: "1",
			},
			want: "ConditionID is required",
		},
		{
			name: "invalid condition id",
			params: SplitServerWalletParams{
				ConditionID: "bad",
				Amount:      "1",
			},
			want: "ConditionID must be a 0x-prefixed 32-byte hex string",
		},
		{
			name: "invalid amount",
			params: SplitServerWalletParams{
				ConditionID: "0x1111111111111111111111111111111111111111111111111111111111111111",
				Amount:      "0",
			},
			want: "Amount must be a positive integer string in the token smallest unit",
		},
		{
			name: "invalid on behalf of",
			params: SplitServerWalletParams{
				ConditionID: "0x1111111111111111111111111111111111111111111111111111111111111111",
				Amount:      "1",
				OnBehalfOf:  -1,
			},
			want: "OnBehalfOf must be a positive integer",
		},
		{
			name: "missing venue",
			params: SplitServerWalletParams{
				ConditionID: "0x1111111111111111111111111111111111111111111111111111111111111111",
				Amount:      "1",
			},
			want: "Venue is required",
		},
		{
			name: "missing exchange without adapter",
			params: SplitServerWalletParams{
				ConditionID: "0x1111111111111111111111111111111111111111111111111111111111111111",
				Amount:      "1",
				Venue:       &ServerWalletVenue{},
			},
			want: "Venue.Exchange is required when Venue.Adapter is not provided",
		},
		{
			name: "invalid venue exchange",
			params: SplitServerWalletParams{
				ConditionID: "0x1111111111111111111111111111111111111111111111111111111111111111",
				Amount:      "1",
				Venue:       &ServerWalletVenue{Exchange: "bad"},
			},
			want: "Venue.Exchange must be a valid EVM address",
		},
		{
			name: "invalid venue adapter",
			params: SplitServerWalletParams{
				ConditionID: "0x1111111111111111111111111111111111111111111111111111111111111111",
				Amount:      "1",
				Venue:       &ServerWalletVenue{Adapter: "bad"},
			},
			want: "Venue.Adapter must be a valid EVM address",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.SplitPositions(context.Background(), tc.params)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("expected split error %q, got %v", tc.want, err)
			}

			_, err = service.MergePositions(context.Background(), MergeServerWalletParams(tc.params))
			if err == nil || err.Error() != tc.want {
				t.Fatalf("expected merge error %q, got %v", tc.want, err)
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
