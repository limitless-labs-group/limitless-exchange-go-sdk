package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/limitless-labs-group/limitless-exchange-go-sdk/limitless"
)

func main() {
	sdk := limitless.NewClient(
		limitless.WithLogger(limitless.NewConsoleLogger(limitless.LogLevelInfo)),
	)

	// No API key needed for public subscriptions
	ws := sdk.NewWebSocketClient()

	// Register event handler before connecting
	ws.OnOrderbookUpdate(func(update limitless.OrderbookUpdate) {
		fmt.Printf("\nOrderbook update for %s:\n", update.MarketSlug)
		fmt.Printf("  Bids: %d, Asks: %d\n", len(update.Orderbook.Bids), len(update.Orderbook.Asks))
		fmt.Printf("  Midpoint: %.3f\n", update.Orderbook.AdjustedMidpoint)

		if len(update.Orderbook.Bids) > 0 {
			fmt.Printf("  Best bid: %.3f (size: %.2f)\n",
				update.Orderbook.Bids[0].Price, update.Orderbook.Bids[0].Size)
		}
		if len(update.Orderbook.Asks) > 0 {
			fmt.Printf("  Best ask: %.3f (size: %.2f)\n",
				update.Orderbook.Asks[0].Price, update.Orderbook.Asks[0].Size)
		}
	})

	ctx := context.Background()

	// Connect
	if err := ws.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer ws.Disconnect()

	// Subscribe to orderbook updates
	marketSlug := "will-btc-hit-100k"
	if err := ws.Subscribe(ctx, limitless.ChannelOrderbook, limitless.SubscriptionOptions{
		MarketSlugs: []string{marketSlug},
	}); err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	fmt.Printf("Subscribed to orderbook for %s. Press Ctrl+C to exit.\n", marketSlug)

	// Wait for interrupt
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("\nShutting down...")
}
