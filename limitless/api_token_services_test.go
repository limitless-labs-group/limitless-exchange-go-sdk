package limitless

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestApiTokenServices_Requests(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/api-tokens/derive", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("identity"); got != "Bearer privy-token" {
			t.Fatalf("expected identity header, got %q", got)
		}
		if got := r.Header.Get("X-API-Key"); got != "" {
			t.Fatalf("expected X-API-Key to be suppressed, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(DeriveApiTokenResponse{
			ApiKey:    "token-1",
			Secret:    "secret-1",
			TokenID:   "token-1",
			CreatedAt: "2026-03-23T12:00:00.000Z",
			Scopes:    []string{ScopeTrading},
			Profile:   ApiTokenProfile{ID: 7, Account: "0xabc"},
		})
	})
	mux.HandleFunc("/auth/api-tokens/capabilities", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("identity"); got != "Bearer privy-token" {
			t.Fatalf("expected identity header, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(PartnerCapabilities{
			PartnerProfileID:       7,
			TokenManagementEnabled: true,
			AllowedScopes:          []string{ScopeTrading},
		})
	})
	mux.HandleFunc("/auth/api-tokens", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("lmts-api-key"); got != "token-1" {
			t.Fatalf("expected lmts-api-key token-1, got %q", got)
		}
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode([]ApiToken{{TokenID: "token-1", Scopes: []string{ScopeTrading}, CreatedAt: "2026-03-23T12:00:00.000Z"}})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})
	mux.HandleFunc("/auth/api-tokens/token-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "API token revoked successfully"})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(
		WithBaseURL(srv.URL),
		WithHMACCredentials(HMACCredentials{
			TokenID: "token-1",
			Secret:  "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=",
		}),
	)

	apiTokens := NewApiTokenService(client)
	derived, err := apiTokens.DeriveToken(context.Background(), "privy-token", DeriveApiTokenInput{Scopes: []string{ScopeTrading}})
	if err != nil {
		t.Fatalf("DeriveToken returned error: %v", err)
	}
	if derived.TokenID != "token-1" {
		t.Fatalf("expected token-1, got %s", derived.TokenID)
	}

	capabilities, err := apiTokens.GetCapabilities(context.Background(), "privy-token")
	if err != nil {
		t.Fatalf("GetCapabilities returned error: %v", err)
	}
	if !capabilities.TokenManagementEnabled {
		t.Fatal("expected token management enabled")
	}

	tokens, err := apiTokens.ListTokens(context.Background())
	if err != nil {
		t.Fatalf("ListTokens returned error: %v", err)
	}
	if len(tokens) != 1 || tokens[0].TokenID != "token-1" {
		t.Fatalf("unexpected token list: %+v", tokens)
	}

	msg, err := apiTokens.RevokeToken(context.Background(), "token-1")
	if err != nil {
		t.Fatalf("RevokeToken returned error: %v", err)
	}
	if !strings.Contains(msg, "revoked") {
		t.Fatalf("unexpected revoke message %q", msg)
	}
}
