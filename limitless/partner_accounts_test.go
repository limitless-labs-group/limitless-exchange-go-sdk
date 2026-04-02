package limitless

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func boolPtr(v bool) *bool {
	return &v
}

func TestPartnerAccountService_CreateAccount(t *testing.T) {
	t.Parallel()

	t.Run("passes displayName and EOA headers", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/profiles/partner-accounts", func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("X-API-Key"); got != "api-key" {
				t.Fatalf("expected X-API-Key api-key, got %q", got)
			}
			if got := r.Header.Get("x-account"); got != "0xabc" {
				t.Fatalf("expected x-account header, got %q", got)
			}
			if got := r.Header.Get("x-signing-message"); got != "0x1234" {
				t.Fatalf("expected x-signing-message header, got %q", got)
			}
			if got := r.Header.Get("x-signature"); got != "0xbeef" {
				t.Fatalf("expected x-signature header, got %q", got)
			}

			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("failed to decode request body: %v", err)
			}
			if got := payload["displayName"]; got != "child-account" {
				t.Fatalf("expected displayName child-account, got %#v", got)
			}
			if _, ok := payload["label"]; ok {
				t.Fatal("expected legacy label field to be omitted")
			}

			_ = json.NewEncoder(w).Encode(PartnerAccountResponse{
				ProfileID: 55,
				Account:   "0xabc",
			})
		})

		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)

		service := NewPartnerAccountService(NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("api-key")))
		resp, err := service.CreateAccount(context.Background(), CreatePartnerAccountInput{
			DisplayName: "child-account",
		}, &CreatePartnerAccountEOAHeaders{
			Account:        "0xabc",
			SigningMessage: "0x1234",
			Signature:      "0xbeef",
		})
		if err != nil {
			t.Fatalf("CreateAccount returned error: %v", err)
		}
		if resp.ProfileID != 55 {
			t.Fatalf("expected profile 55, got %d", resp.ProfileID)
		}
	})

	t.Run("requires EOA headers when not creating server wallet", func(t *testing.T) {
		t.Parallel()

		service := NewPartnerAccountService(NewHttpClient(WithBaseURL("https://example.com"), WithAPIKey("api-key")))
		_, err := service.CreateAccount(context.Background(), CreatePartnerAccountInput{}, nil)
		if err == nil {
			t.Fatal("expected CreateAccount to fail without EOA headers")
		}
	})

	t.Run("server wallet mode omits EOA headers", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/profiles/partner-accounts", func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("x-account"); got != "" {
				t.Fatalf("expected x-account to be omitted in server-wallet mode, got %q", got)
			}
			if got := r.Header.Get("x-signing-message"); got != "" {
				t.Fatalf("expected x-signing-message to be omitted in server-wallet mode, got %q", got)
			}
			if got := r.Header.Get("x-signature"); got != "" {
				t.Fatalf("expected x-signature to be omitted in server-wallet mode, got %q", got)
			}

			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("failed to decode request body: %v", err)
			}
			if got := payload["displayName"]; got != "server-wallet" {
				t.Fatalf("expected displayName server-wallet, got %#v", got)
			}
			if got := payload["createServerWallet"]; got != true {
				t.Fatalf("expected createServerWallet true, got %#v", got)
			}
			if _, ok := payload["label"]; ok {
				t.Fatal("expected legacy label field to be omitted")
			}

			_ = json.NewEncoder(w).Encode(PartnerAccountResponse{
				ProfileID: 77,
				Account:   "0xserver",
			})
		})

		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)

		service := NewPartnerAccountService(NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("api-key")))
		resp, err := service.CreateAccount(context.Background(), CreatePartnerAccountInput{
			DisplayName:        "server-wallet",
			CreateServerWallet: boolPtr(true),
		}, nil)
		if err != nil {
			t.Fatalf("CreateAccount returned error: %v", err)
		}
		if resp.ProfileID != 77 {
			t.Fatalf("expected profile 77, got %d", resp.ProfileID)
		}
	})

	t.Run("rejects display names longer than 44 characters", func(t *testing.T) {
		t.Parallel()

		service := NewPartnerAccountService(NewHttpClient(WithBaseURL("https://example.com"), WithAPIKey("api-key")))
		_, err := service.CreateAccount(context.Background(), CreatePartnerAccountInput{
			DisplayName:        strings.Repeat("x", 45),
			CreateServerWallet: boolPtr(true),
		}, nil)
		if err == nil {
			t.Fatal("expected CreateAccount to reject long displayName")
		}
		if got := err.Error(); got != "displayName must be at most 44 characters" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("surfaces duplicate profile address as conflict error", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/profiles/partner-accounts", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"message": "A profile already exists for this address",
			})
		})

		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)

		service := NewPartnerAccountService(NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("api-key")))
		_, err := service.CreateAccount(context.Background(), CreatePartnerAccountInput{
			DisplayName: "duplicate-address",
		}, &CreatePartnerAccountEOAHeaders{
			Account:        "0xabc",
			SigningMessage: "0x1234",
			Signature:      "0xbeef",
		})
		if err == nil {
			t.Fatal("expected conflict error")
		}
		conflictErr, ok := err.(*ConflictError)
		if !ok {
			t.Fatalf("expected ConflictError, got %T", err)
		}
		if conflictErr.Message != "A profile already exists for this address" {
			t.Fatalf("unexpected conflict message: %q", conflictErr.Message)
		}
	})

	t.Run("surfaces self-address rejection as validation error", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/profiles/partner-accounts", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"message": "Cannot create a partner account for the partner's own address",
			})
		})

		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)

		service := NewPartnerAccountService(NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("api-key")))
		_, err := service.CreateAccount(context.Background(), CreatePartnerAccountInput{
			DisplayName: "same-as-partner",
		}, &CreatePartnerAccountEOAHeaders{
			Account:        "0xpartner",
			SigningMessage: "0x1234",
			Signature:      "0xbeef",
		})
		if err == nil {
			t.Fatal("expected validation error")
		}
		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("expected ValidationError, got %T", err)
		}
		if validationErr.Message != "Cannot create a partner account for the partner's own address" {
			t.Fatalf("unexpected validation message: %q", validationErr.Message)
		}
	})
}
