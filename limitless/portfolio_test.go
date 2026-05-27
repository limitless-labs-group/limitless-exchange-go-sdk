package limitless

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPortfolioFetcher_GetProfile(t *testing.T) {
	t.Parallel()

	address := "0xa00BCB04073B243E8A55f3B5899AefF596bF17C6"

	mux := http.NewServeMux()
	mux.HandleFunc("/profiles/"+address, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      42,
			"account": address,
			"rank": map[string]any{
				"id":         1,
				"name":       "Gold",
				"feeRateBps": 250,
			},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("test-key"))
	fetcher := NewPortfolioFetcher(client)

	profile, err := fetcher.GetProfile(context.Background(), address)
	if err != nil {
		t.Fatalf("GetProfile returned error: %v", err)
	}
	if profile.ID != 42 {
		t.Fatalf("expected profile id 42, got %d", profile.ID)
	}
	if profile.Rank == nil || profile.Rank.FeeRateBps != 250 {
		t.Fatalf("expected rank feeRateBps=250, got %+v", profile.Rank)
	}
}

func TestPortfolioFetcher_GetCurrentProfile(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/profiles/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET method, got %s", r.Method)
		}
		if got := r.URL.Path; got != "/profiles/me" {
			t.Fatalf("expected /profiles/me path, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      77,
			"account": "0xa00BCB04073B243E8A55f3B5899AefF596bF17C6",
			"rank": map[string]any{
				"id":         2,
				"name":       "Platinum",
				"feeRateBps": 200,
			},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(WithBaseURL(srv.URL), WithHMACCredentials(HMACCredentials{
		TokenID: "token-1",
		Secret:  "c2VjcmV0",
	}))
	fetcher := NewPortfolioFetcher(client)

	profile, err := fetcher.GetCurrentProfile(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentProfile returned error: %v", err)
	}
	if profile.ID != 77 {
		t.Fatalf("expected profile id 77, got %d", profile.ID)
	}
	if profile.Rank == nil || profile.Rank.FeeRateBps != 200 {
		t.Fatalf("expected rank feeRateBps=200, got %+v", profile.Rank)
	}
}

func TestPortfolioFetcher_GetPositionsAndSlices(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/portfolio/positions", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"amm":   nil,
			"clob":  nil,
			"group": []any{},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("test-key"))
	fetcher := NewPortfolioFetcher(client)

	resp, err := fetcher.GetPositions(context.Background())
	if err != nil {
		t.Fatalf("GetPositions returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil positions response")
	}

	clob, err := fetcher.GetCLOBPositions(context.Background())
	if err != nil {
		t.Fatalf("GetCLOBPositions returned error: %v", err)
	}
	if len(clob) != 0 {
		t.Fatalf("expected empty CLOB positions, got %d", len(clob))
	}

	amm, err := fetcher.GetAMMPositions(context.Background())
	if err != nil {
		t.Fatalf("GetAMMPositions returned error: %v", err)
	}
	if len(amm) != 0 {
		t.Fatalf("expected empty AMM positions, got %d", len(amm))
	}
}

func TestPortfolioFetcher_GetUserHistory_CursorPagination(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/portfolio/history", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if !q.Has("cursor") {
			t.Fatal("expected cursor query parameter to be present")
		}
		if q.Get("cursor") != "" {
			t.Fatalf("expected empty cursor on first page, got cursor=%q", q.Get("cursor"))
		}
		if r.URL.RawQuery != "cursor=&limit=20" {
			t.Fatalf("expected first-page raw query cursor=&limit=20, got %s", r.URL.RawQuery)
		}
		if q.Get("limit") != "20" {
			t.Fatalf("expected default limit=20, got limit=%s", q.Get("limit"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":       []any{},
			"nextCursor": nil,
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("test-key"))
	fetcher := NewPortfolioFetcher(client)

	_, err := fetcher.GetUserHistory(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("GetUserHistory returned error: %v", err)
	}
}

func TestPortfolioFetcher_AuthenticatedMethods_RequireAPIKey(t *testing.T) {
	t.Parallel()

	client := NewHttpClient(WithBaseURL("https://example.com"))
	fetcher := NewPortfolioFetcher(client)

	if _, err := fetcher.GetProfile(context.Background(), "0xa00BCB04073B243E8A55f3B5899AefF596bF17C6"); err == nil {
		t.Fatal("expected GetProfile to fail without API key")
	}
	if _, err := fetcher.GetCurrentProfile(context.Background()); err == nil {
		t.Fatal("expected GetCurrentProfile to fail without API key")
	}
	if _, err := fetcher.GetPositions(context.Background()); err == nil {
		t.Fatal("expected GetPositions to fail without API key")
	}
	if _, err := fetcher.GetUserHistory(context.Background(), "", 10); err == nil {
		t.Fatal("expected GetUserHistory to fail without API key")
	}
}
