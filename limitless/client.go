package limitless

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

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
	hmacCreds  *HMACCredentials
	logger     Logger
	headers    map[string]string
}

// NewHttpClient creates a new HTTP client with the given options.
func NewHttpClient(opts ...ClientOption) *HttpClient {
	transport := &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     60 * time.Second,
	}
	headers := sdkTrackingHeaders()
	headers["Content-Type"] = "application/json"
	headers["Accept"] = "application/json"

	c := &HttpClient{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		baseURL: DefaultAPIURL,
		logger:  NewNoOpLogger(),
		headers: headers,
	}

	for _, opt := range opts {
		opt(c)
	}

	// Auto-load LIMITLESS_API_KEY for v1 compatibility when no explicit API key was provided.
	if c.apiKey == "" {
		c.apiKey = os.Getenv("LIMITLESS_API_KEY")
	}

	if c.apiKey == "" && c.hmacCreds == nil {
		c.logger.Warn("Authentication not set. Authenticated endpoints will fail. " +
			"Pass WithAPIKey(...) or WithHMACCredentials(...) explicitly, or set LIMITLESS_API_KEY for environment-based auto-loading.")
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

// SetHMACCredentials sets HMAC credentials for authenticated requests.
func (c *HttpClient) SetHMACCredentials(creds HMACCredentials) {
	c.hmacCreds = &HMACCredentials{
		TokenID: creds.TokenID,
		Secret:  creds.Secret,
	}
}

// ClearHMACCredentials removes HMAC credentials.
func (c *HttpClient) ClearHMACCredentials() {
	c.hmacCreds = nil
}

// HMACCredentials returns a copy of the current HMAC credentials.
func (c *HttpClient) HMACCredentials() *HMACCredentials {
	if c.hmacCreds == nil {
		return nil
	}
	creds := *c.hmacCreds
	return &creds
}

func (c *HttpClient) requireAuth(operation string) error {
	if c.apiKey != "" || c.hmacCreds != nil {
		return nil
	}
	return fmt.Errorf(
		"authentication is required for %s; pass WithAPIKey(...) or WithHMACCredentials(...) when creating the client, or set LIMITLESS_API_KEY for environment-based auto-loading",
		operation,
	)
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

// Patch performs a PATCH request with the given body and decodes the response into result.
func (c *HttpClient) Patch(ctx context.Context, path string, body, result any) error {
	return c.doRequest(ctx, http.MethodPatch, path, body, result)
}

// Delete performs a DELETE request and decodes the response into result.
func (c *HttpClient) Delete(ctx context.Context, path string, result any) error {
	return c.doRequestWithConfig(ctx, http.MethodDelete, path, nil, requestExecutionConfig{}, result)
}

// PostWithHeaders performs a POST request with additional per-request headers.
func (c *HttpClient) PostWithHeaders(ctx context.Context, path string, body any, extraHeaders map[string]string, result any) error {
	return c.doRequestWithConfig(ctx, http.MethodPost, path, body, requestExecutionConfig{
		extraHeaders: extraHeaders,
	}, result)
}

// PostWithIdentity performs a POST request authenticated with a Privy identity token.
func (c *HttpClient) PostWithIdentity(ctx context.Context, path string, identityToken string, body, result any) error {
	return c.doRequestWithIdentity(ctx, http.MethodPost, path, identityToken, body, result)
}

// GetWithIdentity performs a GET request authenticated with a Privy identity token.
func (c *HttpClient) GetWithIdentity(ctx context.Context, path string, identityToken string, result any) error {
	return c.doRequestWithIdentity(ctx, http.MethodGet, path, identityToken, nil, result)
}

// RequestOption configures a raw HTTP request.
type RequestOption func(*requestConfig)

type requestConfig struct {
	allowedStatuses []int
}

type requestExecutionConfig struct {
	extraHeaders  map[string]string
	identityToken string
}

// AllowStatus adds an HTTP status code to the list of acceptable response statuses.
func AllowStatus(code int) RequestOption {
	return func(cfg *requestConfig) {
		cfg.allowedStatuses = append(cfg.allowedStatuses, code)
	}
}

func (c *HttpClient) doRequest(ctx context.Context, method, path string, body, result any) error {
	return c.doRequestWithConfig(ctx, method, path, body, requestExecutionConfig{}, result)
}

func (c *HttpClient) doRequestWithIdentity(ctx context.Context, method, path string, identityToken string, body, result any) error {
	return c.doRequestWithConfig(ctx, method, path, body, requestExecutionConfig{
		identityToken: identityToken,
	}, result)
}

func (c *HttpClient) doRequestRaw(ctx context.Context, method, path string, body any, opts ...RequestOption) (*RawResponse, error) {
	cfg := &requestConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	bodyReader, bodyBytes, err := buildRequestBody(body)
	if err != nil {
		return nil, err
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

	if err := c.signAndSetHeaders(req, bodyBytes, ""); err != nil {
		return nil, err
	}
	if method == http.MethodDelete {
		req.Header.Del("Content-Type")
	}

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

func buildRequestBody(body any) (io.Reader, []byte, error) {
	if body == nil {
		return nil, nil, nil
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	return bytes.NewReader(bodyBytes), bodyBytes, nil
}

func (c *HttpClient) doRequestWithConfig(
	ctx context.Context,
	method,
	path string,
	body any,
	cfg requestExecutionConfig,
	result any,
) error {
	bodyReader, bodyBytes, err := buildRequestBody(body)
	if err != nil {
		return err
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if err := c.signAndSetHeaders(req, bodyBytes, cfg.identityToken); err != nil {
		return err
	}
	for k, v := range cfg.extraHeaders {
		req.Header.Set(k, v)
	}
	if method == http.MethodDelete {
		req.Header.Del("Content-Type")
	}

	logMeta := map[string]any{
		"headers": c.maskedHeadersForLog(cfg.extraHeaders, cfg.identityToken),
	}
	if len(bodyBytes) > 0 {
		logMeta["body"] = string(bodyBytes)
	}
	c.logger.Debug(fmt.Sprintf("→ %s %s", method, url), logMeta)

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

	c.logger.Debug(fmt.Sprintf("✓ %d %s %s", resp.StatusCode, method, path), map[string]any{
		"body": string(respBody),
	})

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

func (c *HttpClient) signAndSetHeaders(req *http.Request, bodyBytes []byte, overrideIdentityToken string) error {
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	if overrideIdentityToken != "" {
		req.Header.Set("identity", "Bearer "+overrideIdentityToken)
		return nil
	}

	if c.hmacCreds != nil {
		body := ""
		if len(bodyBytes) > 0 {
			body = string(bodyBytes)
		}
		timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		signature, err := computeHMACSignature(c.hmacCreds.Secret, timestamp, req.Method, req.URL.RequestURI(), body)
		if err != nil {
			return fmt.Errorf("failed to compute HMAC signature: %w", err)
		}
		req.Header.Set("lmts-api-key", c.hmacCreds.TokenID)
		req.Header.Set("lmts-timestamp", timestamp)
		req.Header.Set("lmts-signature", signature)
		return nil
	}

	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	return nil
}

func (c *HttpClient) maskedHeadersForLog(extraHeaders map[string]string, identityToken string) map[string]string {
	logHeaders := make(map[string]string, len(c.headers)+len(extraHeaders)+4)
	for k, v := range c.headers {
		logHeaders[k] = v
	}

	if identityToken != "" {
		logHeaders["identity"] = "Bearer ***"
	} else if c.hmacCreds != nil {
		logHeaders["lmts-api-key"] = "***"
		logHeaders["lmts-timestamp"] = "***"
		logHeaders["lmts-signature"] = "***"
	} else if c.apiKey != "" {
		logHeaders["X-API-Key"] = "***"
	}

	for k, v := range extraHeaders {
		if isSensitiveHeader(k) {
			logHeaders[k] = "***"
			continue
		}
		logHeaders[k] = v
	}

	return logHeaders
}

func isSensitiveHeader(name string) bool {
	switch strings.ToLower(name) {
	case "authorization", "identity", "x-api-key", "lmts-api-key", "lmts-timestamp", "lmts-signature", "x-signature":
		return true
	default:
		return false
	}
}
