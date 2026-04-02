package limitless

import (
	"strings"
	"testing"
	"time"
)

func TestOrderBuilder_GenerateSalt_IsFutureBiasedAndPositive(t *testing.T) {
	t.Parallel()

	builder := NewOrderBuilder("0xa00BCB04073B243E8A55f3B5899AefF596bF17C6", 300)
	minExpected := time.Now().UnixMilli()*1000 + 86400000

	saltA := builder.generateSalt()
	saltB := builder.generateSalt()

	if saltA < minExpected {
		t.Fatalf("expected salt >= %d, got %d", minExpected, saltA)
	}
	if saltA <= 0 || saltB <= 0 {
		t.Fatalf("salt must be positive, got %d and %d", saltA, saltB)
	}
	if saltA > saltB {
		t.Fatalf("expected non-decreasing salt sequence, got saltA=%d saltB=%d", saltA, saltB)
	}
}

func TestOrderBuilder_BuildOrder_FOK_DefaultsAndScaling(t *testing.T) {
	t.Parallel()

	builder := NewOrderBuilder("0xa00BCB04073B243E8A55f3B5899AefF596bF17C6", 300)
	order, err := builder.BuildOrder(FOKOrderArgs{
		TokenID:     "12345",
		Side:        SideBuy,
		MakerAmount: 12.345678,
	})
	if err != nil {
		t.Fatalf("BuildOrder(FOK) returned error: %v", err)
	}

	if order.MakerAmount != 12345678 {
		t.Fatalf("expected maker amount 12345678, got %d", order.MakerAmount)
	}
	if order.TakerAmount != 1 {
		t.Fatalf("expected taker amount 1, got %d", order.TakerAmount)
	}
	if order.Taker != ZeroAddress {
		t.Fatalf("expected default taker %s, got %s", ZeroAddress, order.Taker)
	}
	if order.Expiration != "0" {
		t.Fatalf("expected default expiration 0, got %s", order.Expiration)
	}
	if order.Nonce != 0 {
		t.Fatalf("expected default nonce 0, got %d", order.Nonce)
	}
	if order.Price != nil {
		t.Fatalf("expected nil price for FOK, got %v", *order.Price)
	}
}

func TestOrderBuilder_BuildOrder_FOK_RejectsTooManyDecimals(t *testing.T) {
	t.Parallel()

	builder := NewOrderBuilder("0xa00BCB04073B243E8A55f3B5899AefF596bF17C6", 300)
	_, err := builder.BuildOrder(FOKOrderArgs{
		TokenID:     "12345",
		Side:        SideBuy,
		MakerAmount: 1.1234567,
	})
	if err == nil {
		t.Fatal("expected error for makerAmount with > 6 decimals, got nil")
	}
	if !strings.Contains(err.Error(), "max 6 decimal places") {
		t.Fatalf("expected decimal places validation error, got: %v", err)
	}
}

func TestOrderBuilder_BuildOrder_GTC_BuyAndSellAmounts(t *testing.T) {
	t.Parallel()

	builder := NewOrderBuilder("0xa00BCB04073B243E8A55f3B5899AefF596bF17C6", 300)

	buyOrder, err := builder.BuildOrder(GTCOrderArgs{
		TokenID: "12345",
		Side:    SideBuy,
		Price:   0.381,
		Size:    1.234,
	})
	if err != nil {
		t.Fatalf("BuildOrder(GTC buy) returned error: %v", err)
	}

	if buyOrder.MakerAmount != 470154 {
		t.Fatalf("expected buy maker amount 470154, got %d", buyOrder.MakerAmount)
	}
	if buyOrder.TakerAmount != 1234000 {
		t.Fatalf("expected buy taker amount 1234000, got %d", buyOrder.TakerAmount)
	}
	if buyOrder.Price == nil || *buyOrder.Price != 0.381 {
		t.Fatalf("expected buy price 0.381, got %+v", buyOrder.Price)
	}

	sellOrder, err := builder.BuildOrder(GTCOrderArgs{
		TokenID: "12345",
		Side:    SideSell,
		Price:   0.381,
		Size:    1.234,
	})
	if err != nil {
		t.Fatalf("BuildOrder(GTC sell) returned error: %v", err)
	}

	if sellOrder.MakerAmount != 1234000 {
		t.Fatalf("expected sell maker amount 1234000, got %d", sellOrder.MakerAmount)
	}
	if sellOrder.TakerAmount != 470154 {
		t.Fatalf("expected sell taker amount 470154, got %d", sellOrder.TakerAmount)
	}
}

func TestOrderBuilder_BuildOrder_GTC_RejectsNonTickAlignedPrice(t *testing.T) {
	t.Parallel()

	builder := NewOrderBuilder("0xa00BCB04073B243E8A55f3B5899AefF596bF17C6", 300, 0.002)
	_, err := builder.BuildOrder(GTCOrderArgs{
		TokenID: "12345",
		Side:    SideBuy,
		Price:   0.381, // max 3 decimals but not aligned to 0.002
		Size:    1.0,
	})
	if err == nil {
		t.Fatal("expected tick alignment error, got nil")
	}
	if !strings.Contains(err.Error(), "not tick-aligned") {
		t.Fatalf("expected not tick-aligned error, got: %v", err)
	}
}

func TestOrderBuilder_BuildOrder_GTC_RejectsInvalidSizeStep(t *testing.T) {
	t.Parallel()

	builder := NewOrderBuilder("0xa00BCB04073B243E8A55f3B5899AefF596bF17C6", 300, 0.005)
	_, err := builder.BuildOrder(GTCOrderArgs{
		TokenID: "12345",
		Side:    SideBuy,
		Price:   0.5,
		Size:    0.0001, // 100 scaled shares; step is 200 for tick=0.005
	})
	if err == nil {
		t.Fatal("expected size step validation error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid size") {
		t.Fatalf("expected invalid size error, got: %v", err)
	}
}

func TestOrderBuilder_BuildOrder_GTC_RejectsOverflowShares(t *testing.T) {
	t.Parallel()

	builder := NewOrderBuilder("0xa00BCB04073B243E8A55f3B5899AefF596bF17C6", 300)
	// Size of 1e13 at price 1.0 → shares = 1e13 * 1e6 = 1e19, exceeds int64 max (9.22e18)
	_, err := builder.BuildOrder(GTCOrderArgs{
		TokenID: "12345",
		Side:    SideBuy,
		Price:   1.0,
		Size:    10000000000000,
	})
	if err == nil {
		t.Fatal("expected overflow error for extremely large size, got nil")
	}
	if !strings.Contains(err.Error(), "overflow") {
		t.Fatalf("expected overflow error, got: %v", err)
	}
}

func TestOrderBuilder_BuildOrder_FOK_RejectsOverflowMakerAmount(t *testing.T) {
	t.Parallel()

	builder := NewOrderBuilder("0xa00BCB04073B243E8A55f3B5899AefF596bF17C6", 300)
	// 1e13 * 1e6 = 1e19, exceeds int64 max
	_, err := builder.BuildOrder(FOKOrderArgs{
		TokenID:     "12345",
		Side:        SideBuy,
		MakerAmount: 10000000000000,
	})
	if err == nil {
		t.Fatal("expected overflow error for extremely large FOK makerAmount, got nil")
	}
	if !strings.Contains(err.Error(), "overflow") {
		t.Fatalf("expected overflow error, got: %v", err)
	}
}
