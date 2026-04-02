package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/limitless-labs-group/limitless-exchange-go-sdk/limitless"
)

func mustIntEnv(key string) int {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s environment variable is required", key)
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("%s must be an integer: %v", key, err)
	}

	return parsed
}

func main() {
	tokenID := os.Getenv("LIMITLESS_API_TOKEN_ID")
	tokenSecret := os.Getenv("LIMITLESS_API_TOKEN_SECRET")
	marketSlug := os.Getenv("MARKET_SLUG")

	if tokenID == "" || tokenSecret == "" {
		log.Fatal("LIMITLESS_API_TOKEN_ID and LIMITLESS_API_TOKEN_SECRET are required")
	}
	if marketSlug == "" {
		log.Fatal("MARKET_SLUG environment variable is required")
	}

	targetProfileID := mustIntEnv("LIMITLESS_PARTNER_PROFILE_ID")
	feeRateBps := mustIntEnv("LIMITLESS_TARGET_FEE_RATE_BPS")
	makerAmount := 1.0

	sdk := limitless.NewClient(
		limitless.WithHMACCredentials(limitless.HMACCredentials{
			TokenID: tokenID,
			Secret:  tokenSecret,
		}),
		limitless.WithLogger(limitless.NewConsoleLogger(limitless.LogLevelInfo)),
	)

	ctx := context.Background()
	market, err := sdk.Markets.GetMarket(ctx, marketSlug)
	if err != nil {
		log.Fatalf("Failed to fetch market: %v", err)
	}
	if market.Tokens == nil {
		log.Fatal("market has no tokens")
	}

	fmt.Printf(
		"Submitting delegated FOK BUY order: onBehalfOf=%d makerAmount=%.2f USDC\n",
		targetProfileID,
		makerAmount,
	)

	resp, err := sdk.DelegatedOrders.CreateOrder(ctx, limitless.CreateDelegatedOrderParams{
		MarketSlug: marketSlug,
		OrderType:  limitless.OrderTypeFOK,
		OnBehalfOf: targetProfileID,
		FeeRateBps: feeRateBps,
		Args: limitless.FOKOrderArgs{
			TokenID:     market.Tokens.Yes,
			Side:        limitless.SideBuy,
			MakerAmount: makerAmount,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create delegated FOK order: %v", err)
	}

	fmt.Printf("Delegated FOK order created: %s\n", resp.Order.ID)
	if len(resp.MakerMatches) > 0 {
		fmt.Printf("Delegated FOK order fully matched with %d fill(s)\n", len(resp.MakerMatches))
		return
	}

	fmt.Println("Delegated FOK order was not matched and was cancelled automatically.")
}
