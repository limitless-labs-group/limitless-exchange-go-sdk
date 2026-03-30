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
	apiKey := os.Getenv("LIMITLESS_API_KEY")
	tokenID := os.Getenv("LIMITLESS_API_TOKEN_ID")
	tokenSecret := os.Getenv("LIMITLESS_API_TOKEN_SECRET")

	opts := []limitless.ClientOption{
		limitless.WithLogger(limitless.NewConsoleLogger(limitless.LogLevelInfo)),
	}
	switch {
	case tokenID != "" || tokenSecret != "":
		if tokenID == "" || tokenSecret == "" {
			log.Fatal("both LIMITLESS_API_TOKEN_ID and LIMITLESS_API_TOKEN_SECRET are required for scoped API-key auth")
		}
		opts = append(opts, limitless.WithHMACCredentials(limitless.HMACCredentials{
			TokenID: tokenID,
			Secret:  tokenSecret,
		}))
	case apiKey != "":
		opts = append(opts, limitless.WithAPIKey(apiKey))
	default:
		log.Fatal("LIMITLESS_API_KEY or LIMITLESS_API_TOKEN_ID/LIMITLESS_API_TOKEN_SECRET is required for authenticated subscriptions")
	}

	sdk := limitless.NewClient(opts...)
	ws := sdk.NewWebSocketClient()

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
	marketSlug := os.Getenv("MARKET_SLUG")
	if marketSlug == "" {
		marketSlug = "will-btc-hit-100k"
	}
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
