package mathutil

import (
	"fmt"
	"math/big"
	"strings"
)

// Scale6 is the scaling factor for 6 decimal places (1,000,000).
var Scale6 = big.NewInt(1_000_000)

// ParseDecToInt parses a decimal string and scales it by the given factor.
// For example, ParseDecToInt("0.38", 1_000_000) returns 380000.
func ParseDecToInt(value string, scale *big.Int) *big.Int {
	s := strings.TrimSpace(value)
	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]
	fracPart := ""
	if len(parts) > 1 {
		fracPart = parts[1]
	}

	decimals := len(scale.String()) - 1

	// Pad or trim fractional part
	if len(fracPart) < decimals {
		fracPart += strings.Repeat("0", decimals-len(fracPart))
	} else {
		fracPart = fracPart[:decimals]
	}

	negative := false
	if strings.HasPrefix(intPart, "-") {
		negative = true
		intPart = intPart[1:]
	}
	if intPart == "" {
		intPart = "0"
	}

	intVal, ok := new(big.Int).SetString(intPart, 10)
	if !ok {
		return big.NewInt(0)
	}

	fracVal, ok := new(big.Int).SetString(fracPart, 10)
	if !ok {
		fracVal = big.NewInt(0)
	}

	result := new(big.Int).Mul(intVal, scale)
	result.Add(result, fracVal)

	if negative {
		result.Neg(result)
	}

	return result
}

// DivCeil performs ceiling division: (a + b - 1) / b
func DivCeil(a, b *big.Int) (*big.Int, error) {
	if b.Sign() == 0 {
		return nil, fmt.Errorf("division by zero")
	}
	result := new(big.Int)
	result.Add(a, new(big.Int).Sub(b, big.NewInt(1)))
	result.Div(result, b)
	return result, nil
}

// ScaleTo6Decimals converts a float64 amount to an int64 scaled to 6 decimal places.
func ScaleTo6Decimals(amount float64) int64 {
	s := fmt.Sprintf("%.6f", amount)
	result := ParseDecToInt(s, Scale6)
	return result.Int64()
}

// IsTickAligned checks if a value is a multiple of the given tick size.
// Both value and tick are parsed at the given scale.
func IsTickAligned(value, tick float64, scale *big.Int) bool {
	v := ParseDecToInt(fmt.Sprintf("%v", value), scale)
	t := ParseDecToInt(fmt.Sprintf("%v", tick), scale)
	if t.Sign() == 0 {
		return false
	}
	mod := new(big.Int).Mod(v, t)
	return mod.Sign() == 0
}
