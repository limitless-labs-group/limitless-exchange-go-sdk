package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/limitless-labs-group/limitless-exchange-go-sdk/limitless"
)

func requireEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		log.Fatalf("%s environment variable is required", key)
	}
	return value
}

func optionalEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func partnerAccountProfileIDFromEnv() int {
	for _, key := range []string{"LIMITLESS_PARTNER_ACCOUNT_PROFILE_ID", "LIMITLESS_ON_BEHALF_OF"} {
		value := optionalEnv(key)
		if value == "" {
			continue
		}

		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			log.Fatalf("%s must be a positive integer", key)
		}
		return parsed
	}

	log.Fatal("LIMITLESS_PARTNER_ACCOUNT_PROFILE_ID environment variable is required")
	return 0
}

func envFlag(key string, fallback bool) bool {
	value := strings.ToLower(optionalEnv(key))
	if value == "" {
		return fallback
	}

	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		log.Fatalf("%s must be one of 1,true,yes,on,0,false,no,off", key)
		return fallback
	}
}

func main() {
	ctx := context.Background()
	profileID := partnerAccountProfileIDFromEnv()
	skipRetry := envFlag("LIMITLESS_SKIP_ALLOWANCE_RETRY", false)

	sdk := limitless.NewClient(
		limitless.WithHMACCredentials(limitless.HMACCredentials{
			TokenID: requireEnv("LIMITLESS_API_TOKEN_ID"),
			Secret:  requireEnv("LIMITLESS_API_TOKEN_SECRET"),
		}),
		limitless.WithLogger(limitless.NewConsoleLogger(limitless.LogLevelInfo)),
	)

	fmt.Printf("GET /profiles/partner-accounts/%d/allowances\n", profileID)
	allowances, err := sdk.PartnerAccounts.CheckAllowances(ctx, profileID)
	if err != nil {
		log.Fatalf("Failed to check partner-account allowances: %v", err)
	}
	printAllowanceResponse(allowances)

	if allowances.Ready {
		fmt.Println("Allowance targets are ready.")
		return
	}
	if !hasRetryableMissingOrFailedTarget(allowances.Targets) {
		fmt.Println("No retryable missing or failed targets were returned.")
		return
	}
	if skipRetry {
		fmt.Println("Skipping retry because LIMITLESS_SKIP_ALLOWANCE_RETRY is enabled.")
		return
	}

	fmt.Printf("POST /profiles/partner-accounts/%d/allowances/retry\n", profileID)
	retried, err := sdk.PartnerAccounts.RetryAllowances(ctx, profileID)
	if err != nil {
		handleRetryError(err)
		return
	}
	printAllowanceResponse(retried)

	if submittedTargets(retried.Targets) > 0 {
		fmt.Println("Retry submitted sponsored allowance work. Poll the GET endpoint again after a short delay.")
	}
}

func hasRetryableMissingOrFailedTarget(targets []limitless.PartnerAccountAllowanceTarget) bool {
	for _, target := range targets {
		if !target.Retryable {
			continue
		}
		if target.Status == limitless.PartnerAccountAllowanceStatusMissing ||
			target.Status == limitless.PartnerAccountAllowanceStatusFailed {
			return true
		}
	}
	return false
}

func submittedTargets(targets []limitless.PartnerAccountAllowanceTarget) int {
	count := 0
	for _, target := range targets {
		if target.Status == limitless.PartnerAccountAllowanceStatusSubmitted {
			count++
		}
	}
	return count
}

func handleRetryError(err error) {
	var rateLimitErr *limitless.RateLimitError
	if errors.As(err, &rateLimitErr) {
		fmt.Printf(
			"Retry is rate limited. retryAfterSeconds=%s\n",
			retryAfterSeconds(rateLimitErr.Data),
		)
		log.Fatalf("Allowance retry failed: %v", err)
	}

	var conflictErr *limitless.ConflictError
	if errors.As(err, &conflictErr) {
		fmt.Println("Another allowance retry is already running. Wait briefly and poll the GET endpoint again.")
		log.Fatalf("Allowance retry failed: %v", err)
	}

	log.Fatalf("Allowance retry failed: %v", err)
}

func retryAfterSeconds(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "(not provided)"
	}

	var body struct {
		RetryAfterSeconds *int `json:"retryAfterSeconds"`
	}
	if err := json.Unmarshal(raw, &body); err != nil || body.RetryAfterSeconds == nil {
		return "(not provided)"
	}
	return strconv.Itoa(*body.RetryAfterSeconds)
}

func printAllowanceResponse(resp *limitless.PartnerAccountAllowanceResponse) {
	fmt.Printf(
		"profileId=%d partnerProfileId=%d chainId=%d wallet=%s ready=%t\n",
		resp.ProfileID,
		resp.PartnerProfileID,
		resp.ChainID,
		resp.WalletAddress,
		resp.Ready,
	)
	fmt.Printf(
		"summary: total=%d confirmed=%d missing=%d submitted=%d failed=%d\n",
		resp.Summary.Total,
		resp.Summary.Confirmed,
		resp.Summary.Missing,
		resp.Summary.Submitted,
		resp.Summary.Failed,
	)

	for i, target := range resp.Targets {
		fmt.Printf(
			"target[%d]: type=%s label=%s requiredFor=%s status=%s confirmed=%t retryable=%t spenderOrOperator=%s",
			i,
			target.Type,
			target.Label,
			target.RequiredFor,
			target.Status,
			target.Confirmed,
			target.Retryable,
			target.SpenderOrOperator,
		)
		if target.TransactionID != nil {
			fmt.Printf(" transactionId=%s", *target.TransactionID)
		}
		if target.TxHash != nil {
			fmt.Printf(" txHash=%s", *target.TxHash)
		}
		if target.UserOperationHash != nil {
			fmt.Printf(" userOperationHash=%s", *target.UserOperationHash)
		}
		if target.ErrorCode != nil {
			fmt.Printf(" errorCode=%s", *target.ErrorCode)
		}
		if target.ErrorMessage != nil {
			fmt.Printf(" errorMessage=%q", *target.ErrorMessage)
		}
		fmt.Println()
	}
}
