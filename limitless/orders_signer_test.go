package limitless

import (
	"testing"
)

const (
	testPrivateKeyHex      = "0x59c6995e998f97a5a0044966f0945382d7f84be58f4d1e8e8f8d0f9f5c5e7d5a"
	testSignerAddress      = "0xa00BCB04073B243E8A55f3B5899AefF596bF17C6"
	expectedOrderSignature = "0x0cd29c83ce6390a8adba01ae3d49a0dbf875bd8d06902f6381676a11159afbcf22c35f5acaea3aca85152f254851e548946fe56f68f793ced0b08c2626729adc1c"
)

func testUnsignedOrderForSigner() *UnsignedOrder {
	price := 0.381
	return &UnsignedOrder{
		Salt:          1742191300000000,
		Maker:         testSignerAddress,
		Signer:        testSignerAddress,
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

func TestNewOrderSigner_InvalidPrivateKey(t *testing.T) {
	t.Parallel()

	_, err := NewOrderSigner("not-a-valid-key")
	if err == nil {
		t.Fatal("expected invalid private key error, got nil")
	}
}

func TestOrderSigner_AddressDerivation(t *testing.T) {
	t.Parallel()

	signer, err := NewOrderSigner(testPrivateKeyHex)
	if err != nil {
		t.Fatalf("NewOrderSigner returned error: %v", err)
	}

	if signer.Address().Hex() != testSignerAddress {
		t.Fatalf("expected signer address %s, got %s", testSignerAddress, signer.Address().Hex())
	}
}

func TestOrderSigner_SignOrder_DeterministicVector(t *testing.T) {
	t.Parallel()

	signer, err := NewOrderSigner(testPrivateKeyHex)
	if err != nil {
		t.Fatalf("NewOrderSigner returned error: %v", err)
	}

	config := OrderSigningConfig{
		ChainID:         8453,
		ContractAddress: "0xa4409D988CA2218d956BeEFD3874100F444f0DC3",
	}
	sig, err := signer.SignOrder(testUnsignedOrderForSigner(), config)
	if err != nil {
		t.Fatalf("SignOrder returned error: %v", err)
	}

	if sig != expectedOrderSignature {
		t.Fatalf("unexpected signature:\nexpected: %s\nactual:   %s", expectedOrderSignature, sig)
	}
}

func TestOrderSigner_SignOrder_RejectsWalletMismatch(t *testing.T) {
	t.Parallel()

	signer, err := NewOrderSigner(testPrivateKeyHex)
	if err != nil {
		t.Fatalf("NewOrderSigner returned error: %v", err)
	}

	order := testUnsignedOrderForSigner()
	order.Signer = "0x1111111111111111111111111111111111111111"

	_, err = signer.SignOrder(order, OrderSigningConfig{
		ChainID:         8453,
		ContractAddress: "0xa4409D988CA2218d956BeEFD3874100F444f0DC3",
	})
	if err == nil {
		t.Fatal("expected wallet mismatch error, got nil")
	}
}
