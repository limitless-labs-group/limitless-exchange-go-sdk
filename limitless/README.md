# limitless

`limitless` is the main package for the Limitless Exchange Go SDK.

It provides:

- public market and orderbook reads
- signed CLOB order placement for `GTC`, `FAK`, and `FOK`
- delegated order creation plus server-wallet redeem/withdraw for partner flows
- portfolio, market-page, API-token, and partner-account APIs
- WebSocket clients for real-time streams

## Installation

```bash
go get github.com/limitless-labs-group/limitless-exchange-go-sdk@v1.1.0
```

## Import

```go
import "github.com/limitless-labs-group/limitless-exchange-go-sdk/limitless"
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/limitless-labs-group/limitless-exchange-go-sdk/limitless"
)

func main() {
	ctx := context.Background()

	sdk := limitless.NewClient(
		limitless.WithAPIKey(os.Getenv("LIMITLESS_API_KEY")),
	)

	market, err := sdk.Markets.GetMarket(ctx, os.Getenv("MARKET_SLUG"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Market: %s\n", market.Title)
}
```

## Authentication

The package supports two authenticated modes:

- legacy API key auth via `X-API-Key`
- scoped API key auth via HMAC headers

```go
sdk := limitless.NewClient(
	limitless.WithAPIKey(os.Getenv("LIMITLESS_API_KEY")),
)
```

```go
sdk := limitless.NewClient(
	limitless.WithHMACCredentials(limitless.HMACCredentials{
		TokenID: os.Getenv("LIMITLESS_API_TOKEN_ID"),
		Secret:  os.Getenv("LIMITLESS_API_TOKEN_SECRET"),
	}),
)
```

## Partner HMAC Flows

Use `PartnerAccounts.CheckAllowances`, `PartnerAccounts.RetryAllowances`, and `ServerWallets` only for child profiles created with `CreateServerWallet=true`.

```go
sdk := limitless.NewClient(
	limitless.WithHMACCredentials(limitless.HMACCredentials{
		TokenID: os.Getenv("LIMITLESS_API_TOKEN_ID"),
		Secret:  os.Getenv("LIMITLESS_API_TOKEN_SECRET"),
	}),
)

allowances, _ := sdk.PartnerAccounts.CheckAllowances(ctx, 12345)
if allowances != nil && !allowances.Ready {
	// Retry submits only targets that are still missing after a live chain re-check.
	allowances, _ = sdk.PartnerAccounts.RetryAllowances(ctx, 12345)
}

treasuryAddress := "0xTreasuryAddress"
withdrawalAddress, _ := sdk.PartnerAccounts.AddWithdrawalAddress(ctx, os.Getenv("LIMITLESS_IDENTITY_TOKEN"), limitless.PartnerWithdrawalAddressInput{
	Address: treasuryAddress,
	Label:   "treasury",
})

redeem, _ := sdk.ServerWallets.RedeemPositions(ctx, limitless.RedeemServerWalletParams{
	ConditionID: "0x...",
	OnBehalfOf:  12345,
})

withdraw, _ := sdk.ServerWallets.Withdraw(ctx, limitless.WithdrawServerWalletParams{
	Amount:      "1000000",
	OnBehalfOf:  12345,
	Destination: "0xReceiverAddress",
})

ownWalletWithdraw, _ := sdk.ServerWallets.Withdraw(ctx, limitless.WithdrawServerWalletParams{
	Amount:      "1000000",
	Destination: treasuryAddress,
})

_ = sdk.PartnerAccounts.DeleteWithdrawalAddress(ctx, os.Getenv("LIMITLESS_IDENTITY_TOKEN"), treasuryAddress)

_, _, _, _, _ = allowances, withdrawalAddress, redeem, withdraw, ownWalletWithdraw
```

Derive the scoped token with `limitless.ScopeAccountCreation` and `limitless.ScopeDelegatedSigning`; add `limitless.ScopeWithdrawal` for withdraw flows.

Withdrawal destination allowlist management is Privy-only. Use `PartnerAccounts.AddWithdrawalAddress` and `PartnerAccounts.DeleteWithdrawalAddress` with the partner operator's Privy identity token. Scoped API-token withdrawal requests can then target the authenticated partner account address, authenticated partner smart wallet, or an active allowlisted destination. If `Destination` is omitted, the API defaults to the authenticated partner's smart wallet when present; otherwise it defaults to the authenticated partner account. Leave `OnBehalfOf` as zero only when withdrawing the authenticated caller's own server wallet to an explicit `Destination`.

