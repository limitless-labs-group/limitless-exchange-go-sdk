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

	sdk := limitless.NewClient(limitless.WithAPIKey(apiKey))

	ctx := context.Background()

	// Fetch profile
	profile, err := sdk.Portfolio.GetProfile(ctx, "0xYourWalletAddress")
	if err != nil {
		log.Fatalf("Failed to fetch profile: %v", err)
	}

	fmt.Printf("User ID: %d\n", profile.ID)
	fmt.Printf("Account: %s\n", profile.Account)
	if profile.Rank != nil {
		fmt.Printf("Rank: %s (Fee: %d bps)\n", profile.Rank.Name, profile.Rank.FeeRateBps)
	}

	// Fetch CLOB positions
	clobPositions, err := sdk.Portfolio.GetCLOBPositions(ctx)
	if err != nil {
		log.Fatalf("Failed to fetch CLOB positions: %v", err)
	}

	fmt.Printf("\nCLOB Positions: %d\n", len(clobPositions))
	for _, pos := range clobPositions {
		fmt.Printf("  %s\n", pos.Market.Title)
		fmt.Printf("    YES balance: %s\n", pos.TokensBalance.Yes)
		fmt.Printf("    NO balance: %s\n", pos.TokensBalance.No)
	}

	// Fetch AMM positions
	ammPositions, err := sdk.Portfolio.GetAMMPositions(ctx)
	if err != nil {
		log.Fatalf("Failed to fetch AMM positions: %v", err)
	}

	fmt.Printf("\nAMM Positions: %d\n", len(ammPositions))
	for _, pos := range ammPositions {
		fmt.Printf("  %s (outcome: %d)\n", pos.Market.Title, pos.OutcomeIndex)
		fmt.Printf("    Collateral: %s\n", pos.CollateralAmount)
		fmt.Printf("    PnL: %s\n", pos.UnrealizedPnl)
	}

	// Fetch user history
	history, err := sdk.Portfolio.GetUserHistory(ctx, 1, 10)
	if err != nil {
		log.Fatalf("Failed to fetch history: %v", err)
	}

	fmt.Printf("\nHistory: %d entries (total: %d)\n", len(history.Data), history.TotalCount)
	for _, entry := range history.Data {
		fmt.Printf("  [%s] %s", entry.Type, entry.CreatedAt)
		if entry.MarketSlug != nil {
			fmt.Printf(" - %s", *entry.MarketSlug)
		}
		fmt.Println()
	}
}
