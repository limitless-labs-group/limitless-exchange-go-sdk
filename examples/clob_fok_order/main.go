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

	// Fetch a market to get venue info (caches for order creation)
	marketSlug := "will-btc-hit-100k"
	market, err := sdk.Markets.GetMarket(ctx, marketSlug)
	if err != nil {
		log.Fatalf("Failed to fetch market: %v", err)
	}

	fmt.Printf("Market: %s\n", market.Title)
	if market.Tokens != nil {
		fmt.Printf("  YES Token: %s\n", market.Tokens.Yes)
		fmt.Printf("  NO Token: %s\n", market.Tokens.No)
	}

	// Create order client
	orderClient, err := sdk.NewOrderClient(privateKey)
	if err != nil {
		log.Fatalf("Failed to create order client: %v", err)
	}

	fmt.Printf("Wallet: %s\n\n", orderClient.WalletAddress())

	// Place a FOK (Fill-or-Kill) market order
	// BUY side: MakerAmount is USDC to spend (e.g., 5 = $5 USDC)
	resp, err := orderClient.CreateOrder(ctx, limitless.CreateOrderParams{
		OrderType:  limitless.OrderTypeFOK,
		MarketSlug: marketSlug,
		Args: limitless.FOKOrderArgs{
			TokenID:     market.Tokens.Yes,
			Side:        limitless.SideBuy,
			MakerAmount: 5.0, // $5 USDC
		},
	})
	if err != nil {
		log.Fatalf("Failed to create order: %v", err)
	}

	fmt.Printf("Order created: %s\n", resp.Order.ID)
	fmt.Printf("  Maker Amount: %d\n", resp.Order.MakerAmount)
	fmt.Printf("  Taker Amount: %d\n", resp.Order.TakerAmount)
	if len(resp.MakerMatches) > 0 {
		fmt.Printf("  Matched: %d fills\n", len(resp.MakerMatches))
	}
}