Allowance checks are always based on live chain reads. A retry response with submitted targets means that retry request submitted a sponsored transaction or user operation; poll `CheckAllowances` again after a short delay to observe confirmed chain state. `RetryAllowances` returns `*limitless.RateLimitError` for `429` responses, with `retryAfterSeconds` in the raw API body, and `*limitless.ConflictError` for `409` responses when another retry is already running.

## Orders

Use `NewOrderClient` to create signed orders.

### GTC

`GTC` uses `Price` and `Size` and may include `PostOnly`.

```go
orderClient, _ := sdk.NewOrderClient(os.Getenv("PRIVATE_KEY"))

resp, err := orderClient.CreateOrder(ctx, limitless.CreateOrderParams{
	OrderType:  limitless.OrderTypeGTC,
	MarketSlug: market.Slug,
	Args: limitless.GTCOrderArgs{
		TokenID:  market.Tokens.Yes,
		Side:     limitless.SideBuy,
		Price:    0.50,
		Size:     10.0,
		PostOnly: true,
	},
})

_ = resp
_ = err
```

### Optional Receive Window

Pass `ReceiveWindowOptions` to opt into order freshness checks. `Timestamp` and `RecvWindow` serialize as top-level `POST /orders` fields and are not part of the EIP-712 signed order.

```go
recvWindow := int64(1500)

resp, err := orderClient.CreateOrder(ctx, limitless.CreateOrderParams{
	OrderType:  limitless.OrderTypeGTC,
	MarketSlug: market.Slug,
	Args: limitless.GTCOrderArgs{
		TokenID: market.Tokens.Yes,
		Side:    limitless.SideBuy,
		Price:   0.50,
		Size:    10.0,
	},
	ReceiveWindow: limitless.ReceiveWindowOptions{
		RecvWindow: &recvWindow,
	},
})

_ = resp
_ = err
```

If omitted, both fields stay omitted. `RecvWindow` must be `1..10000` milliseconds. When `RecvWindow` is supplied without `Timestamp`, the SDK stamps the current Unix time in milliseconds. Keep trading hosts NTP-synced and build a fresh order instead of retrying the same payload after `425 Too Early`.

### FAK

`FAK` uses the same `Price` and `Size` semantics as `GTC`, but any unmatched remainder is cancelled immediately. `PostOnly` is not supported for `FAK`.

```go
resp, err := orderClient.CreateOrder(ctx, limitless.CreateOrderParams{
	OrderType:  limitless.OrderTypeFAK,
	MarketSlug: market.Slug,
	Args: limitless.FAKOrderArgs{
		TokenID: market.Tokens.Yes,
		Side:    limitless.SideBuy,
		Price:   0.45,
		Size:    10.0,
	},
})

_ = resp
_ = err
```

### FOK

`FOK` uses `MakerAmount`. For buys, that is the USDC amount to spend. For sells, that is the share quantity to sell.

```go
resp, err := orderClient.CreateOrder(ctx, limitless.CreateOrderParams{
	OrderType:  limitless.OrderTypeFOK,
	MarketSlug: market.Slug,
	Args: limitless.FOKOrderArgs{
		TokenID:     market.Tokens.Yes,
		Side:        limitless.SideBuy,
		MakerAmount: 50.0,
	},
})

_ = resp
_ = err
```

## Examples

Repository examples:

- [`examples/clob_gtc_order/main.go`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/blob/main/examples/clob_gtc_order/main.go)
- [`examples/clob_fak_order/main.go`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/blob/main/examples/clob_fak_order/main.go)
- [`examples/clob_fok_order/main.go`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/blob/main/examples/clob_fok_order/main.go)
- [`examples/delegated_order/main.go`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/blob/main/examples/delegated_order/main.go)
- [`examples/delegated_fok_order/main.go`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/blob/main/examples/delegated_fok_order/main.go)
- [`examples/partner_account_allowances/main.go`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/blob/main/examples/partner_account_allowances/main.go)
- [`examples/server_wallet_redeem_withdraw/main.go`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/blob/main/examples/server_wallet_redeem_withdraw/main.go)

Full module documentation, release notes, and setup guidance are in the repository root [`README.md`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/blob/main/README.md).
