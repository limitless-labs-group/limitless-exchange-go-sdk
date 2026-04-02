package limitless

import (
	"strings"
	"testing"
)

func ptrInt(v int) *int { return &v }

func validUnsignedOrder() *UnsignedOrder {
	price := 0.381
	return &UnsignedOrder{
		Salt:          1742191300000001,
		Maker:         "0xa00BCB04073B243E8A55f3B5899AefF596bF17C6",
		Signer:        "0xa00BCB04073B243E8A55f3B5899AefF596bF17C6",
		Taker:         ZeroAddress,
		TokenID:       "12345",
		MakerAmount:   470154,
		TakerAmount:   1234000,
		Expiration:    "0",
		Nonce:         0,
		FeeRateBps:    300,
		Side:          SideBuy,
		SignatureType: SignatureTypeEOA,
		Price:         &price,
	}
}

func validSignedOrder() *SignedOrder {
	u := validUnsignedOrder()
	return &SignedOrder{
		Salt:          u.Salt,
		Maker:         u.Maker,
		Signer:        u.Signer,
		Taker:         u.Taker,
		TokenID:       u.TokenID,
		MakerAmount:   u.MakerAmount,
		TakerAmount:   u.TakerAmount,
		Expiration:    u.Expiration,
		Nonce:         u.Nonce,
		FeeRateBps:    u.FeeRateBps,
		Side:          u.Side,
		SignatureType: u.SignatureType,
		Price:         u.Price,
		Signature:     "0x" + strings.Repeat("a", 130),
	}
}

func TestValidateOrderArgs_FOK_ValidationRules(t *testing.T) {
	t.Parallel()

	valid := FOKOrderArgs{
		TokenID:     "12345",
		Side:        SideBuy,
		MakerAmount: 10.12,
		Expiration:  "0",
		Nonce:       ptrInt(1),
		Taker:       ZeroAddress,
	}
	if err := ValidateOrderArgs(valid); err != nil {
		t.Fatalf("expected valid FOK args, got error: %v", err)
	}

	cases := []struct {
		name  string
		args  FOKOrderArgs
		field string
	}{
		{name: "missing tokenId", args: FOKOrderArgs{MakerAmount: 1}, field: "tokenId"},
		{name: "zero tokenId", args: FOKOrderArgs{TokenID: "0", MakerAmount: 1}, field: "tokenId"},
		{name: "nonnumeric tokenId", args: FOKOrderArgs{TokenID: "abc", MakerAmount: 1}, field: "tokenId"},
		{name: "nonpositive makerAmount", args: FOKOrderArgs{TokenID: "1", MakerAmount: 0}, field: "makerAmount"},
		{name: "too many decimals", args: FOKOrderArgs{TokenID: "1", MakerAmount: 1.1234567}, field: "makerAmount"},
		{name: "invalid taker", args: FOKOrderArgs{TokenID: "1", MakerAmount: 1, Taker: "bad"}, field: "taker"},
		{name: "invalid expiration", args: FOKOrderArgs{TokenID: "1", MakerAmount: 1, Expiration: "abc"}, field: "expiration"},
		{name: "invalid nonce", args: FOKOrderArgs{TokenID: "1", MakerAmount: 1, Nonce: ptrInt(-1)}, field: "nonce"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateOrderArgs(tc.args)
			if err == nil {
				t.Fatalf("expected validation error for %s, got nil", tc.name)
			}
			ve, ok := err.(*OrderValidationError)
			if !ok {
				t.Fatalf("expected OrderValidationError, got %T (%v)", err, err)
			}
			if ve.Field != tc.field {
				t.Fatalf("expected field %s, got %s", tc.field, ve.Field)
			}
		})
	}
}

