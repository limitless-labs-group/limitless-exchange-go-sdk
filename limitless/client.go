package limitless

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"
)

const sdkID = "lmts-sdk-go"

// sdkVersion is injected at release build time via -ldflags.
var sdkVersion = "0.0.0"

func resolveSDKVersion() string {
	if sdkVersion != "" {
		return sdkVersion
	}
	return "0.0.0"
}

// RawResponse holds the HTTP response metadata alongside the decoded body.
type RawResponse struct {
	Status  int
	Headers http.Header
	Body    json.RawMessage
}

// HttpClient is the base HTTP client for the Limitless Exchange API.
type HttpClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	logger     Logger
	headers    map[string]string
}

// NewHttpClient creates a new HTTP client with the given options.
func NewHttpClient(opts ...ClientOption) *HttpClient {
	sdkToken := fmt.Sprintf("%s/%s", sdkID, resolveSDKVersion())
	userAgent := fmt.Sprintf("%s (go/%s)", sdkToken, runtime.Version())
	transport := &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     60 * time.Second,
	}

	c := &HttpClient{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		baseURL: DefaultAPIURL,
		logger:  NewNoOpLogger(),
		headers: map[string]string{
			"user-agent":    userAgent,
			"x-sdk-version": sdkToken,
			"Content-Type":  "application/json",
			"Accept":        "application/json",
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	// Auto-load API key from environment if not set
	if c.apiKey == "" {
		c.apiKey = os.Getenv("LIMITLESS_API_KEY")
	}

	if c.apiKey == "" {
		c.logger.Warn("API key not set. Authenticated endpoints will fail. " +
			"Set LIMITLESS_API_KEY environment variable or pass WithAPIKey option.")
	}

	c.logger.Debug("HTTP client initialized", map[string]any{
		"baseURL": c.baseURL,
	})

	return c
}

// SetAPIKey sets the API key for authenticated requests.
func (c *HttpClient) SetAPIKey(key string) {
	c.apiKey = key
}

// ClearAPIKey removes the API key.
func (c *HttpClient) ClearAPIKey() {
	c.apiKey = ""
}

// APIKey returns the current API key.
func (c *HttpClient) APIKey() string {
	return c.apiKey
}

// Get performs a GET request and decodes the response into result.
func (c *HttpClient) Get(ctx context.Context, path string, result any) error {
	return c.doRequest(ctx, http.MethodGet, path, nil, result)
}

// GetRaw performs a GET request and returns the raw response with status code and headers.
// The caller provides optional request options (e.g. to allow 301 status).
func (c *HttpClient) GetRaw(ctx context.Context, path string, opts ...RequestOption) (*RawResponse, error) {
	return c.doRequestRaw(ctx, http.MethodGet, path, nil, opts...)
}

// Post performs a POST request with the given body and decodes the response into result.
func (c *HttpClient) Post(ctx context.Context, path string, body, result any) error {
	return c.doRequest(ctx, http.MethodPost, path, body, result)
}

// Delete performs a DELETE request and decodes the response into result.
func (c *HttpClient) Delete(ctx context.Context, path string, result any) error {
	return c.doRequestDelete(ctx, path, result)
}

// RequestOption configures a raw HTTP request.
type RequestOption func(*requestConfig)

type requestConfig struct {
	allowedStatuses []int
}

// AllowStatus adds an HTTP status code to the list of acceptable response statuses.
func AllowStatus(code int) RequestOption {
	return func(cfg *requestConfig) {
		cfg.allowedStatuses = append(cfg.allowedStatuses, code)
	}
}

func (c *HttpClient) doRequest(ctx context.Context, method, path string, body, result any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	logHeaders := make(map[string]string)
	for k, v := range c.headers {
		logHeaders[k] = v
	}
	if c.apiKey != "" {
		logHeaders["X-API-Key"] = "***"
	}
	c.logger.Debug(fmt.Sprintf("→ %s %s", method, url), map[string]any{
		"headers": logHeaders,
	})

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		c.logger.Debug(fmt.Sprintf("✗ %d %s %s", resp.StatusCode, method, path), map[string]any{
			"error": string(respBody),
		})
		return parseAPIError(resp.StatusCode, respBody, path, method)
	}

	c.logger.Debug(fmt.Sprintf("✓ %d %s %s", resp.StatusCode, method, path))

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

func (c *HttpClient) doRequestDelete(ctx context.Context, path string, result any) error {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)
	// Remove Content-Type for DELETE (no body)
	req.Header.Del("Content-Type")

	c.logger.Debug(fmt.Sprintf("→ DELETE %s", url))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return parseAPIError(resp.StatusCode, respBody, path, http.MethodDelete)
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

func (c *HttpClient) doRequestRaw(ctx context.Context, method, path string, body any, opts ...RequestOption) (*RawResponse, error) {
	cfg := &requestConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	url := c.baseURL + path

	// Use a transport-level redirect policy to prevent auto-follow
	client := *c.httpClient
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check if this status is allowed
	isAllowed := resp.StatusCode >= 200 && resp.StatusCode < 400
	for _, s := range cfg.allowedStatuses {
		if resp.StatusCode == s {
			isAllowed = true
			break
		}
	}

	if resp.StatusCode >= 400 && !isAllowed {
		return nil, parseAPIError(resp.StatusCode, respBody, path, method)
	}

	return &RawResponse{
		Status:  resp.StatusCode,
		Headers: resp.Header,
		Body:    json.RawMessage(respBody),
	}, nil
}

func (c *HttpClient) setHeaders(req *http.Request) {
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
}
