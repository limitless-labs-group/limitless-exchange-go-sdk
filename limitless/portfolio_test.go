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

	client := NewHttpClient(WithBaseURL(srv.URL))
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

	client := NewHttpClient(WithBaseURL(srv.URL))
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

func TestPortfolioFetcher_GetUserHistory_DefaultPagination(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/portfolio/history", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("page") != "1" || q.Get("limit") != "10" {
			t.Fatalf("expected default pagination page=1 limit=10, got query=%s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":       []any{},
			"totalCount": 0,
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(WithBaseURL(srv.URL))
	fetcher := NewPortfolioFetcher(client)

	_, err := fetcher.GetUserHistory(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("GetUserHistory returned error: %v", err)
	}
}
