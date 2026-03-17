package limitless

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHttpClient_Get_Post_Delete(t *testing.T) {
	t.Parallel()

	type postPayload struct {
		Value string `json:"value"`
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET method, got %s", r.Method)
		}
		if got := r.Header.Get("X-API-Key"); got != "test-key" {
			t.Fatalf("expected X-API-Key=test-key, got %q", got)
		}
		if got := strings.ToLower(r.Header.Get("X-SDK-Version")); !strings.HasPrefix(got, "lmts-sdk-go/") {
			t.Fatalf("expected x-sdk-version header, got %q", got)
		}
		if got := strings.ToLower(r.Header.Get("User-Agent")); !strings.Contains(got, "lmts-sdk-go/") {
			t.Fatalf("expected user-agent header to include lmts-sdk-go/, got %q", got)
		}

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read post body: %v", err)
		}
		var payload postPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to parse post body: %v", err)
		}
		if payload.Value != "hello" {
			t.Fatalf("expected payload value hello, got %q", payload.Value)
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"created": true})
	})

	mux.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE method, got %s", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "" {
			t.Fatalf("expected empty Content-Type for DELETE, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "deleted"})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(WithBaseURL(srv.URL), WithAPIKey("test-key"))
	ctx := context.Background()

	var getResp struct {
		Status string `json:"status"`
	}
	if err := client.Get(ctx, "/get", &getResp); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if getResp.Status != "ok" {
		t.Fatalf("expected status ok, got %q", getResp.Status)
	}

	var postResp struct {
		Created bool `json:"created"`
	}
	if err := client.Post(ctx, "/post", postPayload{Value: "hello"}, &postResp); err != nil {
		t.Fatalf("Post returned error: %v", err)
	}
	if !postResp.Created {
		t.Fatalf("expected created=true, got false")
	}

	var deleteResp struct {
		Message string `json:"message"`
	}
	if err := client.Delete(ctx, "/delete", &deleteResp); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if deleteResp.Message != "deleted" {
		t.Fatalf("expected message deleted, got %q", deleteResp.Message)
	}
}

func TestHttpClient_TypedErrorParsing(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/rate", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"too many requests"}`))
	})
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	})
	mux.HandleFunc("/validation", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid payload"}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(WithBaseURL(srv.URL))

	var out map[string]any
	err := client.Get(context.Background(), "/rate", &out)
	var rateErr *RateLimitError
	if !errors.As(err, &rateErr) {
		t.Fatalf("expected RateLimitError, got %T (%v)", err, err)
	}

	err = client.Get(context.Background(), "/auth", &out)
	var authErr *AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthenticationError, got %T (%v)", err, err)
	}

	err = client.Get(context.Background(), "/validation", &out)
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T (%v)", err, err)
	}
}

func TestHttpClient_GetRaw_StatusHandling(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/new-path")
		w.WriteHeader(http.StatusMovedPermanently)
	})
	mux.HandleFunc("/not-found", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(WithBaseURL(srv.URL))
	ctx := context.Background()

	raw, err := client.GetRaw(ctx, "/redirect", AllowStatus(http.StatusMovedPermanently))
	if err != nil {
		t.Fatalf("GetRaw(301) returned error: %v", err)
	}
	if raw.Status != http.StatusMovedPermanently {
		t.Fatalf("expected status 301, got %d", raw.Status)
	}
	if got := raw.Headers.Get("Location"); got != "/new-path" {
		t.Fatalf("expected location /new-path, got %q", got)
	}

	_, err = client.GetRaw(ctx, "/not-found")
	if err == nil {
		t.Fatal("expected GetRaw 404 error, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T (%v)", err, err)
	}
	if apiErr.Status != http.StatusNotFound {
		t.Fatalf("expected 404 status, got %d", apiErr.Status)
	}
}
