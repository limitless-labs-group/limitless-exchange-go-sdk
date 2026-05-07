package limitless

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPartnerAccountService_AddWithdrawalAddress(t *testing.T) {
	t.Parallel()

	const secret = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="

	mux := http.NewServeMux()
	mux.HandleFunc("/portfolio/withdrawal-addresses", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
		}
		if got := r.Header.Get("identity"); got != "Bearer identity-token" {
			t.Fatalf("expected identity header, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("expected Authorization header to be omitted, got %q", got)
		}
		if got := r.Header.Get("lmts-api-key"); got != "" {
			t.Fatalf("expected HMAC auth to be suppressed for identity request, got %q", got)
		}

		var body PartnerWithdrawalAddressInput
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body.Address != "0x0F3262730c909408042F9Da345a916dc0e1F9787" || body.Label != "treasury" {
			t.Fatalf("unexpected request body: %+v", body)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":                 "11111111-1111-4111-8111-111111111111",
			"profileId":          1292711,
			"destinationAddress": "0x0F3262730c909408042F9Da345a916dc0e1F9787",
			"label":              "treasury",
			"createdAt":          "2026-04-30T12:00:00.000Z",
			"deletedAt":          nil,
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

	resp, err := service.AddWithdrawalAddress(context.Background(), "identity-token", PartnerWithdrawalAddressInput{
		Address: "0x0F3262730c909408042F9Da345a916dc0e1F9787",
		Label:   "treasury",
	})
	if err != nil {
		t.Fatalf("AddWithdrawalAddress returned error: %v", err)
	}
	if resp.ID != "11111111-1111-4111-8111-111111111111" || resp.ProfileID != 1292711 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.DestinationAddress != "0x0F3262730c909408042F9Da345a916dc0e1F9787" || resp.Label != "treasury" {
		t.Fatalf("unexpected destination response: %+v", resp)
	}
	if resp.DeletedAt != nil {
		t.Fatalf("expected nil DeletedAt, got %v", *resp.DeletedAt)
	}
}

func TestPartnerAccountService_DeleteWithdrawalAddress(t *testing.T) {
	t.Parallel()

	const secret = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="
	const address = "0x0F3262730c909408042F9Da345a916dc0e1F9787"

	mux := http.NewServeMux()
	mux.HandleFunc("/portfolio/withdrawal-addresses/"+address, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE method, got %s", r.Method)
		}
		if got := r.Header.Get("identity"); got != "Bearer identity-token" {
			t.Fatalf("expected identity header, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("expected Authorization header to be omitted, got %q", got)
		}
		if got := r.Header.Get("lmts-api-key"); got != "" {
			t.Fatalf("expected HMAC auth to be suppressed for identity request, got %q", got)
		}

		w.WriteHeader(http.StatusNoContent)
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

	if err := service.DeleteWithdrawalAddress(context.Background(), "identity-token", address); err != nil {
		t.Fatalf("DeleteWithdrawalAddress returned error: %v", err)
	}
}

func TestPartnerAccountService_WithdrawalAddressValidation(t *testing.T) {
	t.Parallel()

	service := NewPartnerAccountService(NewHttpClient())

	_, err := service.AddWithdrawalAddress(context.Background(), "", PartnerWithdrawalAddressInput{Address: "0x1"})
	if err == nil || err.Error() != "identity token is required for AddWithdrawalAddress" {
		t.Fatalf("expected missing identity token error, got %v", err)
	}

	_, err = service.AddWithdrawalAddress(context.Background(), "identity-token", PartnerWithdrawalAddressInput{})
	if err == nil || err.Error() != "address is required for AddWithdrawalAddress" {
		t.Fatalf("expected missing address error, got %v", err)
	}

	err = service.DeleteWithdrawalAddress(context.Background(), "", "0x1")
	if err == nil || err.Error() != "identity token is required for DeleteWithdrawalAddress" {
		t.Fatalf("expected missing identity token error, got %v", err)
	}

	err = service.DeleteWithdrawalAddress(context.Background(), "identity-token", "")
	if err == nil || err.Error() != "address is required for DeleteWithdrawalAddress" {
		t.Fatalf("expected missing address error, got %v", err)
	}
}
