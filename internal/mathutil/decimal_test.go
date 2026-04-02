package mathutil

import (
	"math"
	"math/big"
	"testing"
)

func TestScaleTo6Decimals_NormalValues(t *testing.T) {
	tests := []struct {
		input    float64
		expected int64
	}{
		{1.0, 1_000_000},
		{0.5, 500_000},
		{12.345678, 12_345_678},
		{0.000001, 1},
		{100.0, 100_000_000},
		{0.0, 0},
	}

	for _, tt := range tests {
		result, err := ScaleTo6Decimals(tt.input)
		if err != nil {
			t.Errorf("ScaleTo6Decimals(%v) returned error: %v", tt.input, err)
			continue
		}
		if result != tt.expected {
			t.Errorf("ScaleTo6Decimals(%v) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestScaleTo6Decimals_OverflowReturnsError(t *testing.T) {
	// 1e13 * 1e6 = 1e19, which exceeds MaxInt64 (9.22e18)
	_, err := ScaleTo6Decimals(10_000_000_000_000)
	if err == nil {
		t.Fatal("expected overflow error for 1e13, got nil")
	}

	// Negative overflow
	_, err = ScaleTo6Decimals(-10_000_000_000_000)
	if err == nil {
		t.Fatal("expected overflow error for -1e13, got nil")
	}
}

func TestScaleTo6Decimals_MaxSafeValueSucceeds(t *testing.T) {
	// MaxInt64 / 1e6 ≈ 9.22e12 — use a value safely below that
	result, err := ScaleTo6Decimals(9_000_000_000_000)
	if err != nil {
		t.Fatalf("expected success for 9e12, got error: %v", err)
	}
	if result != 9_000_000_000_000_000_000 {
		t.Fatalf("expected 9e18, got %d", result)
	}
}

func TestParsDecToInt_LargeValues(t *testing.T) {
	// Verify big.Int handles values beyond int64 range
	result := ParseDecToInt("99999999999999999999", Scale6)
	maxInt := new(big.Int).SetInt64(math.MaxInt64)
	if result.Cmp(maxInt) <= 0 {
		t.Fatalf("expected result > MaxInt64, got %s", result.String())
	}
	if result.IsInt64() {
		t.Fatalf("expected IsInt64() to be false for value %s", result.String())
	}
}
