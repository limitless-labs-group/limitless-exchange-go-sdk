package limitless

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/limitless-labs-group/limitless-exchange-go-sdk/internal/mathutil"
)

// OrderBuilder constructs unsigned order payloads.
type OrderBuilder struct {
	makerAddress string
	feeRateBps   int
	priceTick    float64
}

// NewOrderBuilder creates a new order builder.
// An optional priceTick can be provided (default: 0.001).
func NewOrderBuilder(makerAddress string, feeRateBps int, priceTick ...float64) *OrderBuilder {
	tick := 0.001
	if len(priceTick) > 0 && priceTick[0] > 0 {
		tick = priceTick[0]
	}
	return &OrderBuilder{
		makerAddress: makerAddress,
		feeRateBps:   feeRateBps,
		priceTick:    tick,
	}
}

// BuildOrder constructs an unsigned order from the given arguments.
func (b *OrderBuilder) BuildOrder(args OrderArgs) (*UnsignedOrder, error) {
	if err := validateOrderArgs(args, b.priceTick); err != nil {
		return nil, err
	}

	var makerAmount, takerAmount int64
	var price *float64

	switch a := args.(type) {
	case FOKOrderArgs:
		ma, ta, err := b.calculateFOKAmounts(a.MakerAmount)
		if err != nil {
			return nil, err
		}
		makerAmount = ma
		takerAmount = ta

	case GTCOrderArgs:
		ma, ta, p, err := b.calculateGTCAmounts(a.Price, a.Size, a.Side)
		if err != nil {
			return nil, err
		}
		makerAmount = ma
		takerAmount = ta
		price = &p
	}

	taker := ZeroAddress
	expiration := "0"
	nonce := 0

	switch a := args.(type) {
	case FOKOrderArgs:
		if a.Taker != "" {
			taker = a.Taker
		}
		if a.Expiration != "" {
			expiration = a.Expiration
		}
		if a.Nonce != nil {
			nonce = *a.Nonce
		}
	case GTCOrderArgs:
		if a.Taker != "" {
			taker = a.Taker
		}
		if a.Expiration != "" {
			expiration = a.Expiration
		}
		if a.Nonce != nil {
			nonce = *a.Nonce
		}
	}

	order := &UnsignedOrder{
		Salt:          b.generateSalt(),
		Maker:         b.makerAddress,
		Signer:        b.makerAddress,
		Taker:         taker,
		TokenID:       tokenIDFromArgs(args),
		MakerAmount:   makerAmount,
		TakerAmount:   takerAmount,
		Expiration:    expiration,
		Nonce:         nonce,
		FeeRateBps:    b.feeRateBps,
		Side:          sideFromArgs(args),
		SignatureType: SignatureTypeEOA,
		Price:         price,
	}

	return order, nil
}

func (b *OrderBuilder) generateSalt() int64 {
	timestamp := time.Now().UnixMilli()
	nanoOffset := time.Now().UnixNano() / 1000 % 1000000
	oneDayMs := int64(86400000)
	return timestamp*1000 + nanoOffset + oneDayMs
}

func (b *OrderBuilder) calculateFOKAmounts(makerAmount float64) (int64, int64, error) {
	amountStr := strconv.FormatFloat(makerAmount, 'f', -1, 64)
	decIdx := strings.Index(amountStr, ".")
	if decIdx != -1 {
		decPlaces := len(amountStr) - decIdx - 1
		if decPlaces > 6 {
			return 0, 0, fmt.Errorf("invalid makerAmount: %v. Can have max 6 decimal places. Try %.6f instead", makerAmount, makerAmount)
		}
	}

	scaled, err := mathutil.ScaleTo6Decimals(makerAmount)
	if err != nil {
		return 0, 0, fmt.Errorf("makerAmount %v: %w", makerAmount, err)
	}
	return scaled, 1, nil
}

