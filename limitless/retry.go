package limitless

import (
	"context"
	"errors"
	"io"
	"math"
	"net"
	neturl "net/url"
	"time"
)

// RetryConfig configures retry behavior.
type RetryConfig struct {
	// StatusCodes are the HTTP status codes that trigger a retry.
	// Default: [429, 500, 502, 503, 504]
	StatusCodes []int

	// MaxRetries is the maximum number of retry attempts.
	// Default: 3
	MaxRetries int

	// Delays is a list of fixed delays in seconds for each retry attempt.
	// If set, overrides exponential backoff.
	Delays []time.Duration

	// ExponentialBase is the base for exponential backoff calculation.
	// Default: 2.0
	ExponentialBase float64

	// MaxDelay is the maximum delay for exponential backoff.
	// Default: 60s
	MaxDelay time.Duration

	// OnRetry is called before each retry attempt.
	OnRetry func(attempt int, err error, delay time.Duration)
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		StatusCodes:     []int{429, 500, 502, 503, 504},
		MaxRetries:      3,
		ExponentialBase: 2.0,
		MaxDelay:        60 * time.Second,
	}
}

// delay returns the delay for the given retry attempt.
func (rc *RetryConfig) delay(attempt int) time.Duration {
	if len(rc.Delays) > 0 {
		idx := attempt
		if idx >= len(rc.Delays) {
			idx = len(rc.Delays) - 1
		}
		return rc.Delays[idx]
	}

	base := rc.ExponentialBase
	if base == 0 {
		base = 2.0
	}

	d := time.Duration(math.Pow(base, float64(attempt)) * float64(time.Second))
	if rc.MaxDelay > 0 && d > rc.MaxDelay {
		d = rc.MaxDelay
	}
	return d
}

func (rc *RetryConfig) shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if isRetryableTransportError(err) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	codes := rc.StatusCodes
	if len(codes) == 0 {
		codes = []int{429, 500, 502, 503, 504}
	}

	status, ok := retryStatusCodeFromError(err)
	if !ok {
		return false
	}

	for _, code := range codes {
		if status == code {
			return true
		}
	}
	return false
}

func retryStatusCodeFromError(err error) (int, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status, true
	}

	var rateErr *RateLimitError
	if errors.As(err, &rateErr) {
		return rateErr.Status, true
	}

	var authErr *AuthenticationError
	if errors.As(err, &authErr) {
		return authErr.Status, true
	}

	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Status, true
	}

	var conflictErr *ConflictError
	if errors.As(err, &conflictErr) {
		return conflictErr.Status, true
	}

	var unprocessableErr *UnprocessableEntityError
	if errors.As(err, &unprocessableErr) {
		return unprocessableErr.Status, true
	}

	var tooEarlyErr *TooEarlyError
	if errors.As(err, &tooEarlyErr) {
		return tooEarlyErr.Status, true
	}

	var upstreamErr *UpstreamUnavailableError
	if errors.As(err, &upstreamErr) {
		return upstreamErr.Status, true
	}

	return 0, false
}

func isRetryableTransportError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	var urlErr *neturl.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return true
		}
		if urlErr.Err == nil {
			return true
		}
		return isRetryableTransportError(urlErr.Err)
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
		type temporary interface {
			Temporary() bool
		}
		var tempErr temporary
		if errors.As(err, &tempErr) && tempErr.Temporary() {
			return true
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	return false
}

// WithRetry executes fn with retry logic based on the given config.
// An optional logger can be provided for retry diagnostics.
func WithRetry[T any](ctx context.Context, fn func() (T, error), config RetryConfig, logger ...Logger) (T, error) {
	var zero T
	l := Logger(NewNoOpLogger())
	if len(logger) > 0 && logger[0] != nil {
		l = logger[0]
	}

	// First attempt
	result, err := fn()
	if err == nil {
		return result, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return zero, ctxErr
	}
	if !config.shouldRetry(err) {
		return zero, err
	}

	lastErr := err
	l.Warn("API error, starting retries", map[string]any{"error": err.Error()})

	// Retry attempts
	maxRetries := config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		d := config.delay(attempt)

		if config.OnRetry != nil {
			config.OnRetry(attempt, lastErr, d)
		}

		l.Info("Retrying operation", map[string]any{"attempt": attempt + 1, "delay": d.String()})

		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(d):
		}

		result, err = fn()
		if err == nil {
			return result, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return zero, ctxErr
		}
		if !config.shouldRetry(err) {
			return zero, err
		}
		lastErr = err
		l.Warn("Retry failed", map[string]any{"attempt": attempt + 1, "error": err.Error()})
	}

	l.Error("All retries exhausted", lastErr)
	return zero, lastErr
}

// RetryableClient wraps an HttpClient with automatic retry logic.
type RetryableClient struct {
	client *HttpClient
	config RetryConfig
	logger Logger
}

// NewRetryableClient creates a new retryable client wrapper.
func NewRetryableClient(client *HttpClient, config RetryConfig, logger ...Logger) *RetryableClient {
	l := Logger(NewNoOpLogger())
	if len(logger) > 0 && logger[0] != nil {
		l = logger[0]
	}
	return &RetryableClient{
		client: client,
		config: config,
		logger: l,
	}
}

