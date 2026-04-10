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

	// Fetch market and orderbook so we can inspect the current midpoint.
	marketSlug := "will-btc-hit-100k"
	market, err := sdk.Markets.GetMarket(ctx, marketSlug)
	if err != nil {
		log.Fatalf("Failed to fetch market: %v", err)
	}

	orderbook, err := sdk.Markets.GetOrderBook(ctx, marketSlug)
	if err != nil {
		log.Fatalf("Failed to fetch orderbook: %v", err)
	}

	fmt.Printf("Market: %s\n", market.Title)
	fmt.Printf("Orderbook midpoint: %.3f\n\n", orderbook.AdjustedMidpoint)

	orderClient, err := sdk.NewOrderClient(privateKey)
	if err != nil {
		log.Fatalf("Failed to create order client: %v", err)
	}

	// Place a FAK (Fill-And-Kill) limit order.
	// FAK uses the same price + size inputs as GTC, but any unmatched remainder
	// is cancelled immediately instead of resting on the book.
	resp, err := orderClient.CreateOrder(ctx, limitless.CreateOrderParams{
		OrderType:  limitless.OrderTypeFAK,
		MarketSlug: marketSlug,
		Args: limitless.FAKOrderArgs{
			TokenID: market.Tokens.Yes,
			Side:    limitless.SideBuy,
			Price:   0.450, // Max price willing to pay
			Size:    10.0,  // Shares to buy
		},
	})
	if err != nil {
		log.Fatalf("Failed to create FAK order: %v", err)
	}

	fmt.Printf("FAK order created: %s\n", resp.Order.ID)
	fmt.Printf("  Price: %v\n", resp.Order.Price)
	fmt.Printf("  Maker Amount: %d\n", resp.Order.MakerAmount)
	fmt.Printf("  Taker Amount: %d\n", resp.Order.TakerAmount)

	if len(resp.MakerMatches) > 0 {
		fmt.Printf("  Matched immediately with %d fill(s)\n", len(resp.MakerMatches))
		return
	}

	fmt.Println("  No immediate match. Unfilled remainder was cancelled by FAK semantics.")
}