func (b *OrderBuilder) calculateGTCAmounts(price float64, size float64, side Side) (int64, int64, float64, error) {
	scale := mathutil.Scale6

	shares := mathutil.ParseDecToInt(strconv.FormatFloat(size, 'f', -1, 64), scale)
	priceInt := mathutil.ParseDecToInt(strconv.FormatFloat(price, 'f', -1, 64), scale)
	tickInt := mathutil.ParseDecToInt(strconv.FormatFloat(b.priceTick, 'f', -1, 64), scale)

	if tickInt.Sign() <= 0 {
		return 0, 0, 0, fmt.Errorf("invalid priceTick: %v", b.priceTick)
	}
	if priceInt.Sign() <= 0 {
		return 0, 0, 0, fmt.Errorf("invalid price: %v", price)
	}

	// Validate price is tick-aligned
	if new(big.Int).Mod(priceInt, tickInt).Sign() != 0 {
		return 0, 0, 0, fmt.Errorf("price %v is not tick-aligned. Must be multiple of %v (e.g., 0.380, 0.381, etc.)", price, b.priceTick)
	}

	// Calculate shares step
	sharesStep := new(big.Int).Div(scale, tickInt)

	// Validate size produces tick-aligned shares (NO AUTO-ROUNDING)
	if new(big.Int).Mod(shares, sharesStep).Sign() != 0 {
		validDown := new(big.Int).Div(shares, sharesStep)
		validDown.Mul(validDown, sharesStep)
		validUp, err := mathutil.DivCeil(shares, sharesStep)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to calculate valid size: %w", err)
		}
		validUp.Mul(validUp, sharesStep)

		return 0, 0, 0, fmt.Errorf(
			"invalid size: %v. Size must produce contracts divisible by %s (sharesStep). Try %s (rounded down) or %s (rounded up) instead",
			size, sharesStep.String(), formatScaledBigInt(validDown, 6), formatScaledBigInt(validUp, 6),
		)
	}

	// Calculate collateral: (shares * price * collateralScale) / (sharesScale * priceScale)
	collateralScale := new(big.Int).Set(scale)
	numerator := new(big.Int).Mul(shares, priceInt)
	numerator.Mul(numerator, collateralScale)
	denominator := new(big.Int).Mul(scale, scale)

	var collateral *big.Int
	if side == SideBuy {
		var err error
		collateral, err = mathutil.DivCeil(numerator, denominator)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to calculate collateral: %w", err)
		}
	} else {
		collateral = new(big.Int).Div(numerator, denominator)
	}

	if !collateral.IsInt64() {
		return 0, 0, 0, fmt.Errorf("collateral overflow: value %s exceeds int64 range", collateral.String())
	}
	if !shares.IsInt64() {
		return 0, 0, 0, fmt.Errorf("shares overflow: value %s exceeds int64 range", shares.String())
	}

	var makerAmount, takerAmount int64
	if side == SideBuy {
		makerAmount = collateral.Int64()
		takerAmount = shares.Int64()
	} else {
		makerAmount = shares.Int64()
		takerAmount = collateral.Int64()
	}

	return makerAmount, takerAmount, price, nil
}

func tokenIDFromArgs(args OrderArgs) string {
	switch a := args.(type) {
	case FOKOrderArgs:
		return a.TokenID
	case GTCOrderArgs:
		return a.TokenID
	}
	return ""
}

func sideFromArgs(args OrderArgs) Side {
	switch a := args.(type) {
	case FOKOrderArgs:
		return a.Side
	case GTCOrderArgs:
		return a.Side
	}
	return SideBuy
}

// postOnlyFromArgs returns a pointer to the PostOnly flag for GTC args when set,
// or nil so the field is omitted from the JSON payload.
func postOnlyFromArgs(args OrderArgs) *bool {
	if a, ok := args.(GTCOrderArgs); ok && a.PostOnly {
		v := true
		return &v
	}
	return nil
}

// isValidAddress does a basic Ethereum address format check.
func isValidAddress(addr string) bool {
	if len(addr) != 42 {
		return false
	}
	if !strings.HasPrefix(addr, "0x") {
		return false
	}
	for _, c := range addr[2:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
