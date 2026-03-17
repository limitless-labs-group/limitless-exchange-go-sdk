package limitless

import "fmt"

const (
	// DefaultAPIURL is the default Limitless Exchange API URL.
	DefaultAPIURL = "https://api.limitless.exchange"

	// DefaultWSURL is the default WebSocket URL for real-time data.
	DefaultWSURL = "wss://ws.limitless.exchange"

	// DefaultChainID is the default chain ID (Base mainnet).
	DefaultChainID = 8453

	// BaseSepoliaChainID is the Base Sepolia testnet chain ID.
	BaseSepoliaChainID = 84532

	// ZeroAddress is the Ethereum zero address constant.
	ZeroAddress = "0x0000000000000000000000000000000000000000"

	// SigningMessageTemplate is used by the API for identity verification.
	SigningMessageTemplate = "Welcome to Limitless.exchange! Please sign this message to verify your identity.\n\nNonce: {NONCE}"
)

// ContractAddresses maps chain IDs to contract addresses.
var ContractAddresses = map[int]map[string]string{
	8453: {
		"USDC": "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
		"CTF":  "0xC9c98965297Bc527861c898329Ee280632B76e18",
	},
	84532: {
		"USDC": "0x...",
		"CTF":  "0x...",
	},
}

// GetContractAddress returns the contract address for a given type and chain ID.
// contractType must be "USDC" or "CTF". chainID defaults to DefaultChainID if not provided.
func GetContractAddress(contractType string, chainID ...int) (string, error) {
	cid := DefaultChainID
	if len(chainID) > 0 {
		cid = chainID[0]
	}

	addresses, ok := ContractAddresses[cid]
	if !ok {
		return "", fmt.Errorf("no contract addresses configured for chainId %d", cid)
	}

	address, ok := addresses[contractType]
	if !ok || address == "" || address == "0x..." {
		return "", fmt.Errorf("contract address for %s not available on chainId %d", contractType, cid)
	}

	return address, nil
}
