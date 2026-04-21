package limitless

import (
	"encoding/json"
	"testing"
)

func TestCreatedOrder_UnmarshalJSON_AcceptsNumericStringsForOrderPayloadFields(t *testing.T) {
	payload := []byte(`{
		"id":"order-1",
		"createdAt":"2026-03-16T00:00:00Z",
		"makerAmount":"50000000",
		"takerAmount":"100000000",
		"expiration":"0",
		"signatureType":0,
		"salt":"1742000000000000",
		"maker":"0xmaker",
		"signer":"0xsigner",
		"taker":"0xtaker",
		"tokenId":"123",
		"side":0,
		"feeRateBps":300,
		"nonce":1,
		"signature":"0xsig",
		"orderType":"GTC",
		"price":"0.52",
		"marketId":42
	}`)

	var order CreatedOrder
	if err := json.Unmarshal(payload, &order); err != nil {
		t.Fatalf("expected unmarshal to succeed, got error: %v", err)
	}

	if order.MakerAmount != 50000000 {
		t.Fatalf("unexpected makerAmount: got %d", order.MakerAmount)
	}
	if order.TakerAmount != 100000000 {
		t.Fatalf("unexpected takerAmount: got %d", order.TakerAmount)
	}
	if order.Salt != 1742000000000000 {
		t.Fatalf("unexpected salt: got %d", order.Salt)
	}
	if order.Price == nil || *order.Price != 0.52 {
		t.Fatalf("unexpected price: got %v", order.Price)
	}
}

func TestCreatedOrder_UnmarshalJSON_AcceptsNumericValuesForOrderPayloadFields(t *testing.T) {
	payload := []byte(`{
		"id":"order-1",
		"createdAt":"2026-03-16T00:00:00Z",
		"makerAmount":50000000,
		"takerAmount":100000000,
		"expiration":"0",
		"signatureType":0,
		"salt":1742000000000000,
		"maker":"0xmaker",
		"signer":"0xsigner",
		"taker":"0xtaker",
		"tokenId":"123",
		"side":0,
		"feeRateBps":300,
		"nonce":1,
		"signature":"0xsig",
		"orderType":"GTC",
		"price":0.52,
		"marketId":42
	}`)

	var order CreatedOrder
	if err := json.Unmarshal(payload, &order); err != nil {
		t.Fatalf("expected unmarshal to succeed, got error: %v", err)
	}

	if order.MakerAmount != 50000000 {
		t.Fatalf("unexpected makerAmount: got %d", order.MakerAmount)
	}
	if order.TakerAmount != 100000000 {
		t.Fatalf("unexpected takerAmount: got %d", order.TakerAmount)
	}
	if order.Salt != 1742000000000000 {
		t.Fatalf("unexpected salt: got %d", order.Salt)
	}
	if order.Price == nil || *order.Price != 0.52 {
		t.Fatalf("unexpected price: got %v", order.Price)
	}
}

func TestCreatedOrder_UnmarshalJSON_InvalidMakerAmountFails(t *testing.T) {
	payload := []byte(`{
		"id":"order-1",
		"createdAt":"2026-03-16T00:00:00Z",
		"makerAmount":"not-a-number",
		"takerAmount":"100000000",
		"expiration":"0",
		"signatureType":0,
		"salt":"1742000000000000",
		"maker":"0xmaker",
		"signer":"0xsigner",
		"taker":"0xtaker",
		"tokenId":"123",
		"side":0,
		"feeRateBps":300,
		"nonce":1,
		"signature":"0xsig",
		"orderType":"GTC",
		"price":"0.52",
		"marketId":42
	}`)

	var order CreatedOrder
	if err := json.Unmarshal(payload, &order); err == nil {
		t.Fatalf("expected unmarshal to fail for invalid makerAmount")
	}
}

func TestCreatedOrder_UnmarshalJSON_InvalidSaltFails(t *testing.T) {
	payload := []byte(`{
		"id":"order-1",
		"createdAt":"2026-03-16T00:00:00Z",
		"makerAmount":"50000000",
		"takerAmount":"100000000",
		"expiration":"0",
		"signatureType":0,
		"salt":"not-a-number",
		"maker":"0xmaker",
		"signer":"0xsigner",
		"taker":"0xtaker",
		"tokenId":"123",
		"side":0,
		"feeRateBps":300,
		"nonce":1,
		"signature":"0xsig",
		"orderType":"GTC",
		"price":"0.52",
		"marketId":42
	}`)

	var order CreatedOrder
	if err := json.Unmarshal(payload, &order); err == nil {
		t.Fatalf("expected unmarshal to fail for invalid salt")
	}
}

func TestCreatedOrder_UnmarshalJSON_OverflowMakerAmountStringFails(t *testing.T) {
	// MaxInt64 + 1 as string
	payload := []byte(`{
		"id":"order-1",
		"createdAt":"2026-03-16T00:00:00Z",
		"makerAmount":"9223372036854775808",
		"takerAmount":"100000000",
		"expiration":"0",
		"signatureType":0,
		"salt":"1742000000000000",
		"maker":"0xmaker",
		"signer":"0xsigner",
		"taker":"0xtaker",
		"tokenId":"123",
		"side":0,
		"feeRateBps":300,
		"nonce":1,
		"signature":"0xsig",
		"orderType":"GTC",
		"price":"0.52",
		"marketId":42
	}`)

	var order CreatedOrder
	if err := json.Unmarshal(payload, &order); err == nil {
		t.Fatalf("expected unmarshal to fail for makerAmount exceeding int64 range")
	}
}

