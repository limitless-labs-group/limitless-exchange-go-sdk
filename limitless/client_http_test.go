package limitless

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
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
	mux.HandleFunc("/conflict", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"duplicate request"}`))
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

	err = client.Get(context.Background(), "/conflict", &out)
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected ConflictError, got %T (%v)", err, err)
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

func TestHttpClient_HMACPatchAndIdentityOverride(t *testing.T) {
	t.Parallel()

	secret := "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="

	mux := http.NewServeMux()
	mux.HandleFunc("/patch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH method, got %s", r.Method)
		}
		if got := r.Header.Get("X-API-Key"); got != "" {
			t.Fatalf("expected X-API-Key to be suppressed when HMAC is configured, got %q", got)
		}
		if got := r.Header.Get("lmts-api-key"); got != "token-1" {
			t.Fatalf("expected lmts-api-key token-1, got %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read patch body: %v", err)
		}

		expectedSig, err := computeHMACSignature(secret, r.Header.Get("lmts-timestamp"), http.MethodPatch, r.URL.RequestURI(), string(body))
		if err != nil {
			t.Fatalf("failed to compute expected signature: %v", err)
		}
		if got := r.Header.Get("lmts-signature"); got != expectedSig {
			t.Fatalf("expected lmts-signature %q, got %q", expectedSig, got)
		}

		_ = json.NewEncoder(w).Encode(map[string]bool{"patched": true})
	})
	mux.HandleFunc("/identity", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("identity"); got != "Bearer privy-token" {
			dump, _ := httputil.DumpRequest(r, false)
			t.Fatalf("expected identity header, got request:\n%s", string(dump))
		}
		if got := r.Header.Get("X-API-Key"); got != "" {
			t.Fatalf("expected X-API-Key to be suppressed, got %q", got)
		}
		if got := r.Header.Get("lmts-api-key"); got != "" {
			t.Fatalf("expected lmts-api-key to be suppressed, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/partner", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-account"); got != "0xabc" {
			t.Fatalf("expected x-account header, got %q", got)
		}
		if got := r.Header.Get("x-signing-message"); got != "0x1234" {
			t.Fatalf("expected x-signing-message header, got %q", got)
		}
		if got := r.Header.Get("x-signature"); got != "0xbeef" {
			t.Fatalf("expected x-signature header, got %q", got)
		}
		if got := r.Header.Get("lmts-api-key"); got != "token-1" {
			t.Fatalf("expected lmts-api-key token-1, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewHttpClient(
		WithBaseURL(srv.URL),
		WithAPIKey("api-key-value"),
		WithHMACCredentials(HMACCredentials{TokenID: "token-1", Secret: secret}),
	)

	var patchResp struct {
		Patched bool `json:"patched"`
	}
	if err := client.Patch(context.Background(), "/patch", map[string]string{"value": "patched"}, &patchResp); err != nil {
		t.Fatalf("Patch returned error: %v", err)
	}
	if !patchResp.Patched {
		t.Fatal("expected patched=true")
	}

	var identityResp struct {
		Status string `json:"status"`
	}
	if err := client.GetWithIdentity(context.Background(), "/identity", "privy-token", &identityResp); err != nil {
		t.Fatalf("GetWithIdentity returned error: %v", err)
	}
	if identityResp.Status != "ok" {
		t.Fatalf("expected status ok, got %q", identityResp.Status)
	}

	if err := client.PostWithHeaders(context.Background(), "/partner", map[string]string{"mode": "eoa"}, map[string]string{
		"x-account":         "0xabc",
		"x-signing-message": "0x1234",
		"x-signature":       "0xbeef",
	}, &identityResp); err != nil {
		t.Fatalf("PostWithHeaders returned error: %v", err)
	}
}
