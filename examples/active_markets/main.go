package main

import (
	"context"
	"fmt"
	"log"

	"github.com/limitless-labs-group/limitless-exchange-go-sdk/limitless"
)

func main() {
	client := limitless.NewHttpClient()
	fetcher := limitless.NewMarketFetcher(client)

	ctx := context.Background()

	// Fetch active markets
	resp, err := fetcher.GetActiveMarkets(ctx, &limitless.ActiveMarketsParams{
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
