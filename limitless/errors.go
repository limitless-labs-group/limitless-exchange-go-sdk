package limitless

import (
	"encoding/json"
	"fmt"
	"strings"
)

// APIError represents an error returned by the Limitless Exchange API.
type APIError struct {
	// Status is the HTTP status code.
	Status int
	// Message is a human-readable error message.
	Message string
	// Data is the raw API response body.
	Data json.RawMessage
	// URL is the request URL that failed.
	URL string
	// Method is the HTTP method (GET, POST, etc.).
	Method string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d %s %s: %s", e.Status, e.Method, e.URL, e.Message)
}

// IsAuthError returns true if the error is an authentication/authorization error.
func (e *APIError) IsAuthError() bool {
	return e.Status == 401 || e.Status == 403
}

// RateLimitError represents an HTTP 429 rate limit error.
type RateLimitError struct {
	APIError
}

// AuthenticationError represents an HTTP 401/403 authentication error.
type AuthenticationError struct {
	APIError
}

// ValidationError represents an HTTP 400 validation error.
type ValidationError struct {
	APIError
}

// ConflictError represents an HTTP 409 conflict error.
type ConflictError struct {
	APIError
}

// UnprocessableEntityError represents an HTTP 422 semantic validation error,
// such as an insufficient balance or invalid AMM quote.
type UnprocessableEntityError struct {
	APIError
}

// TooEarlyError represents an HTTP 425 response. AMM endpoints use this status
// while trading is disabled by maintenance mode.
type TooEarlyError struct {
	APIError
}

// UpstreamUnavailableError represents an HTTP 502/503 failure in an upstream
// dependency such as RPC, quote, submission, Redis, or market state lookup.
type UpstreamUnavailableError struct {
	APIError
}

// parseAPIError creates the appropriate typed error from an HTTP response.
func parseAPIError(status int, body []byte, url, method string) error {
	msg := extractErrorMessage(body, fmt.Sprintf("Request failed with status %d", status))

	base := APIError{
		Status:  status,
		Message: msg,
		Data:    json.RawMessage(body),
		URL:     url,
		Method:  method,
	}

	switch {
	case status == 429:
		return &RateLimitError{APIError: base}
	case status == 401 || status == 403:
		return &AuthenticationError{APIError: base}
	case status == 400:
		return &ValidationError{APIError: base}
	case status == 409:
		return &ConflictError{APIError: base}
	case status == 422:
		return &UnprocessableEntityError{APIError: base}
	case status == 425:
		return &TooEarlyError{APIError: base}
	case status == 502 || status == 503:
		return &UpstreamUnavailableError{APIError: base}
	default:
		return &base
	}
}

// extractErrorMessage tries to extract a human-readable message from the API response body.
func extractErrorMessage(body []byte, fallback string) string {
	if len(body) == 0 {
		return fallback
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return string(body)
	}

	// Check if message is an array (validation errors)
	if msgArr, ok := data["message"].([]any); ok {
		var messages []string
		for _, item := range msgArr {
			if m, ok := item.(map[string]any); ok {
				var parts []string
				for k, v := range m {
					if v != nil && v != "" {
						parts = append(parts, fmt.Sprintf("%s: %v", k, v))
					}
				}
				if len(parts) > 0 {
					messages = append(messages, strings.Join(parts, ", "))
				}
			}
		}
		if len(messages) > 0 {
			result := ""
			for i, m := range messages {
				if i > 0 {
					result += " | "
				}
				result += m
			}
			return result
		}
	}

	// Try common error field names
	for _, key := range []string{"message", "error", "msg"} {
		if v, ok := data[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}

	if errors, ok := data["errors"]; ok {
		b, _ := json.Marshal(errors)
		return string(b)
	}

	return string(body)
}
