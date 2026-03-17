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
