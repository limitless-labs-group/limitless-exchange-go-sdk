package limitless

import (
	"fmt"
	"regexp"
	"strings"
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
	switch a := args.(type) {
	case FOKOrderArgs:
		if a.TokenID == "" {
			return &OrderValidationError{Field: "tokenId", Message: "tokenId is required"}
		}
		if a.TokenID == "0" {
			return &OrderValidationError{Field: "tokenId", Message: "tokenId cannot be zero"}
		}
		if !numericRegex.MatchString(a.TokenID) {
			return &OrderValidationError{Field: "tokenId", Message: fmt.Sprintf("invalid tokenId format: %s", a.TokenID)}
		}
		if a.MakerAmount <= 0 {
			return &OrderValidationError{Field: "makerAmount", Message: fmt.Sprintf("amount must be positive, got: %v", a.MakerAmount)}
		}
		// Validate max 2 decimal places
		amountStr := fmt.Sprintf("%v", a.MakerAmount)
		if idx := strings.Index(amountStr, "."); idx != -1 {
			decPlaces := len(amountStr) - idx - 1
			if decPlaces > 2 {
				return &OrderValidationError{
					Field:   "makerAmount",
					Message: fmt.Sprintf("amount must have max 2 decimal places, got: %v (%d decimals)", a.MakerAmount, decPlaces),
				}
			}
		}
		if a.Taker != "" && !isValidAddress(a.Taker) {
			return &OrderValidationError{Field: "taker", Message: fmt.Sprintf("invalid taker address: %s", a.Taker)}
		}
		if a.Expiration != "" && !numericRegex.MatchString(a.Expiration) {
			return &OrderValidationError{Field: "expiration", Message: fmt.Sprintf("invalid expiration format: %s", a.Expiration)}
		}
		if a.Nonce != nil && *a.Nonce < 0 {
			return &OrderValidationError{Field: "nonce", Message: fmt.Sprintf("invalid nonce: %d", *a.Nonce)}
		}

	case GTCOrderArgs:
		if a.TokenID == "" {
			return &OrderValidationError{Field: "tokenId", Message: "tokenId is required"}
		}
		if a.TokenID == "0" {
			return &OrderValidationError{Field: "tokenId", Message: "tokenId cannot be zero"}
		}
		if !numericRegex.MatchString(a.TokenID) {
			return &OrderValidationError{Field: "tokenId", Message: fmt.Sprintf("invalid tokenId format: %s", a.TokenID)}
		}
		if a.Price < 0 || a.Price > 1 {
			return &OrderValidationError{Field: "price", Message: fmt.Sprintf("price must be between 0 and 1, got: %v", a.Price)}
		}
		if a.Size <= 0 {
			return &OrderValidationError{Field: "size", Message: fmt.Sprintf("size must be positive, got: %v", a.Size)}
		}
		if a.Taker != "" && !isValidAddress(a.Taker) {
			return &OrderValidationError{Field: "taker", Message: fmt.Sprintf("invalid taker address: %s", a.Taker)}
		}
		if a.Expiration != "" && !numericRegex.MatchString(a.Expiration) {
			return &OrderValidationError{Field: "expiration", Message: fmt.Sprintf("invalid expiration format: %s", a.Expiration)}
		}
		if a.Nonce != nil && *a.Nonce < 0 {
			return &OrderValidationError{Field: "nonce", Message: fmt.Sprintf("invalid nonce: %d", *a.Nonce)}
		}
	}

	return nil
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
