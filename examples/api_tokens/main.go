package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/limitless-labs-group/limitless-exchange-go-sdk/limitless"
)

func main() {
	tokenID := os.Getenv("LIMITLESS_API_TOKEN_ID")
	tokenSecret := os.Getenv("LIMITLESS_API_TOKEN_SECRET")
	identityToken := os.Getenv("LIMITLESS_IDENTITY_TOKEN")

	if tokenID == "" || tokenSecret == "" {
		log.Fatal("LIMITLESS_API_TOKEN_ID and LIMITLESS_API_TOKEN_SECRET are required")
	}

	sdk := limitless.NewClient(
		limitless.WithHMACCredentials(limitless.HMACCredentials{
			TokenID: tokenID,
			Secret:  tokenSecret,
		}),
		limitless.WithLogger(limitless.NewConsoleLogger(limitless.LogLevelInfo)),
	)

	ctx := context.Background()

	tokens, err := sdk.ApiTokens.ListTokens(ctx)
	if err != nil {
		log.Fatalf("Failed to list API tokens: %v", err)
	}

	fmt.Printf("Active tokens: %d\n", len(tokens))
	for _, token := range tokens {
		fmt.Printf("- %s scopes=%v createdAt=%s\n", token.TokenID, token.Scopes, token.CreatedAt)
	}

	if identityToken == "" {
		fmt.Println("\nSet LIMITLESS_IDENTITY_TOKEN to also fetch partner capabilities.")
		return
	}

	capabilities, err := sdk.ApiTokens.GetCapabilities(ctx, identityToken)
	if err != nil {
		log.Fatalf("Failed to fetch capabilities: %v", err)
	}

	fmt.Printf("\nPartner profile: %d\n", capabilities.PartnerProfileID)
	fmt.Printf("Token management enabled: %t\n", capabilities.TokenManagementEnabled)
	fmt.Printf("Allowed scopes: %v\n", capabilities.AllowedScopes)
}
