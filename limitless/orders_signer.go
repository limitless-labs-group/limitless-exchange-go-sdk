package limitless

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// OrderSigner handles EIP-712 signing for Limitless Exchange orders.
type OrderSigner struct {
	privateKey *ecdsa.PrivateKey
	address    common.Address
	logger     Logger
}

// SignerOption configures an OrderSigner.
type SignerOption func(*OrderSigner)

// WithSignerLogger sets the logger for the OrderSigner.
func WithSignerLogger(l Logger) SignerOption {
	return func(s *OrderSigner) {
		s.logger = l
	}
}

// NewOrderSigner creates a new EIP-712 order signer from a hex-encoded private key.
// The private key may optionally include the "0x" prefix.
func NewOrderSigner(privateKeyHex string, opts ...SignerOption) (*OrderSigner, error) {
	hex := strings.TrimPrefix(privateKeyHex, "0x")
	key, err := crypto.HexToECDSA(hex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	addr := crypto.PubkeyToAddress(key.PublicKey)

	s := &OrderSigner{
		privateKey: key,
		address:    addr,
		logger:     NewNoOpLogger(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Address returns the Ethereum address derived from the private key.
func (s *OrderSigner) Address() common.Address {
	return s.address
}

// SignOrder signs an unsigned order with EIP-712 and returns the hex-encoded signature.
func (s *OrderSigner) SignOrder(order *UnsignedOrder, config OrderSigningConfig) (string, error) {
	s.logger.Debug("Signing order with EIP-712", map[string]any{
		"tokenId":           order.TokenID,
		"side":              order.Side,
		"verifyingContract": config.ContractAddress,
	})

	// Verify wallet address matches order signer
	if !strings.EqualFold(s.address.Hex(), order.Signer) {
		return "", fmt.Errorf("wallet address mismatch! Signing with: %s, but order signer is: %s",
			s.address.Hex(), order.Signer)
	}

	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"Order": {
				{Name: "salt", Type: "uint256"},
				{Name: "maker", Type: "address"},
				{Name: "signer", Type: "address"},
				{Name: "taker", Type: "address"},
				{Name: "tokenId", Type: "uint256"},
				{Name: "makerAmount", Type: "uint256"},
				{Name: "takerAmount", Type: "uint256"},
				{Name: "expiration", Type: "uint256"},
				{Name: "nonce", Type: "uint256"},
				{Name: "feeRateBps", Type: "uint256"},
				{Name: "side", Type: "uint8"},
				{Name: "signatureType", Type: "uint8"},
			},
		},
		PrimaryType: "Order",
		Domain: apitypes.TypedDataDomain{
			Name:              "Limitless CTF Exchange",
			Version:           "1",
			ChainId:           math.NewHexOrDecimal256(int64(config.ChainID)),
			VerifyingContract: config.ContractAddress,
		},
		Message: apitypes.TypedDataMessage{
			"salt":          fmt.Sprintf("%d", order.Salt),
			"maker":         order.Maker,
			"signer":        order.Signer,
			"taker":         order.Taker,
			"tokenId":       order.TokenID,
			"makerAmount":   fmt.Sprintf("%d", order.MakerAmount),
			"takerAmount":   fmt.Sprintf("%d", order.TakerAmount),
			"expiration":    order.Expiration,
			"nonce":         fmt.Sprintf("%d", order.Nonce),
			"feeRateBps":    fmt.Sprintf("%d", order.FeeRateBps),
			"side":          fmt.Sprintf("%d", order.Side),
			"signatureType": fmt.Sprintf("%d", order.SignatureType),
		},
	}

	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return "", fmt.Errorf("failed to hash domain: %w", err)
	}

	typedDataHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return "", fmt.Errorf("failed to hash message: %w", err)
	}

	// Create the EIP-712 hash: keccak256("\x19\x01" + domainSeparator + structHash)
	rawData := []byte{0x19, 0x01}
	rawData = append(rawData, domainSeparator...)
	rawData = append(rawData, typedDataHash...)
	hash := crypto.Keccak256Hash(rawData)

	// Sign the hash
	sig, err := crypto.Sign(hash.Bytes(), s.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}

	// Adjust V value: go-ethereum returns 0/1, EIP-712 expects 27/28
	if sig[64] < 27 {
		sig[64] += 27
	}

	signature := fmt.Sprintf("0x%s", common.Bytes2Hex(sig))

	s.logger.Info("Successfully generated EIP-712 signature", map[string]any{
		"signature": signature[:12] + "...",
	})

	return signature, nil
}

// bigIntFromInt64 creates a big.Int from an int64. (unused but kept for reference)
func bigIntFromInt64(v int64) *big.Int {
	return new(big.Int).SetInt64(v)
}