// Get performs a GET request with retry logic.
func (rc *RetryableClient) Get(ctx context.Context, path string, result any) error {
	_, err := WithRetry(ctx, func() (struct{}, error) {
		err := rc.client.Get(ctx, path, result)
		return struct{}{}, err
	}, rc.config, rc.logger)
	return err
}

// GetRaw performs a raw GET request with retry logic.
func (rc *RetryableClient) GetRaw(ctx context.Context, path string, opts ...RequestOption) (*RawResponse, error) {
	return WithRetry(ctx, func() (*RawResponse, error) {
		return rc.client.GetRaw(ctx, path, opts...)
	}, rc.config, rc.logger)
}

// PostRaw performs a raw POST request with retry logic.
func (rc *RetryableClient) PostRaw(ctx context.Context, path string, body any, opts ...RequestOption) (*RawResponse, error) {
	return WithRetry(ctx, func() (*RawResponse, error) {
		return rc.client.PostRaw(ctx, path, body, opts...)
	}, rc.config, rc.logger)
}

// DeleteRaw performs a raw DELETE request with retry logic.
func (rc *RetryableClient) DeleteRaw(ctx context.Context, path string, opts ...RequestOption) (*RawResponse, error) {
	return WithRetry(ctx, func() (*RawResponse, error) {
		return rc.client.DeleteRaw(ctx, path, opts...)
	}, rc.config, rc.logger)
}

// Post performs a POST request with retry logic.
func (rc *RetryableClient) Post(ctx context.Context, path string, body, result any) error {
	_, err := WithRetry(ctx, func() (struct{}, error) {
		err := rc.client.Post(ctx, path, body, result)
		return struct{}{}, err
	}, rc.config, rc.logger)
	return err
}

// Patch performs a PATCH request with retry logic.
func (rc *RetryableClient) Patch(ctx context.Context, path string, body, result any) error {
	_, err := WithRetry(ctx, func() (struct{}, error) {
		err := rc.client.Patch(ctx, path, body, result)
		return struct{}{}, err
	}, rc.config, rc.logger)
	return err
}

// Delete performs a DELETE request with retry logic.
func (rc *RetryableClient) Delete(ctx context.Context, path string, result any) error {
	_, err := WithRetry(ctx, func() (struct{}, error) {
		err := rc.client.Delete(ctx, path, result)
		return struct{}{}, err
	}, rc.config, rc.logger)
	return err
}

// SetAPIKey sets the API key on the underlying HTTP client.
func (rc *RetryableClient) SetAPIKey(key string) {
	rc.client.SetAPIKey(key)
}

// ClearAPIKey removes the API key from the underlying HTTP client.
func (rc *RetryableClient) ClearAPIKey() {
	rc.client.ClearAPIKey()
}

// SetHMACCredentials sets HMAC credentials on the underlying HTTP client.
func (rc *RetryableClient) SetHMACCredentials(creds HMACCredentials) {
	rc.client.SetHMACCredentials(creds)
}

// ClearHMACCredentials removes HMAC credentials from the underlying HTTP client.
func (rc *RetryableClient) ClearHMACCredentials() {
	rc.client.ClearHMACCredentials()
}

// HMACCredentials returns the current HMAC credentials from the underlying HTTP client.
func (rc *RetryableClient) HMACCredentials() *HMACCredentials {
	return rc.client.HMACCredentials()
}

// PostWithHeaders performs a POST request with retry logic and extra headers.
func (rc *RetryableClient) PostWithHeaders(ctx context.Context, path string, body any, extraHeaders map[string]string, result any) error {
	_, err := WithRetry(ctx, func() (struct{}, error) {
		err := rc.client.PostWithHeaders(ctx, path, body, extraHeaders, result)
		return struct{}{}, err
	}, rc.config, rc.logger)
	return err
}

// PostWithIdentity performs a POST request with retry logic using a Privy identity token.
func (rc *RetryableClient) PostWithIdentity(ctx context.Context, path string, identityToken string, body, result any) error {
	_, err := WithRetry(ctx, func() (struct{}, error) {
		err := rc.client.PostWithIdentity(ctx, path, identityToken, body, result)
		return struct{}{}, err
	}, rc.config, rc.logger)
	return err
}

// GetWithIdentity performs a GET request with retry logic using a Privy identity token.
func (rc *RetryableClient) GetWithIdentity(ctx context.Context, path string, identityToken string, result any) error {
	_, err := WithRetry(ctx, func() (struct{}, error) {
		err := rc.client.GetWithIdentity(ctx, path, identityToken, result)
		return struct{}{}, err
	}, rc.config, rc.logger)
	return err
}

// DeleteWithIdentity performs a DELETE request with retry logic using a Privy identity token.
func (rc *RetryableClient) DeleteWithIdentity(ctx context.Context, path string, identityToken string, result any) error {
	_, err := WithRetry(ctx, func() (struct{}, error) {
		err := rc.client.DeleteWithIdentity(ctx, path, identityToken, result)
		return struct{}{}, err
	}, rc.config, rc.logger)
	return err
}
