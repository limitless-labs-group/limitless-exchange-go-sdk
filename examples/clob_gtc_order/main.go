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

	// Fetch market and orderbook
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

	// Create order client
	orderClient, err := sdk.NewOrderClient(privateKey)
	if err != nil {
		log.Fatalf("Failed to create order client: %v", err)
	}

	// Place a GTC (Good-Til-Cancelled) limit order
	// Price must be tick-aligned (multiple of 0.001)
	// Size must produce contracts divisible by sharesStep
	resp, err := orderClient.CreateOrder(ctx, limitless.CreateOrderParams{
		OrderType:  limitless.OrderTypeGTC,
		MarketSlug: marketSlug,
		Args: limitless.GTCOrderArgs{
			TokenID: market.Tokens.Yes,
			Side:    limitless.SideBuy,
			Price:   0.500, // 50 cents
			Size:    10.0,  // 10 shares
		},
	})
	if err != nil {
		log.Fatalf("Failed to create order: %v", err)
	}

	fmt.Printf("Order created: %s\n", resp.Order.ID)
	fmt.Printf("  Price: %v\n", resp.Order.Price)
	fmt.Printf("  Maker Amount: %d\n", resp.Order.MakerAmount)
	fmt.Printf("  Taker Amount: %d\n", resp.Order.TakerAmount)

	// Cancel the order
	msg, err := orderClient.Cancel(ctx, resp.Order.ID)
	if err != nil {
		log.Fatalf("Failed to cancel order: %v", err)
	}
	fmt.Printf("Order cancelled: %s\n", msg)
}
