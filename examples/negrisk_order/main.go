package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/limitless-labs-group/limitless-exchange-go-sdk/limitless"
)

func main() {
	apiKey := os.Getenv("LIMITLESS_API_KEY")
	if apiKey == "" {
		log.Fatal("LIMITLESS_API_KEY environment variable is required")
	}

	privateKey := os.Getenv("PRIVATE_KEY")
	if privateKey == "" {
		log.Fatal("PRIVATE_KEY environment variable is required")
	}

	sdk := limitless.NewClient(limitless.WithAPIKey(apiKey))

	ctx := context.Background()

	// Fetch a NegRisk group market
	// NegRisk markets have sub-markets with their own tokens
	marketSlug := "us-presidential-election-2024"
	market, err := sdk.Markets.GetMarket(ctx, marketSlug)
	if err != nil {
		log.Fatalf("Failed to fetch market: %v", err)
	}

	fmt.Printf("Group Market: %s\n", market.Title)
	fmt.Printf("  Sub-markets: %d\n\n", len(market.Markets))

	if len(market.Markets) == 0 {
		log.Fatal("No sub-markets found")
	}

	// Pick the first sub-market
	subMarket := market.Markets[0]
	fmt.Printf("Sub-market: %s\n", subMarket.Title)

	if subMarket.Tokens == nil {
		log.Fatal("Sub-market has no tokens")
	}

	// Create order client
	orderClient, err := sdk.NewOrderClient(privateKey)
	if err != nil {
		log.Fatalf("Failed to create order client: %v", err)
	}

	// Place a GTC order on the sub-market
	resp, err := orderClient.CreateOrder(ctx, limitless.CreateOrderParams{
		OrderType:  limitless.OrderTypeGTC,
		MarketSlug: subMarket.Slug,
		Args: limitless.GTCOrderArgs{
			TokenID: subMarket.Tokens.Yes,
			Side:    limitless.SideBuy,
			Price:   0.600,
			Size:    5.0,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create order: %v", err)
	}

	fmt.Printf("Order created: %s\n", resp.Order.ID)
}
