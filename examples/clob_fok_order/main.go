package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/limitless-labs-group/limitless-exchange-go-sdk/limitless"
)

func main() {
	privateKey := os.Getenv("PRIVATE_KEY")
	if privateKey == "" {
		log.Fatal("PRIVATE_KEY environment variable is required")
	}

	client := limitless.NewHttpClient()
	fetcher := limitless.NewMarketFetcher(client)

	ctx := context.Background()

	// Fetch a market to get venue info (caches for order creation)
	marketSlug := "will-btc-hit-100k"
	market, err := fetcher.GetMarket(ctx, marketSlug)
	if err != nil {
		log.Fatalf("Failed to fetch market: %v", err)
	}

	fmt.Printf("Market: %s\n", market.Title)
	if market.Tokens != nil {
		fmt.Printf("  YES Token: %s\n", market.Tokens.Yes)
		fmt.Printf("  NO Token: %s\n", market.Tokens.No)
	}

	// Create order client
	orderClient, err := limitless.NewOrderClient(
		client,
		privateKey,
		limitless.WithOrderMarketFetcher(fetcher),
	)
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