func TestCreatedOrder_UnmarshalJSON_OverflowSaltStringFails(t *testing.T) {
	// Very large salt as string
	payload := []byte(`{
		"id":"order-1",
		"createdAt":"2026-03-16T00:00:00Z",
		"makerAmount":"50000000",
		"takerAmount":"100000000",
		"expiration":"0",
		"signatureType":0,
		"salt":"99999999999999999999",
		"maker":"0xmaker",
		"signer":"0xsigner",
		"taker":"0xtaker",
		"tokenId":"123",
		"side":0,
		"feeRateBps":300,
		"nonce":1,
		"signature":"0xsig",
		"orderType":"GTC",
		"price":"0.52",
		"marketId":42
	}`)

	var order CreatedOrder
	if err := json.Unmarshal(payload, &order); err == nil {
		t.Fatalf("expected unmarshal to fail for salt exceeding int64 range")
	}
}

func TestCreatedOrder_UnmarshalJSON_OverflowTakerAmountStringFails(t *testing.T) {
	payload := []byte(`{
		"id":"order-1",
		"createdAt":"2026-03-16T00:00:00Z",
		"makerAmount":"50000000",
		"takerAmount":"9223372036854775808",
		"expiration":"0",
		"signatureType":0,
		"salt":"1742000000000000",
		"maker":"0xmaker",
		"signer":"0xsigner",
		"taker":"0xtaker",
		"tokenId":"123",
		"side":0,
		"feeRateBps":300,
		"nonce":1,
		"signature":"0xsig",
		"orderType":"GTC",
		"price":"0.52",
		"marketId":42
	}`)

	var order CreatedOrder
	if err := json.Unmarshal(payload, &order); err == nil {
		t.Fatalf("expected unmarshal to fail for takerAmount exceeding int64 range")
	}
}

func TestCreatedOrder_UnmarshalJSON_MaxInt64ValuesSucceed(t *testing.T) {
	// MaxInt64 = 9223372036854775807 — should succeed
	payload := []byte(`{
		"id":"order-1",
		"createdAt":"2026-03-16T00:00:00Z",
		"makerAmount":"9223372036854775807",
		"takerAmount":"9223372036854775807",
		"expiration":"0",
		"signatureType":0,
		"salt":"9223372036854775807",
		"maker":"0xmaker",
		"signer":"0xsigner",
		"taker":"0xtaker",
		"tokenId":"123",
		"side":0,
		"feeRateBps":300,
		"nonce":1,
		"signature":"0xsig",
		"orderType":"GTC",
		"price":"0.52",
		"marketId":42
	}`)

	var order CreatedOrder
	if err := json.Unmarshal(payload, &order); err != nil {
		t.Fatalf("expected unmarshal to succeed for MaxInt64 values, got error: %v", err)
	}
	if order.MakerAmount != 9223372036854775807 {
		t.Fatalf("unexpected makerAmount: got %d", order.MakerAmount)
	}
	if order.Salt != 9223372036854775807 {
		t.Fatalf("unexpected salt: got %d", order.Salt)
	}
}

func TestCreatedOrder_UnmarshalJSON_NullPriceSucceeds(t *testing.T) {
	payload := []byte(`{
		"id":"order-1",
		"createdAt":"2026-03-16T00:00:00Z",
		"makerAmount":"50000000",
		"takerAmount":"100000000",
		"expiration":"0",
		"signatureType":0,
		"salt":"1742000000000000",
		"maker":"0xmaker",
		"signer":"0xsigner",
		"taker":"0xtaker",
		"tokenId":"123",
		"side":0,
		"feeRateBps":300,
		"nonce":1,
		"signature":"0xsig",
		"orderType":"FOK",
		"price":null,
		"marketId":42
	}`)

	var order CreatedOrder
	if err := json.Unmarshal(payload, &order); err != nil {
		t.Fatalf("expected unmarshal to succeed with null price, got error: %v", err)
	}
	if order.Price != nil {
		t.Fatalf("expected nil price, got %v", *order.Price)
	}
}

func TestOrderResponse_UnmarshalJSON_MakerMatchesAllowNullCreatedAt(t *testing.T) {
	payload := []byte(`{
		"order":{
			"id":"order-1",
			"createdAt":"2026-03-16T00:00:00Z",
			"makerAmount":"50000000",
			"takerAmount":"100000000",
			"expiration":"0",
			"signatureType":0,
			"salt":"1742000000000000",
			"maker":"0xmaker",
			"signer":"0xsigner",
			"taker":"0xtaker",
			"tokenId":"123",
			"side":0,
			"feeRateBps":300,
			"nonce":1,
			"signature":"0xsig",
			"orderType":"FAK",
			"price":"0.52",
			"marketId":42
		},
		"makerMatches":[
			{
				"id":"e6ef7cf5-d43b-4927-80d1-23f34feb48d3",
				"createdAt":null,
				"matchedSize":"1000000",
				"orderId":"2c92ce01-e59b-4966-9d3f-a03bdb85e3eb"
			}
		]
	}`)

	var response OrderResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("expected unmarshal to succeed with null maker match createdAt, got error: %v", err)
	}
	if len(response.MakerMatches) != 1 {
		t.Fatalf("expected 1 maker match, got %d", len(response.MakerMatches))
	}
	if response.MakerMatches[0].CreatedAt != nil {
		t.Fatalf("expected nil createdAt, got %v", *response.MakerMatches[0].CreatedAt)
	}
	if response.MakerMatches[0].MatchedSize != "1000000" {
		t.Fatalf("unexpected matchedSize: got %s", response.MakerMatches[0].MatchedSize)
	}
}
