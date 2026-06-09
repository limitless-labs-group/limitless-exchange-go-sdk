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

	// Place a GTC limit order with a self-trade-prevention policy.
	//
	// StpPolicy decides what happens when this order would match against one of
	// your own resting orders:
	//   - StpPolicyCancelMaker: cancel your resting maker order, let this one
	//     continue (this is also the engine default when StpPolicy is empty).
	//   - StpPolicyCancelTaker: cancel this incoming order, keep your resting one.
	//   - StpPolicyCancelBoth: cancel both.
	//
	// The policy is sent top-level on the request, never inside the signed
	// EIP-712 order, so it does not affect the signature.
	resp, err := orderClient.CreateOrder(ctx, limitless.CreateOrderParams{
		OrderType:  limitless.OrderTypeGTC,
		MarketSlug: marketSlug,
		StpPolicy:  limitless.StpPolicyCancelMaker,
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

	// The response now surfaces the execution object describing how the order
	// was matched and settled. On an STP self-match, Execution.Reason carries
	// the taker signal (for example STP_TAKER_REJECTED) and StpMakerCancels
	// lists any canceled maker order UUIDs.
	exec := resp.Execution
	fmt.Printf("Execution:\n")
	fmt.Printf("  Matched: %t\n", exec.Matched)
	fmt.Printf("  Settlement status: %s\n", exec.SettlementStatus)
	fmt.Printf("  Effective fee bps: %d\n", exec.EffectiveFeeBps)
	fmt.Printf("  USD net: %s\n", exec.TotalsRaw.UsdNet)
	if exec.Reason != "" {
		fmt.Printf("  Reason: %s\n", exec.Reason)
	}
	if len(exec.StpMakerCancels) > 0 {
		fmt.Printf("  STP maker cancels: %v\n", exec.StpMakerCancels)
	}

	// Cancel the order if it is still resting.
	msg, err := orderClient.Cancel(ctx, resp.Order.ID)
	if err != nil {
		log.Fatalf("Failed to cancel order: %v", err)
	}
	fmt.Printf("Order cancelled: %s\n", msg)
}