func TestValidateOrderArgs_GTC_ValidationRules(t *testing.T) {
	t.Parallel()

	valid := GTCOrderArgs{
		TokenID:    "12345",
		Side:       SideSell,
		Price:      0.5,
		Size:       10,
		Expiration: "0",
		Nonce:      ptrInt(0),
		Taker:      ZeroAddress,
	}
	if err := ValidateOrderArgs(valid); err != nil {
		t.Fatalf("expected valid GTC args, got error: %v", err)
	}

	cases := []struct {
		name  string
		args  GTCOrderArgs
		field string
	}{
		{name: "missing tokenId", args: GTCOrderArgs{Price: 0.5, Size: 1}, field: "tokenId"},
		{name: "bad tokenId", args: GTCOrderArgs{TokenID: "abc", Price: 0.5, Size: 1}, field: "tokenId"},
		{name: "invalid price", args: GTCOrderArgs{TokenID: "1", Price: 1.5, Size: 1}, field: "price"},
		{name: "non tick aligned price", args: GTCOrderArgs{TokenID: "1", Price: 0.3814, Size: 1}, field: "price"},
		{name: "invalid size", args: GTCOrderArgs{TokenID: "1", Price: 0.5, Size: 0}, field: "size"},
		{name: "invalid size step", args: GTCOrderArgs{TokenID: "1", Price: 0.5, Size: 0.0001}, field: "size"},
		{name: "invalid taker", args: GTCOrderArgs{TokenID: "1", Price: 0.5, Size: 1, Taker: "bad"}, field: "taker"},
		{name: "invalid expiration", args: GTCOrderArgs{TokenID: "1", Price: 0.5, Size: 1, Expiration: "bad"}, field: "expiration"},
		{name: "invalid nonce", args: GTCOrderArgs{TokenID: "1", Price: 0.5, Size: 1, Nonce: ptrInt(-2)}, field: "nonce"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateOrderArgs(tc.args)
			if err == nil {
				t.Fatalf("expected validation error for %s, got nil", tc.name)
			}
			ve, ok := err.(*OrderValidationError)
			if !ok {
				t.Fatalf("expected OrderValidationError, got %T (%v)", err, err)
			}
			if ve.Field != tc.field {
				t.Fatalf("expected field %s, got %s", tc.field, ve.Field)
			}
		})
	}
}

func TestValidateUnsignedOrder_ValidationRules(t *testing.T) {
	t.Parallel()

	valid := validUnsignedOrder()
	if err := ValidateUnsignedOrder(valid); err != nil {
		t.Fatalf("expected valid unsigned order, got error: %v", err)
	}

	makerBad := *valid
	makerBad.Maker = "bad"
	if err := ValidateUnsignedOrder(&makerBad); err == nil {
		t.Fatal("expected invalid maker error, got nil")
	}

	tokenBad := *valid
	tokenBad.TokenID = "abc"
	if err := ValidateUnsignedOrder(&tokenBad); err == nil {
		t.Fatal("expected invalid tokenId error, got nil")
	}

	sideBad := *valid
	sideBad.Side = Side(3)
	if err := ValidateUnsignedOrder(&sideBad); err == nil {
		t.Fatal("expected invalid side error, got nil")
	}

	priceBad := *valid
	price := 1.2
	priceBad.Price = &price
	if err := ValidateUnsignedOrder(&priceBad); err == nil {
		t.Fatal("expected invalid price error, got nil")
	}
}

func TestValidateSignedOrder_ValidationRules(t *testing.T) {
	t.Parallel()

	valid := validSignedOrder()
	if err := ValidateSignedOrder(valid); err != nil {
		t.Fatalf("expected valid signed order, got error: %v", err)
	}

	missing := *valid
	missing.Signature = ""
	if err := ValidateSignedOrder(&missing); err == nil {
		t.Fatal("expected missing signature error, got nil")
	}

	short := *valid
	short.Signature = "0x1234"
	if err := ValidateSignedOrder(&short); err == nil {
		t.Fatal("expected short signature error, got nil")
	}

	notHex := *valid
	notHex.Signature = "0x" + strings.Repeat("z", 130)
	if err := ValidateSignedOrder(&notHex); err == nil {
		t.Fatal("expected non-hex signature error, got nil")
	}
}
