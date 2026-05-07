package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

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

func optionalEnvWithFallback(key, fallback string) string {
	value := optionalEnv(key)
	if value == "" {
		return fallback
	}
	return value
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

func optionalPositiveInt(key string) (int, bool) {
	value := optionalEnv(key)
	if value == "" {
		return 0, false
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		log.Fatalf("%s must be a positive integer", key)
	}
	return parsed, true
}

func boolPtr(value bool) *bool {
	return &value
}

func main() {
	ctx := context.Background()
	identityToken := requireEnv("LIMITLESS_IDENTITY_TOKEN")
	marketSlug := requireEnv("MARKET_SLUG")
	skipWithdraw := envFlag("LIMITLESS_SKIP_WITHDRAW", true)

	bootstrap := limitless.NewClient(
		limitless.WithLogger(limitless.NewConsoleLogger(limitless.LogLevelInfo)),
	)

	requestedScopes := []string{
		limitless.ScopeTrading,
		limitless.ScopeDelegatedSigning,
		limitless.ScopeAccountCreation,
		limitless.ScopeWithdrawal,
	}

	capabilities, err := bootstrap.ApiTokens.GetCapabilities(ctx, identityToken)
	if err != nil {
		log.Fatalf("Failed to fetch capabilities: %v", err)
	}
	fmt.Printf(
		"Capabilities: enabled=%t allowedScopes=%v\n",
		capabilities.TokenManagementEnabled,
		capabilities.AllowedScopes,
	)

	derived, err := bootstrap.ApiTokens.DeriveToken(ctx, identityToken, limitless.DeriveApiTokenInput{
		Label:  fmt.Sprintf("go-sdk-server-wallet-%d", time.Now().Unix()),
		Scopes: requestedScopes,
	})
	if err != nil {
		log.Fatalf("Failed to derive scoped token: %v", err)
	}
	fmt.Printf(
		"Derived token: tokenId=%s profileId=%d scopes=%v\n",
		derived.TokenID,
		derived.Profile.ID,
		derived.Scopes,
	)

	scoped := limitless.NewClient(
		limitless.WithHMACCredentials(limitless.HMACCredentials{
			TokenID: derived.TokenID,
			Secret:  derived.Secret,
		}),
		limitless.WithLogger(limitless.NewConsoleLogger(limitless.LogLevelInfo)),
	)

	market, err := bootstrap.Markets.GetMarket(ctx, marketSlug)
	if err != nil {
		log.Fatalf("Failed to fetch market: %v", err)
	}
	if market.ConditionID == nil || strings.TrimSpace(*market.ConditionID) == "" {
		log.Fatalf("Market %s does not expose conditionId", marketSlug)
	}

	onBehalfOf, hasExistingProfile := optionalPositiveInt("LIMITLESS_ON_BEHALF_OF")
	account := optionalEnv("LIMITLESS_SERVER_WALLET_ACCOUNT")
	createdAccount := false

	if hasExistingProfile {
		if account == "" {
			account = "(not provided)"
		}
		fmt.Println("Using existing server-wallet child account from env.")
	} else {
		partnerAccount, err := scoped.PartnerAccounts.CreateAccount(ctx, limitless.CreatePartnerAccountInput{
			DisplayName:        optionalEnvWithFallback("PARTNER_ACCOUNT_DISPLAY_NAME", "Go SDK Server Wallet"),
			CreateServerWallet: boolPtr(true),
		}, nil)
		if err != nil {
			log.Fatalf("Failed to create partner account: %v", err)
		}
		onBehalfOf = partnerAccount.ProfileID
		account = partnerAccount.Account
		createdAccount = true
	}

	fmt.Printf(
		"Server-wallet target: onBehalfOf=%d account=%s conditionId=%s\n",
		onBehalfOf,
		account,
		*market.ConditionID,
	)

	if createdAccount {
		fmt.Println("Redeem is usually most useful on an existing traded child profile. Set LIMITLESS_ON_BEHALF_OF to reuse one.")
	}

	redeem, err := scoped.ServerWallets.RedeemPositions(ctx, limitless.RedeemServerWalletParams{
		ConditionID: *market.ConditionID,
		OnBehalfOf:  onBehalfOf,
	})
	if err != nil {
		log.Fatalf("Failed to redeem server-wallet positions: %v", err)
	}
	fmt.Printf(
		"Redeem submitted: transactionId=%s userOperationHash=%s wallet=%s\n",
		redeem.TransactionID,
		redeem.UserOperationHash,
		redeem.WalletAddress,
	)

	if skipWithdraw {
		fmt.Println("Skipping withdraw because LIMITLESS_SKIP_WITHDRAW is enabled. Set LIMITLESS_SKIP_WITHDRAW=0 to run the withdraw step.")
		return
	}

	amount := requireEnv("LIMITLESS_WITHDRAW_AMOUNT")
	destination := optionalEnv("LIMITLESS_WITHDRAW_DESTINATION")
	allowlistDestination := envFlag("LIMITLESS_ALLOWLIST_WITHDRAW_DESTINATION", false)
	destinationLabel := optionalEnvWithFallback("LIMITLESS_WITHDRAW_DESTINATION_LABEL", "treasury")
	token := optionalEnv("LIMITLESS_WITHDRAW_TOKEN")

	fmt.Printf(
		"Withdrawing amount=%s token=%s destination=%s\n",
		amount,
		emptyFallback(token, "(default token)"),
		emptyFallback(destination, "(default: authenticated smart wallet when present, otherwise account)"),
	)

	if destination != "" && allowlistDestination {
		fmt.Printf("Allowlisting withdraw destination=%s label=%s\n", destination, destinationLabel)
		withdrawalAddress, err := bootstrap.PartnerAccounts.AddWithdrawalAddress(ctx, identityToken, limitless.PartnerWithdrawalAddressInput{
			Address: destination,
			Label:   destinationLabel,
		})
		if err != nil {
			log.Fatalf("Failed to allowlist withdraw destination: %v", err)
		}
		fmt.Printf(
			"Withdrawal destination allowlisted: id=%s profileId=%d destination=%s label=%s\n",
			withdrawalAddress.ID,
			withdrawalAddress.ProfileID,
			withdrawalAddress.DestinationAddress,
			withdrawalAddress.Label,
		)
	}

	withdraw, err := scoped.ServerWallets.Withdraw(ctx, limitless.WithdrawServerWalletParams{
		Amount:      amount,
		OnBehalfOf:  onBehalfOf,
		Token:       token,
		Destination: destination,
	})
	if err != nil {
		log.Fatalf("Failed to withdraw from server wallet: %v", err)
	}
	fmt.Printf(
		"Withdraw submitted: transactionId=%s userOperationHash=%s destination=%s\n",
		withdraw.TransactionID,
		withdraw.UserOperationHash,
		withdraw.Destination,
	)
}

func emptyFallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
