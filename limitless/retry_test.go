package limitless

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWithRetry_RetriesAndEventuallySucceeds(t *testing.T) {
	t.Parallel()

	attempts := 0
	cfg := RetryConfig{
		StatusCodes: []int{429},
		MaxRetries:  3,
		Delays:      []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond},
	}

	result, err := WithRetry(context.Background(), func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", &RateLimitError{
				APIError: APIError{Status: 429, Message: "slow down"},
			}
		}
		return "ok", nil
	}, cfg)
	if err != nil {
		t.Fatalf("WithRetry returned error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected result ok, got %q", result)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestWithRetry_DoesNotRetryNonRetryableError(t *testing.T) {
	t.Parallel()

	attempts := 0
	cfg := RetryConfig{
		StatusCodes: []int{429},
		MaxRetries:  3,
		Delays:      []time.Duration{time.Millisecond},
	}

	_, err := WithRetry(context.Background(), func() (struct{}, error) {
		attempts++
		return struct{}{}, errors.New("boom")
	}, cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 1 {
		t.Fatalf("expected exactly 1 attempt for non-retryable errors, got %d", attempts)
	}
}

func TestWithRetry_RespectsContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	cfg := RetryConfig{
		StatusCodes: []int{429},
		MaxRetries:  3,
		Delays:      []time.Duration{100 * time.Millisecond},
	}

	_, err := WithRetry(ctx, func() (struct{}, error) {
		return struct{}{}, &RateLimitError{
			APIError: APIError{Status: 429, Message: "still limited"},
		}
	}, cfg)
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation error, got %T (%v)", err, err)
	}
}
