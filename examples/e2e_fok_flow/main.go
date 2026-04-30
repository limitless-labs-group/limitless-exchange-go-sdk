package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/limitless-labs-group/limitless-exchange-go-sdk/limitless"
)

func requireEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		log.Fatalf("%s environment variable is required", key)
	}
	return value
}

func optionalEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envFlag(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func optionalPositiveInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		log.Fatalf("%s must be a zero-or-positive integer", key)
	}
	return parsed
}

func boolPtr(value bool) *bool {
	return &value
}

func main() {
	ctx := context.Background()
	identityToken := requireEnv("LIMITLESS_IDENTITY_TOKEN")
	marketSlug := requireEnv("MARKET_SLUG")
	partnerName := optionalEnv("PARTNER_NAME", "partner")

	bootstrap := limitless.NewClient(
		limitless.WithLogger(limitless.NewConsoleLogger(limitless.LogLevelInfo)),
	)

	requestedScopes := []string{
		limitless.ScopeTrading,
		limitless.ScopeDelegatedSigning,
		limitless.ScopeAccountCreation,
	}

	fmt.Println("1. Read current partner capabilities with the Privy identity token.")
	capabilities, err := bootstrap.ApiTokens.GetCapabilities(ctx, identityToken)
	if err != nil {
		log.Fatalf("Failed to fetch capabilities: %v", err)
	}
	fmt.Printf(
		"   Capabilities: enabled=%t allowedScopes=%v\n",
		capabilities.TokenManagementEnabled,
		capabilities.AllowedScopes,
	)

	fmt.Println("2. Derive a scoped HMAC token for partner operations.")
	derived, err := bootstrap.ApiTokens.DeriveToken(ctx, identityToken, limitless.DeriveApiTokenInput{
		Label:  fmt.Sprintf("go-sdk-e2e-fok-flow-%d", time.Now().Unix()),
		Scopes: requestedScopes,
	})
	if err != nil {
		log.Fatalf("Failed to derive scoped token: %v", err)
	}
	fmt.Printf(
		"   Derived token: tokenId=%s profileId=%d scopes=%v\n",
		derived.TokenID,
		derived.Profile.ID,
		derived.Scopes,
	)

	scopedClient := limitless.NewClient(
		limitless.WithHMACCredentials(limitless.HMACCredentials{
			TokenID: derived.TokenID,
			Secret:  derived.Secret,
		}),
		limitless.WithLogger(limitless.NewConsoleLogger(limitless.LogLevelInfo)),
	)

	fmt.Println("3. Verify the derived HMAC token works on authenticated partner endpoints.")
	activeTokens, err := scopedClient.ApiTokens.ListTokens(ctx)
	if err != nil {
		log.Fatalf("Failed to list scoped tokens: %v", err)
	}
	fmt.Printf("   Active tokens visible to scoped client: %d\n", len(activeTokens))

	fmt.Println("4. Fetch the market that will be used for delegated trading.")
	market, err := bootstrap.Markets.GetMarket(ctx, marketSlug)
	if err != nil {
		log.Fatalf("Failed to fetch market: %v", err)
	}
	if market.Tokens == nil {
		log.Fatal("market has no tokens")
	}
	if market.Venue == nil {
		log.Fatal("market has no venue")
	}
	if market.CollateralToken.Address == "" || market.CollateralToken.Symbol == "" {
		log.Fatal("market has incomplete collateral token information")
	}
	fmt.Printf(
		"   Market: slug=%s exchange=%s collateral=%s(%s)\n",
		market.Slug,
		market.Venue.Exchange,
		market.CollateralToken.Symbol,
		market.CollateralToken.Address,
	)

	fmt.Println("5. Create a partner-owned child account with a server wallet.")
	partnerAccount, err := scopedClient.PartnerAccounts.CreateAccount(ctx, limitless.CreatePartnerAccountInput{
		DisplayName:        fmt.Sprintf("%s-e2e-fok-%d", partnerName, time.Now().Unix()),
		CreateServerWallet: boolPtr(true),
	}, nil)
	if err != nil {
		log.Fatalf("Failed to create partner account: %v", err)
	}
	fmt.Printf(
		"   Created partner account: profileId=%d account=%s\n",
		partnerAccount.ProfileID,
		partnerAccount.Account,
	)

	fmt.Println("6. Important: fund the created account before attempting to trade.")
	fmt.Printf(
		"   Fund %s with %s on %s.\n",
		partnerAccount.Account,
		market.CollateralToken.Symbol,
		market.CollateralToken.Address,
	)
	fmt.Println("   Check delegated allowances with PartnerAccounts.CheckAllowances; retry missing or failed targets with RetryAllowances, then poll again.")

	readyDelayMs := optionalPositiveInt("LIMITLESS_DELEGATED_ACCOUNT_READY_DELAY_MS", 10_000)
	if readyDelayMs > 0 {
		fmt.Printf("   Waiting %dms before the delegated FOK trade step...\n", readyDelayMs)
		time.Sleep(time.Duration(readyDelayMs) * time.Millisecond)
	}

	if !envFlag("LIMITLESS_PLACE_DELEGATED_ORDER") {
		fmt.Println("7. Trading step skipped.")
		fmt.Println("   Re-run with LIMITLESS_PLACE_DELEGATED_ORDER=1 after funding the created account.")
		return
	}

	fmt.Println("7. Place a delegated FOK order with the HMAC-scoped client.")
	fmt.Printf(
		"   Delegated FOK context: onBehalfOf=%d account=%s exchange=%s collateral=%s(%s)\n",
		partnerAccount.ProfileID,
		partnerAccount.Account,
		market.Venue.Exchange,
		market.CollateralToken.Symbol,
		market.CollateralToken.Address,
	)

	makerAmount := 1.0
	fmt.Printf("   FOK makerAmount=%.2f USDC side=BUY\n", makerAmount)

	order, err := scopedClient.DelegatedOrders.CreateOrder(ctx, limitless.CreateDelegatedOrderParams{
		MarketSlug: marketSlug,
		OrderType:  limitless.OrderTypeFOK,
		OnBehalfOf: partnerAccount.ProfileID,
		Args: limitless.FOKOrderArgs{
			TokenID:     market.Tokens.Yes,
			Side:        limitless.SideBuy,
			MakerAmount: makerAmount,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create delegated FOK order: %v", err)
	}
	fmt.Printf("   Created delegated FOK order: orderId=%s\n", order.Order.ID)

	fmt.Println("8. No cleanup step for this flow.")
	if len(order.MakerMatches) > 0 {
		fmt.Printf("   Delegated FOK order matched immediately with %d fill(s).\n", len(order.MakerMatches))
	} else {
		fmt.Println("   Delegated FOK order was not matched and auto-cancelled by FOK semantics.")
	}
}
