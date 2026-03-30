package limitless

import (
	"net/http"
	"time"
)

// ClientOption configures an HttpClient.
type ClientOption func(*HttpClient)

// WithBaseURL sets the base URL for API requests.
// Default: "https://api.limitless.exchange"
func WithBaseURL(url string) ClientOption {
	return func(c *HttpClient) {
		c.baseURL = url
	}
}

// WithTimeout sets the HTTP request timeout.
// Default: 30s
func WithTimeout(d time.Duration) ClientOption {
	return func(c *HttpClient) {
		c.httpClient.Timeout = d
	}
}

// WithAPIKey sets the API key for authenticated requests.
func WithAPIKey(key string) ClientOption {
	return func(c *HttpClient) {
		c.apiKey = key
	}
}

// WithHMACCredentials sets HMAC credentials for authenticated requests.
func WithHMACCredentials(creds HMACCredentials) ClientOption {
	return func(c *HttpClient) {
		c.hmacCreds = &HMACCredentials{
			TokenID: creds.TokenID,
			Secret:  creds.Secret,
		}
	}
}

// WithLogger sets the logger for the HTTP client.
func WithLogger(l Logger) ClientOption {
	return func(c *HttpClient) {
		c.logger = l
	}
}

// WithTransport sets a custom HTTP transport.
func WithTransport(t http.RoundTripper) ClientOption {
	return func(c *HttpClient) {
		c.httpClient.Transport = t
	}
}

// WithMaxIdleConns sets the maximum number of idle connections.
// Default: 50
func WithMaxIdleConns(n int) ClientOption {
	return func(c *HttpClient) {
		if t, ok := c.httpClient.Transport.(*http.Transport); ok {
			t.MaxIdleConns = n
			t.MaxIdleConnsPerHost = n
		}
	}
}

// WithIdleConnTimeout sets the idle connection timeout.
// Default: 60s
func WithIdleConnTimeout(d time.Duration) ClientOption {
	return func(c *HttpClient) {
		if t, ok := c.httpClient.Transport.(*http.Transport); ok {
			t.IdleConnTimeout = d
		}
	}
}

// WithAdditionalHeaders sets additional headers to include in all requests.
func WithAdditionalHeaders(h map[string]string) ClientOption {
	return func(c *HttpClient) {
		for k, v := range h {
			c.headers[k] = v
		}
	}
}
