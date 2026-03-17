package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/limitless-labs-group/limitless-exchange-go-sdk/limitless"
)

func main() {
	// API key required for authenticated subscriptions
	apiKey := os.Getenv("LIMITLESS_API_KEY")
	if apiKey == "" {
		log.Fatal("LIMITLESS_API_KEY environment variable is required for position subscriptions")
	}

	ws := limitless.NewWebSocketClient(
		limitless.WithWebSocketAPIKey(apiKey),
		limitless.WithWebSocketLogger(limitless.NewConsoleLogger(limitless.LogLevelInfo)),
	)

	// Register event handlers
	ws.On("positions", func(data json.RawMessage) {
		fmt.Printf("\nPosition update: %s\n", string(data))
	})

	ws.OnTransaction(func(tx limitless.TransactionEvent) {
		fmt.Printf("\nTransaction: status=%s source=%s\n", tx.Status, tx.Source)
		if tx.MarketSlug != nil {
			fmt.Printf("  Market: %s\n", *tx.MarketSlug)
		}
		if tx.TxHash != nil {
			fmt.Printf("  TxHash: %s\n", *tx.TxHash)
		}
	})

	ctx := context.Background()

	// Connect
	if err := ws.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer ws.Disconnect()

	// Subscribe to position updates
	marketSlug := "will-btc-hit-100k"
	if err := ws.Subscribe(ctx, limitless.ChannelSubscribePositions, limitless.SubscriptionOptions{
		MarketSlugs: []string{marketSlug},
	}); err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	// Subscribe to transaction events
	if err := ws.Subscribe(ctx, limitless.ChannelSubscribeTransactions, limitless.SubscriptionOptions{}); err != nil {
		log.Fatalf("Failed to subscribe to transactions: %v", err)
	}

	fmt.Printf("Subscribed to positions for %s. Press Ctrl+C to exit.\n", marketSlug)

	// Wait for interrupt
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("\nShutting down...")
}
