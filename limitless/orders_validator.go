package limitless

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/limitless-labs-group/limitless-exchange-go-sdk/internal/mathutil"
)

// OrderValidationError represents a client-side order validation error.
type OrderValidationError struct {
	Field   string
	Message string
}

func (e *OrderValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("order validation error [%s]: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("order validation error: %s", e.Message)
}

var numericRegex = regexp.MustCompile(`^\d+$`)

// ValidateOrderArgs validates order arguments before building.
func ValidateOrderArgs(args OrderArgs) error {
	return validateOrderArgs(args, 0.001)
}

func validateOrderArgs(args OrderArgs, priceTick float64) error {
	switch a := args.(type) {
	case FOKOrderArgs:
		if err := validateTokenID(a.TokenID); err != nil {
			return err
		}
		if a.MakerAmount <= 0 {
			return &OrderValidationError{Field: "makerAmount", Message: fmt.Sprintf("amount must be positive, got: %v", a.MakerAmount)}
		}
		if decimals := decimalPlaces(a.MakerAmount); decimals > 6 {
			return &OrderValidationError{
				Field:   "makerAmount",
				Message: fmt.Sprintf("amount must have max 6 decimal places, got: %v (%d decimals)", a.MakerAmount, decimals),
			}
		}
		if err := validateOptionalOrderFields(a.Taker, a.Expiration, a.Nonce); err != nil {
			return err
		}

	case GTCOrderArgs:
		if err := validateTokenID(a.TokenID); err != nil {
			return err
		}
		if a.Price <= 0 || a.Price > 1 {
			return &OrderValidationError{Field: "price", Message: fmt.Sprintf("price must be between 0 and 1, got: %v", a.Price)}
		}
		if a.Size <= 0 {
			return &OrderValidationError{Field: "size", Message: fmt.Sprintf("size must be positive, got: %v", a.Size)}
		}

		maxPriceDecimals := decimalPlaces(priceTick)
		if decimals := decimalPlaces(a.Price); decimals > maxPriceDecimals {
			return &OrderValidationError{
				Field:   "price",
				Message: fmt.Sprintf("price must have max %d decimal places, got: %v (%d decimals)", maxPriceDecimals, a.Price, decimals),
			}
		}
		if decimals := decimalPlaces(a.Size); decimals > 6 {
			return &OrderValidationError{
				Field:   "size",
				Message: fmt.Sprintf("size must have max 6 decimal places, got: %v (%d decimals)", a.Size, decimals),
			}
		}

		scale := mathutil.Scale6
		priceInt := mathutil.ParseDecToInt(strconv.FormatFloat(a.Price, 'f', -1, 64), scale)
		tickInt := mathutil.ParseDecToInt(strconv.FormatFloat(priceTick, 'f', -1, 64), scale)
		if tickInt.Sign() <= 0 {
			return &OrderValidationError{Field: "price", Message: fmt.Sprintf("invalid priceTick: %v", priceTick)}
		}
		if new(big.Int).Mod(priceInt, tickInt).Sign() != 0 {
			return &OrderValidationError{
				Field:   "price",
				Message: fmt.Sprintf("price %v is not tick-aligned. Must be multiple of %v (e.g., 0.380, 0.381, etc.)", a.Price, priceTick),
			}
		}

		shares := mathutil.ParseDecToInt(strconv.FormatFloat(a.Size, 'f', -1, 64), scale)
		sharesStep := new(big.Int).Div(scale, tickInt)
		if new(big.Int).Mod(shares, sharesStep).Sign() != 0 {
			validDown := new(big.Int).Div(new(big.Int).Set(shares), sharesStep)
			validDown.Mul(validDown, sharesStep)
			validUp, err := mathutil.DivCeil(shares, sharesStep)
			if err != nil {
				return &OrderValidationError{Field: "size", Message: fmt.Sprintf("failed to calculate valid size: %v", err)}
			}
			validUp.Mul(validUp, sharesStep)

			return &OrderValidationError{
				Field: "size",
				Message: fmt.Sprintf(
					"invalid size: %v. Size must produce contracts divisible by %s (sharesStep). Try %s (rounded down) or %s (rounded up) instead",
					a.Size, sharesStep.String(), formatScaledBigInt(validDown, 6), formatScaledBigInt(validUp, 6),
				),
			}
		}

		if err := validateOptionalOrderFields(a.Taker, a.Expiration, a.Nonce); err != nil {
			return err
		}
	}

	return nil
}

func validateTokenID(tokenID string) error {
	if tokenID == "" {
		return &OrderValidationError{Field: "tokenId", Message: "tokenId is required"}
	}
	if tokenID == "0" {
		return &OrderValidationError{Field: "tokenId", Message: "tokenId cannot be zero"}
	}
	if !numericRegex.MatchString(tokenID) {
		return &OrderValidationError{Field: "tokenId", Message: fmt.Sprintf("invalid tokenId format: %s", tokenID)}
	}
	return nil
}

func validateOptionalOrderFields(taker, expiration string, nonce *int) error {
	if taker != "" && !isValidAddress(taker) {
		return &OrderValidationError{Field: "taker", Message: fmt.Sprintf("invalid taker address: %s", taker)}
	}
	if expiration != "" && !numericRegex.MatchString(expiration) {
		return &OrderValidationError{Field: "expiration", Message: fmt.Sprintf("invalid expiration format: %s", expiration)}
	}
	if nonce != nil && *nonce < 0 {
		return &OrderValidationError{Field: "nonce", Message: fmt.Sprintf("invalid nonce: %d", *nonce)}
	}
	return nil
}

func decimalPlaces(value float64) int {
	formatted := strconv.FormatFloat(value, 'f', -1, 64)
	if idx := strings.Index(formatted, "."); idx != -1 {
		return len(formatted) - idx - 1
	}
	return 0
}

func formatScaledBigInt(value *big.Int, decimals int) string {
	if value == nil {
		return "0"
	}

	abs := new(big.Int).Set(value)
	sign := ""
	if abs.Sign() < 0 {
		sign = "-"
		abs.Abs(abs)
	}

	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	intPart := new(big.Int).Div(new(big.Int).Set(abs), scale)
	fracPart := new(big.Int).Mod(new(big.Int).Set(abs), scale)
	if fracPart.Sign() == 0 {
		return sign + intPart.String()
	}

	frac := fracPart.String()
	if len(frac) < decimals {
		frac = strings.Repeat("0", decimals-len(frac)) + frac
	}
	frac = strings.TrimRight(frac, "0")
	if frac == "" {
		return sign + intPart.String()
	}

	return sign + intPart.String() + "." + frac
}

// ValidateUnsignedOrder validates an unsigned order.
func ValidateUnsignedOrder(order *UnsignedOrder) error {
	if !isValidAddress(order.Maker) {
		return &OrderValidationError{Field: "maker", Message: fmt.Sprintf("invalid maker address: %s", order.Maker)}
	}
	if !isValidAddress(order.Signer) {
		return &OrderValidationError{Field: "signer", Message: fmt.Sprintf("invalid signer address: %s", order.Signer)}
	}
	if !isValidAddress(order.Taker) {
		return &OrderValidationError{Field: "taker", Message: fmt.Sprintf("invalid taker address: %s", order.Taker)}
	}
	if order.MakerAmount <= 0 {
		return &OrderValidationError{Field: "makerAmount", Message: "makerAmount must be greater than zero"}
	}
	if order.TakerAmount <= 0 {
		return &OrderValidationError{Field: "takerAmount", Message: "takerAmount must be greater than zero"}
	}
	if !numericRegex.MatchString(order.TokenID) {
		return &OrderValidationError{Field: "tokenId", Message: fmt.Sprintf("invalid tokenId format: %s", order.TokenID)}
	}
	if !numericRegex.MatchString(order.Expiration) {
		return &OrderValidationError{Field: "expiration", Message: fmt.Sprintf("invalid expiration format: %s", order.Expiration)}
	}
	if order.Salt <= 0 {
		return &OrderValidationError{Field: "salt", Message: fmt.Sprintf("invalid salt: %d", order.Salt)}
	}
	if order.Nonce < 0 {
		return &OrderValidationError{Field: "nonce", Message: fmt.Sprintf("invalid nonce: %d", order.Nonce)}
	}
	if order.FeeRateBps < 0 {
		return &OrderValidationError{Field: "feeRateBps", Message: fmt.Sprintf("invalid feeRateBps: %d", order.FeeRateBps)}
	}
	if order.Side != SideBuy && order.Side != SideSell {
		return &OrderValidationError{Field: "side", Message: fmt.Sprintf("invalid side: %d. Must be 0 (BUY) or 1 (SELL)", order.Side)}
	}
	if order.SignatureType < 0 {
		return &OrderValidationError{Field: "signatureType", Message: fmt.Sprintf("invalid signatureType: %d", order.SignatureType)}
	}
	if order.Price != nil {
		p := *order.Price
		if p < 0 || p > 1 {
			return &OrderValidationError{Field: "price", Message: fmt.Sprintf("price must be between 0 and 1, got: %v", p)}
		}
	}

	return nil
}

var signatureRegex = regexp.MustCompile(`^0x[0-9a-fA-F]{130}$`)

// ValidateSignedOrder validates a signed order.
func ValidateSignedOrder(order *SignedOrder) error {
	// Validate unsigned fields
	unsigned := &UnsignedOrder{
		Salt:          order.Salt,
		Maker:         order.Maker,
		Signer:        order.Signer,
		Taker:         order.Taker,
		TokenID:       order.TokenID,
		MakerAmount:   order.MakerAmount,
		TakerAmount:   order.TakerAmount,
		Expiration:    order.Expiration,
		Nonce:         order.Nonce,
		FeeRateBps:    order.FeeRateBps,
		Side:          order.Side,
		SignatureType: order.SignatureType,
		Price:         order.Price,
	}
	if err := ValidateUnsignedOrder(unsigned); err != nil {
		return err
	}

	if order.Signature == "" {
		return &OrderValidationError{Field: "signature", Message: "signature is required"}
	}
	if len(order.Signature) != 132 {
		return &OrderValidationError{Field: "signature", Message: fmt.Sprintf("invalid signature length: %d. Expected 132 characters", len(order.Signature))}
	}
	if !signatureRegex.MatchString(order.Signature) {
		return &OrderValidationError{Field: "signature", Message: "signature must be valid hex string"}
	}

	return nil
}
