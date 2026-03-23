package limitless

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMarketPageFetcher_GetMarketPageByPath_FollowsRedirect(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/market-pages/by-path", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("path") {
		case "/sports":
			w.Header().Set("Location", "/market-pages/by-path?path=%2Fsports%2Fnba")
			w.WriteHeader(http.StatusMovedPermanently)
		case "/sports/nba":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       "page-1",
				"name":     "NBA",
				"slug":     "nba",
				"fullPath": "/sports/nba",
			})
		default:
			t.Fatalf("unexpected path lookup: %s", r.URL.RawQuery)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(WithBaseURL(srv.URL))
	fetcher := NewMarketPageFetcher(client)

	page, err := fetcher.GetMarketPageByPath(context.Background(), "/sports")
	if err != nil {
		t.Fatalf("GetMarketPageByPath returned error: %v", err)
	}
	if page.ID != "page-1" || page.FullPath != "/sports/nba" {
		t.Fatalf("unexpected page: %+v", page)
	}
}

func TestMarketPageFetcher_GetMarkets_UsesEscapedPageIDAndFilters(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/market-pages/", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); got != "/market-pages/page%2Fwith%20space/markets" {
			t.Fatalf("unexpected escaped path: %s", got)
		}
		q := r.URL.Query()
		if q.Get("page") != "2" || q.Get("limit") != "20" || q.Get("sort") != "-updatedAt" {
			t.Fatalf("unexpected pagination query: %s", r.URL.RawQuery)
		}
		if got := q["sports"]; len(got) != 2 {
			t.Fatalf("expected repeated sports filter values, got %v", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []any{},
			"pagination": map[string]any{
				"page":       2,
				"limit":      20,
				"total":      0,
				"totalPages": 0,
			},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(WithBaseURL(srv.URL))
	fetcher := NewMarketPageFetcher(client)
	page := 2
	limit := 20

	resp, err := fetcher.GetMarkets(context.Background(), "page/with space", &MarketPageMarketsParams{
		Page:    &page,
		Limit:   &limit,
		Sort:    MarketPageSort("-updatedAt"),
		Filters: map[string]any{"sports": []any{"nba", "mlb"}},
	})
	if err != nil {
		t.Fatalf("GetMarkets returned error: %v", err)
	}
	if resp.Pagination == nil || resp.Pagination.Page != 2 {
		t.Fatalf("unexpected pagination: %+v", resp.Pagination)
	}
}

func TestMarketPageFetcher_GetPropertyEndpoints_EscapeIDs(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/property-keys/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/property-keys/key%2Fwith%20space":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":   "key/with space",
				"name": "League",
				"slug": "league",
				"type": "select",
			})
		case "/property-keys/key%2Fwith%20space/options":
			if r.URL.Query().Get("parentId") != "parent/value" {
				t.Fatalf("unexpected parentId query: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "opt-1", "propertyKeyId": "key/with space", "value": "NBA", "label": "NBA"},
			})
		default:
			t.Fatalf("unexpected property key path: %s", r.URL.EscapedPath())
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(WithBaseURL(srv.URL))
	fetcher := NewMarketPageFetcher(client)

	key, err := fetcher.GetPropertyKey(context.Background(), "key/with space")
	if err != nil {
		t.Fatalf("GetPropertyKey returned error: %v", err)
	}
	if key.ID != "key/with space" {
		t.Fatalf("unexpected key ID: %s", key.ID)
	}

	parentID := "parent/value"
	options, err := fetcher.GetPropertyOptions(context.Background(), "key/with space", &parentID)
	if err != nil {
		t.Fatalf("GetPropertyOptions returned error: %v", err)
	}
	if len(options) != 1 || options[0].ID != "opt-1" {
		t.Fatalf("unexpected options: %+v", options)
	}
}
