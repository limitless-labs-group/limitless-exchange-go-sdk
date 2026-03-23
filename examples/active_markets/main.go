package main

import (
	"context"
	"fmt"
	"log"

	"github.com/limitless-labs-group/limitless-exchange-go-sdk/limitless"
)

func main() {
	sdk := limitless.NewClient()

	ctx := context.Background()

	// Fetch active markets
	resp, err := sdk.Markets.GetActiveMarkets(ctx, &limitless.ActiveMarketsParams{
		Limit:  5,
		SortBy: "newest",
	})
	if err != nil {
		log.Fatalf("Failed to fetch active markets: %v", err)
	}

	fmt.Printf("Total markets: %d\n\n", resp.TotalMarketsCount)

	for _, m := range resp.Data {
		fmt.Printf("Market: %s\n", m.Title)
		fmt.Printf("  Slug: %s\n", m.Slug)
		fmt.Printf("  Status: %s\n", m.Status)
		fmt.Printf("  Trade Type: %s\n", m.TradeType)
		if len(m.Prices) > 0 {
			fmt.Printf("  Prices: %v\n", m.Prices)
		}
		fmt.Println()
	}
}
