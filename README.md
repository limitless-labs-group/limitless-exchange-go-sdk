# Limitless Exchange Go SDK

**v1.1.0** | Production-Ready | Type-Safe | Fully Documented

A Go SDK for interacting with the Limitless Exchange platform, providing access to CLOB and NegRisk prediction markets.

> **v1.1.0 Release**: Adds typed `MATCHED` and `EXECUTION` `orderEvent` handlers (`OnMatchedOrderEvent`, `OnExecutionOrderEvent`); non-breaking. See [CHANGELOG.md](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/blob/main/CHANGELOG.md) for release notes.

## Disclaimer

**USE AT YOUR OWN RISK**

This SDK is provided "as-is" without any warranties or guarantees. Trading on prediction markets involves financial risk. By using this SDK, you acknowledge that:

- You are responsible for testing the SDK thoroughly before using it in production
- The SDK authors are not liable for any financial losses or damages
- You should review and understand the code before executing any trades
- It is recommended to test all functionality on testnet or with small amounts first

**ALWAYS TEST BEFORE USING IN PRODUCTION WITH REAL FUNDS**

**Feedback Welcome**: We encourage you to report any bugs, suggest improvements, or contribute to the project. Please submit issues or pull requests on our GitHub repository.

## Geographic Restrictions

**Important**: Limitless restricts order placement from US locations due to regulatory requirements and compliance with international sanctions. Before placing orders, builders should verify their location complies with applicable regulations.

## Features

- **Authentication**: Legacy API-key auth and new scoped API-key auth
- **Order Management**: Create, cancel, and manage `GTC`, `FAK`, and `FOK` orders on CLOB and NegRisk markets, including `postOnly` for `GTC` and optional receive-window freshness checks
- **Scoped API Keys**: Self-service key listing/capabilities, partner-account creation, delegated order placement, server-wallet redeem/withdraw
- **AMM Partner Trading**: Server-wallet BUY/SELL allowance setup and idempotent AMM buy/sell submission
- **Market Data**: Access real-time market data and orderbooks
- **NegRisk Markets**: Full support for group markets with multiple outcomes
- **Error Handling & Retry**: Automatic retry logic for rate limits, transient HTTP failures, and retryable transport errors
- **WebSocket**: Real-time CLOB orderbook, AMM/oracle price, position, transaction, order-event, and market lifecycle streaming
- **Market Pages & Navigation**: Navigation tree, path-based page resolution, property filters
- **Portfolio**: Position tracking, user history, and profile access
- **Raw Responses**: Opt-in `...WithRawResponse` variant on every API-backed method exposing HTTP status, headers, and the original body
- **Logging**: Pluggable logger interface with built-in console logger

## Installation

```bash
go get github.com/limitless-labs-group/limitless-exchange-go-sdk@v1.1.0
```

Requires Go 1.24 or later.

Package documentation:

- Module overview: `https://pkg.go.dev/github.com/limitless-labs-group/limitless-exchange-go-sdk`
- Public SDK API: `https://pkg.go.dev/github.com/limitless-labs-group/limitless-exchange-go-sdk/limitless`

## Quick Start

### Fetching Market Data (No Authentication Required)

```go
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

    // Fetch a single market
    market, err := sdk.Markets.GetMarket(ctx, "your-market-slug")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Market: %s\n", market.Title)

    // Fetch orderbook
    ob, err := sdk.Markets.GetOrderBook(ctx, market.Slug)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Midpoint: %.4f | Bids: %d | Asks: %d\n",
        ob.AdjustedMidpoint, len(ob.Bids), len(ob.Asks))

    // Fetch active markets with sorting and pagination
    resp, err := sdk.Markets.GetActiveMarkets(ctx, &limitless.ActiveMarketsParams{
        Limit:  10,
        SortBy: limitless.SortByNewest,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Total markets: %d\n", resp.TotalMarketsCount)
}
```

### Authentication

The SDK supports two authenticated modes:

- Legacy API keys via `X-API-Key`
- New scoped API keys via HMAC headers (`lmts-api-key`, `lmts-timestamp`, `lmts-signature`)

```go
// Legacy API key
sdk := limitless.NewClient(
    limitless.WithAPIKey(os.Getenv("LIMITLESS_API_KEY")),
)

// New scoped API key
sdk := limitless.NewClient(
    limitless.WithHMACCredentials(limitless.HMACCredentials{
        TokenID: os.Getenv("LIMITLESS_API_TOKEN_ID"),
        Secret:  os.Getenv("LIMITLESS_API_TOKEN_SECRET"),
    }),
)
```

Environment auto-loading: `NewHttpClient()` and `NewWebSocketClient()` can read `LIMITLESS_API_KEY` from the environment when no explicit API key is provided. `WithAPIKey(...)` remains fully supported and is the clearest way to configure API-key auth in application code.

Use partner HMAC credentials only in a backend or BFF service. Do not expose `LIMITLESS_API_TOKEN_ID` / `LIMITLESS_API_TOKEN_SECRET` in browser code or client-side storage.

Recommended setup:

- Keep public market and market-page reads in the browser.
- Store the real HMAC credentials on your backend.
- Use this SDK server-side to sign partner-authenticated requests.
- Expose only your own app-specific endpoints to the frontend.

**Environment Variables:**

Create a `.env` file:

```bash
# Required for authenticated endpoints
LIMITLESS_API_KEY=your_api_key_here

# Optional: new scoped API-key auth
LIMITLESS_API_TOKEN_ID=your_api_token_id
LIMITLESS_API_TOKEN_SECRET=your_api_token_secret

# Optional: Privy bootstrap for self-service partner capabilities / derive token
LIMITLESS_IDENTITY_TOKEN=your_privy_identity_token

# Required for order signing (EIP-712)
PRIVATE_KEY=your_private_key_hex

# Market to trade
MARKET_SLUG=your-market-slug

# Required for delegated order examples
LIMITLESS_PARTNER_PROFILE_ID=12345
LIMITLESS_TARGET_FEE_RATE_BPS=300

# Optional for server-wallet redeem / withdraw example
LIMITLESS_SKIP_WITHDRAW=1
LIMITLESS_WITHDRAW_AMOUNT=
LIMITLESS_WITHDRAW_DESTINATION=
LIMITLESS_ALLOWLIST_WITHDRAW_DESTINATION=0
LIMITLESS_WITHDRAW_DESTINATION_LABEL=treasury
LIMITLESS_WITHDRAW_TOKEN=
LIMITLESS_ON_BEHALF_OF=
LIMITLESS_SERVER_WALLET_ACCOUNT=
```

### Token Approvals

**Important**: Before placing orders, you must approve tokens for the exchange contracts. This is a **one-time setup** per wallet.

**CLOB Markets:**
- **BUY orders**: Approve USDC for `market.Venue.Exchange`
- **SELL orders**: Approve Conditional Tokens for `market.Venue.Exchange`

**NegRisk Markets:**
- **BUY orders**: Approve USDC for `market.Venue.Exchange`
- **SELL orders**: Approve Conditional Tokens for **both** `market.Venue.Exchange` AND `market.Venue.Adapter`

### Placing a GTC (Limit) Order

```go
sdk := limitless.NewClient(
    limitless.WithAPIKey(os.Getenv("LIMITLESS_API_KEY")),
)

ctx := context.Background()

// Fetch market for token IDs and venue
market, _ := sdk.Markets.GetMarket(ctx, "your-market-slug")

// Create order client with private key
orderClient, _ := sdk.NewOrderClient(os.Getenv("PRIVATE_KEY"))

// Place a GTC BUY order at 0.50 for 10 shares
// PostOnly is supported only for GTC orders.
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
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Order created: %s\n", resp.Order.ID)
```

### Optional Receive Window

Order creation can opt into API receive-window freshness checks with `ReceiveWindowOptions`. These values are sent as top-level `POST /orders` fields named `timestamp` and `recvWindow`; they are not included in the signed EIP-712 order payload.

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
```

If omitted, no receive-window fields are sent. `RecvWindow` must be between `1` and `10000` milliseconds. When `RecvWindow` is supplied without `Timestamp`, the SDK stamps the current Unix time in milliseconds. Keep trading hosts NTP-synced; server clock skew tolerance is about one second. Do not retry a `425 Too Early` with the same payload; build a fresh order instead.

### FAK Orders (Fill-and-Kill Limit Orders)

FAK orders use the same `Price` + `Size` construction as `GTC`, but any unmatched remainder is cancelled immediately instead of resting on the orderbook.

**Parameter Semantics:**
- **BUY**: `Price` = max price to pay, `Size` = shares to buy
- **SELL**: `Price` = min price to accept, `Size` = shares to sell
- `PostOnly` is **not** supported for `FAK`

```go
// BUY FAK - fill up to 10 shares at 0.45 and kill the remainder
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

if len(resp.MakerMatches) > 0 {
    fmt.Printf("FAK matched immediately with %d fill(s)\n", len(resp.MakerMatches))
} else {
    fmt.Println("FAK did not match and the remainder was cancelled.")
}
```

### FOK Orders (Fill-or-Kill Market Orders)

FOK orders execute immediately at the best available price or cancel entirely.

**Parameter Semantics:**
- **BUY**: `MakerAmount` = total USDC to spend
- **SELL**: `MakerAmount` = number of shares to sell

```go
// BUY FOK - spend 50 USDC at market price
resp, err := orderClient.CreateOrder(ctx, limitless.CreateOrderParams{
    OrderType:  limitless.OrderTypeFOK,
    MarketSlug: market.Slug,
    Args: limitless.FOKOrderArgs{
        TokenID:     market.Tokens.Yes,
        Side:        limitless.SideBuy,
        MakerAmount: 50.0,
    },
})

// SELL FOK - sell 120 shares at market price
resp, err := orderClient.CreateOrder(ctx, limitless.CreateOrderParams{
    OrderType:  limitless.OrderTypeFOK,
    MarketSlug: market.Slug,
    Args: limitless.FOKOrderArgs{
        TokenID:     market.Tokens.Yes,
        Side:        limitless.SideSell,
        MakerAmount: 120.0,
    },
})
```

### Cancel Orders

```go
// Cancel a single order
msg, err := orderClient.Cancel(ctx, "order-id")

// Cancel all orders for a market
msg, err := orderClient.CancelAll(ctx, "market-slug")
```

### Cancel-Replace Orders

Atomically cancel a resting order and submit its replacement in a single request via `POST /orders/cancel-replace`. Target the order to cancel with `CancelByOrderID` or `CancelByClientOrderID`, and set `Mode` to `CancelReplaceStopOnFailure` (skip the replacement if the cancel fails) or `CancelReplaceAllowFailure`.

```go
result, err := orderClient.CancelReplace(ctx, limitless.CancelReplaceParams{
    Cancel: limitless.CancelByOrderID("old-order-id"), // or CancelByClientOrderID("...")
    Mode:   limitless.CancelReplaceStopOnFailure,
    Replacement: limitless.CancelReplaceOrderParams{
        MarketSlug: "market-slug",
        OrderType:  limitless.OrderTypeGTC,
        Args:       limitless.GTCOrderArgs{TokenID: "123", Side: limitless.SideBuy, Price: 0.5, Size: 2},
    },
})
// result.Cancel and result.Replacement each carry a per-leg status.
```

Replace many orders at once with `CancelReplaceBatch(ctx, []limitless.CancelReplaceParams{...})`; the response `Results` are index-aligned to the input. Partner integrations use `sdk.DelegatedOrders.CancelReplace` / `CancelReplaceBatch` with `limitless.DelegatedCancelReplaceParams` (adds `OnBehalfOf`). The single-order variant treats a `409` conflict as a decodable result rather than an error.

### WebSocket (Real-Time Streaming)

```go
sdk := limitless.NewClient(
    limitless.WithHMACCredentials(limitless.HMACCredentials{
        TokenID: os.Getenv("LIMITLESS_API_TOKEN_ID"),
        Secret:  os.Getenv("LIMITLESS_API_TOKEN_SECRET"),
    }),
)
ws := sdk.NewWebSocketClient(
    limitless.WithAutoReconnect(true),
)

// Register typed event handlers
ws.OnOrderbookUpdate(func(update limitless.OrderbookUpdate) {
    fmt.Printf("Orderbook: midpoint=%.4f bids=%d asks=%d\n",
        update.Orderbook.AdjustedMidpoint,
        len(update.Orderbook.Bids),
        len(update.Orderbook.Asks))
})

ws.OnOrderEvent(func(event limitless.OrderEvent) {
    fmt.Printf("Order event: %s\n", string(event))
})

// Connect and subscribe
ws.Connect(context.Background())
defer ws.Disconnect()

ws.Subscribe(ctx, limitless.ChannelSubscribeMarketPrices, limitless.SubscriptionOptions{
    MarketSlugs: []string{"your-market-slug"},
})
```

### Scoped API Keys, Delegated Orders, & Server Wallets

```go
sdk := limitless.NewClient(
    limitless.WithHMACCredentials(limitless.HMACCredentials{
        TokenID: os.Getenv("LIMITLESS_API_TOKEN_ID"),
        Secret:  os.Getenv("LIMITLESS_API_TOKEN_SECRET"),
    }),
)

// List active scoped API keys
tokens, _ := sdk.ApiTokens.ListTokens(ctx)

// Fetch partner capabilities with a Privy identity token
capabilities, _ := sdk.ApiTokens.GetCapabilities(ctx, os.Getenv("LIMITLESS_IDENTITY_TOKEN"))

// List partner-owned sub-accounts. Requires HMAC auth with account_creation scope.
accounts, _ := sdk.PartnerAccounts.ListAccounts(ctx, limitless.ListPartnerAccountsParams{
    Limit: 25,
    Page:  1,
})

// Recover a specific partner-owned account by address.
account, _ := sdk.PartnerAccounts.ListAccounts(ctx, limitless.ListPartnerAccountsParams{
    Account: "0xChildAccount",
    Limit:   25,
    Page:    1,
})

// Add or remove partner withdrawal destinations with a Privy access token.
treasuryAddress := "0xTreasuryAddress"
withdrawalAddress, _ := sdk.PartnerAccounts.AddWithdrawalAddress(ctx, os.Getenv("LIMITLESS_PRIVY_ACCESS_TOKEN"), limitless.PartnerWithdrawalAddressInput{
    Address: treasuryAddress,
    Label:   "treasury",
})

// Place an order signed by the API on behalf of a partner-managed profile
resp, _ := sdk.DelegatedOrders.CreateOrder(ctx, limitless.CreateDelegatedOrderParams{
    MarketSlug: "your-market-slug",
    OrderType:  limitless.OrderTypeGTC,
    OnBehalfOf: 12345,
    FeeRateBps: 300,
    Args: limitless.GTCOrderArgs{
        TokenID: "token-id",
        Side:    limitless.SideBuy,
        Price:   0.500,
        Size:    10.0,
    },
})

// Check live delegated-trading allowance state for a server-wallet child profile
allowances, _ := sdk.PartnerAccounts.CheckAllowances(ctx, 12345)
if allowances != nil && !allowances.Ready {
    // Retry re-checks live chain state and submits only targets still missing.
    // A returned "submitted" status means this request submitted a sponsored tx/user operation.
    allowances, _ = sdk.PartnerAccounts.RetryAllowances(ctx, 12345)
}

// Redeem resolved positions for a server-managed child profile
redeem, _ := sdk.ServerWallets.RedeemPositions(ctx, limitless.RedeemServerWalletParams{
    ConditionID: "0x...",
    OnBehalfOf:  12345,
})

// Split and merge complete sets for a CLOB or NegRisk market.
// CLOB/simple calls require venue.exchange. NegRisk calls require venue.adapter
// and may also forward venue.exchange when present.
market, _ := sdk.Markets.GetMarket(ctx, "your-market-slug")
conditionID := *market.ConditionID
venue := &limitless.ServerWalletVenue{Exchange: market.Venue.Exchange}
if market.NegRiskRequestID != nil && market.Venue.Adapter != nil {
    venue.Adapter = *market.Venue.Adapter
}

split, _ := sdk.ServerWallets.SplitPositions(ctx, limitless.SplitServerWalletParams{
    ConditionID: conditionID,
    Amount:      "1000000",
    Venue:       venue,
    OnBehalfOf:  12345,
})

merge, _ := sdk.ServerWallets.MergePositions(ctx, limitless.MergeServerWalletParams{
    ConditionID: conditionID,
    Amount:      "1000000",
    Venue:       venue,
    OnBehalfOf:  12345,
})

// Withdraw funds from the same child profile
withdraw, _ := sdk.ServerWallets.Withdraw(ctx, limitless.WithdrawServerWalletParams{
    Amount:      "1000000",
    OnBehalfOf:  12345,
    Destination: "0xReceiverAddress",
})

// Withdraw the authenticated caller's own server wallet to an explicit allowed destination.
ownWalletWithdraw, _ := sdk.ServerWallets.Withdraw(ctx, limitless.WithdrawServerWalletParams{
    Amount:      "1000000",
    Destination: treasuryAddress,
})

// Optional cleanup when the destination should no longer be active.
_ = sdk.PartnerAccounts.DeleteWithdrawalAddress(ctx, os.Getenv("LIMITLESS_PRIVY_ACCESS_TOKEN"), treasuryAddress)

_, _, _, _, _, _, _, _, _, _ = resp, allowances, withdrawalAddress, redeem, split, merge, withdraw, ownWalletWithdraw, accounts, account
```

Use `PartnerAccounts.CheckAllowances`, `PartnerAccounts.RetryAllowances`, and `ServerWallets` only for child profiles created with `CreateServerWallet=true`. Derive the scoped token with `limitless.ScopeTrading` for redeem, split, and merge calls; add `limitless.ScopeAccountCreation` only when creating child profiles and `limitless.ScopeWithdrawal` for withdraw flows. `limitless.ScopeDelegatedSigning` is needed for delegated order signing, not split/merge.

Withdrawal destination allowlist management is Privy-only. Use `PartnerAccounts.AddWithdrawalAddress` and `PartnerAccounts.DeleteWithdrawalAddress` with the partner operator's Privy access token. Scoped API-token withdrawal requests can then target the authenticated partner account address, authenticated partner smart wallet, or an active allowlisted destination. If `Destination` is omitted, the API defaults to the authenticated partner's smart wallet when present; otherwise it defaults to the authenticated partner account. Leave `OnBehalfOf` as zero only when withdrawing the authenticated caller's own server wallet to an explicit `Destination`.

For allowance recovery, poll `CheckAllowances` first. If `Ready` is false and one or more targets are `missing` or `failed` with `Retryable=true`, call `RetryAllowances`, then poll `CheckAllowances` again after a short delay. A `429` is returned as `*limitless.RateLimitError`; its API body includes `retryAfterSeconds` in `err.Data`. A `409` is returned as `*limitless.ConflictError` and means another retry is already running, so wait briefly and check status again.

### AMM Partner Trading

AMM trading uses the core API host and requires either an HMAC token with both `trading` and `delegated_signing` scopes or a Privy identity token. Legacy API keys are rejected. Set up the independent `BUY` and `SELL` approvals once for each server-wallet/market pair; `Buy` and `Sell` intentionally do not perform an allowance preflight.

```go
childProfileID := 12345
marketSlug := "your-amm-market-slug"

// EnsureAllowance checks first, submits approval at most once, then polls check.
// The context controls how long to wait for on-chain confirmation.
for _, side := range []limitless.AMMAllowanceSide{
    limitless.AMMAllowanceSideBuy,
    limitless.AMMAllowanceSideSell,
} {
    allowanceCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
    _, err := sdk.AMM.EnsureAllowance(allowanceCtx, limitless.AMMAllowanceParams{
        Market:     marketSlug,
        Side:       side,
        OnBehalfOf: childProfileID,
    })
    cancel()
    if err != nil {
        return err
    }
}

slippageBps := 100
buyParams := limitless.AMMBuyParams{
    Market:           marketSlug,
    OutcomeIndex:     limitless.AMMOutcomeYes,
    CollateralAmount: "1000000", // collateral base units; never use float64
    SlippageBps:      &slippageBps,
    IdempotencyKey:   "buy-unique-key-001",
    OnBehalfOf:       childProfileID,
}

// On a timeout or transient failure, reuse this immutable params value so the
// serialized body and idempotency key remain identical.
buy, err := limitless.WithRetry(ctx, func() (*limitless.AMMBuyResponse, error) {
    return sdk.AMM.Buy(ctx, buyParams)
}, limitless.DefaultRetryConfig())
```

`ApproveAllowance` may return `status: "submitted"` before the approval is confirmed; poll `CheckAllowance`, or use `EnsureAllowance`, instead of approving repeatedly. The four AMM endpoints share a 10-request-per-10-second rate limit, so the helper polls every two seconds by default. Amounts must be positive uint256 integer strings in collateral base units. `SlippageBps` may be `nil` for the API default or point to a value from `0` through `1000`; transaction identifiers in responses are independently optional.

For Privy auth, use the corresponding `CheckAllowanceWithIdentity`, `ApproveAllowanceWithIdentity`, `EnsureAllowanceWithIdentity`, `BuyWithIdentity`, and `SellWithIdentity` methods. HTTP `409`, `422`, `425`, `429`, and `502`/`503` responses map to `ConflictError`, `UnprocessableEntityError`, `TooEarlyError`, `RateLimitError`, and `UpstreamUnavailableError`, respectively.

**Available Channels:**

| Channel | Auth Required | Description |
|---------|:---:|-------------|
| `ChannelSubscribeMarketPrices` | No | CLOB `orderbookUpdate`, AMM `newPriceData`, oracle `oraclePriceData` |
| `ChannelSubscribeLiveSports` | No | Live sports snapshots |
| `ChannelSubscribeLiveEsports` | No | Live esports snapshots |
| `ChannelSubscribeMarketLifecycle` | No | `marketCreated` and `marketResolved` lifecycle events |
| `ChannelSubscribePositions` | Yes | Position updates |
| `ChannelSubscribeTransactions` | Yes | Transaction events |
| `ChannelSubscribeOrderEvents` | Yes | Order lifecycle and settlement events on `orderEvent` |

### Portfolio & Profile

```go
sdk := limitless.NewClient(
    limitless.WithAPIKey(os.Getenv("LIMITLESS_API_KEY")),
)

// Fetch user profile
profile, _ := sdk.Portfolio.GetProfile(ctx, "0xYourWalletAddress")

// Fetch the authenticated caller's private profile
currentProfile, _ := sdk.Portfolio.GetCurrentProfile(ctx)

// Fetch positions
positions, _ := sdk.Portfolio.GetPositions(ctx)
fmt.Printf("CLOB: %d positions | AMM: %d positions\n",
    len(positions.CLOB), len(positions.AMM))

// Fetch user history
history, _ := sdk.Portfolio.GetUserHistory(ctx, "", 20)

_, _ = profile, currentProfile
```

### User Orders

```go
// Prefer the service API
orders, _ := sdk.Markets.GetUserOrders(ctx, "market-slug")

// Legacy fluent API remains available for backward compatibility
market, _ := sdk.Markets.GetMarket(ctx, "market-slug")
orders, _ := market.GetUserOrders(ctx)
```

### Market Pages & Navigation

```go
sdk := limitless.NewClient(
    limitless.WithAPIKey(os.Getenv("LIMITLESS_API_KEY")),
)

// Get navigation tree
nav, _ := sdk.Pages.GetNavigation(ctx)

// Resolve a page by URL path (handles 301 redirects)
page, _ := sdk.Pages.GetMarketPageByPath(ctx, "/crypto")

// Fetch markets for a page with filters
markets, _ := sdk.Pages.GetMarkets(ctx, page.ID, &limitless.MarketPageMarketsParams{
    Limit: intPtr(20),
    Sort:  "-updatedAt",
})

// Browse filter options
keys, _ := sdk.Pages.GetPropertyKeys(ctx)
options, _ := sdk.Pages.GetPropertyOptions(ctx, keys[0].ID, nil)
```

### Error Handling & Retry

The SDK provides typed errors and automatic retry logic:

```go
// Typed error handling
resp, err := orderClient.CreateOrder(ctx, params)
if err != nil {
    var apiErr *limitless.APIError
    if errors.As(err, &apiErr) {
        fmt.Printf("API error %d: %s\n", apiErr.Status, apiErr.Message)
    }

    var rateLimitErr *limitless.RateLimitError
    if errors.As(err, &rateLimitErr) {
        fmt.Println("Rate limited, try again later")
    }
}

// Automatic retry with exponential backoff
retryClient := limitless.NewRetryableClient(client, limitless.RetryConfig{
    StatusCodes:     []int{429, 500, 502, 503, 504},
    MaxRetries:      3,
    ExponentialBase: 2.0,
    MaxDelay:        60 * time.Second,
})

// Or use the generic retry wrapper
result, err := limitless.WithRetry(ctx, func() (*Response, error) {
    return orderClient.CreateOrder(ctx, params)
}, limitless.DefaultRetryConfig())
```

### Logging

```go
// Console logger with configurable level
logger := limitless.NewConsoleLogger(limitless.LogLevelDebug)

sdk := limitless.NewClient(
    limitless.WithLogger(logger),
)

// Output: 2026-03-23 12:44:03 [Limitless SDK] INFO  Market fetched successfully map[slug:my-market]
```

Available log levels: `LogLevelDebug`, `LogLevelInfo`, `LogLevelWarn`, `LogLevelError`.

Implement the `Logger` interface to integrate with your own logging framework:

```go
type Logger interface {
    Debug(msg string, meta ...map[string]any)
    Info(msg string, meta ...map[string]any)
    Warn(msg string, meta ...map[string]any)
    Error(msg string, err error, meta ...map[string]any)
}
```

## Raw Responses

Every API-backed service method has a `...WithRawResponse` sibling that returns
the decoded value alongside the underlying HTTP response — status code, headers,
and the original body. The base method is unchanged and delegates to the raw
variant, keeping the common path allocation-free while making status/header
inspection opt-in.

```go
result, err := sdk.AMM.BuyWithRawResponse(ctx, buyParams)
if err != nil {
    log.Fatal(err)
}

fmt.Println(result.Raw.Status)                      // 201 for a submitted buy
fmt.Println(result.Raw.Headers.Get("X-Request-Id")) // response headers
fmt.Println(result.Data.Status)                     // decoded AMMBuyResponse
```

`RawResult[T]` wraps the decoded value and the raw response:

```go
type RawResult[T any] struct {
    Data T
    Raw  *RawResponse // Status int, Headers http.Header, Body json.RawMessage
}
```

Raw mode preserves typed error handling: any `>= 400` status still returns the
same typed error (`ConflictError`, `UnprocessableEntityError`, `RateLimitError`,
and so on) rather than a `RawResult`. It is available across `Markets`, `Pages`,
`Portfolio`, `ApiTokens`, `PartnerAccounts`, `DelegatedOrders`, `ServerWallets`,
the order client, and `AMM`.

## Examples

Runnable examples are available in the [`examples/`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/tree/main/examples) directory:

| Example | Description | Auth Required |
|---------|-------------|:---:|
| [`active_markets`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/tree/main/examples/active_markets) | Fetch active markets with pagination and sorting | No |
| [`clob_gtc_order`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/tree/main/examples/clob_gtc_order) | Place a GTC (limit) order with `postOnly` | Yes |
| [`clob_fak_order`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/tree/main/examples/clob_fak_order) | Place a FAK (fill-and-kill) limit order | Yes |
| [`clob_fok_order`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/tree/main/examples/clob_fok_order) | Place a FOK (market) order | Yes |
| [`negrisk_order`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/tree/main/examples/negrisk_order) | Trade on NegRisk group markets | Yes |
| [`portfolio`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/tree/main/examples/portfolio) | Fetch profile, positions, and history | Yes |
| [`api_tokens`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/tree/main/examples/api_tokens) | List scoped API keys and fetch capabilities | Scoped API key / Privy |
| [`delegated_order`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/tree/main/examples/delegated_order) | Place a delegated partner order | Scoped API key |
| [`delegated_fok_order`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/tree/main/examples/delegated_fok_order) | Place a delegated FOK partner order | Scoped API key |
| [`partner_account_allowances`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/tree/main/examples/partner_account_allowances) | Check and retry server-wallet allowance recovery with partner HMAC auth only | Scoped API key |
| [`server_wallet_redeem_withdraw`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/tree/main/examples/server_wallet_redeem_withdraw) | Reuse or create a server-wallet child profile, redeem resolved positions, and optionally withdraw funds | Scoped API key / Privy |
| [`e2e_fok_flow`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/tree/main/examples/e2e_fok_flow) | End-to-end partner delegated FOK flow without cleanup | Scoped API key / Privy |
| [`websocket_orderbook`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/tree/main/examples/websocket_orderbook) | Stream live orderbook updates | No |
| [`websocket_positions`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/tree/main/examples/websocket_positions) | Stream position and transaction updates | Yes |

Run any example:

```bash
cd examples/active_markets
go run .

# For authenticated examples, set environment variables first:
export LIMITLESS_API_KEY=your_key
export PRIVATE_KEY=your_private_key
export MARKET_SLUG=your-market-slug

# Partner HMAC examples need:
export LIMITLESS_API_TOKEN_ID=your_api_token_id
export LIMITLESS_API_TOKEN_SECRET=your_api_token_secret
export LIMITLESS_PARTNER_ACCOUNT_PROFILE_ID=12345

# Bootstrap/server-wallet examples that derive or create tokens also need:
export LIMITLESS_IDENTITY_TOKEN=your_privy_identity_token
export LIMITLESS_SKIP_WITHDRAW=1

cd examples/clob_gtc_order
go run .
```

## Project Structure

```
limitless/
├── client.go              # HTTP client with connection pooling
├── client_options.go      # Functional options for HttpClient
├── constants.go           # Chain IDs, contract addresses, URLs
├── amm.go                 # AMMService (allowances, buy, sell)
├── amm_types.go           # AMM request and response types
├── errors.go              # Typed errors (APIError, RateLimitError, etc.)
├── logger.go              # Logger interface and ConsoleLogger
├── markets.go             # MarketFetcher (market data, orderbook, user orders)
├── markets_types.go       # Market, OrderBook, Venue types
├── market_pages.go        # MarketPageFetcher (navigation, pages, filters)
├── market_pages_types.go  # NavigationNode, MarketPage, PropertyKey types
├── orders.go              # OrderClient (create, cancel, sign)
├── orders_builder.go      # Order amount calculation and tick validation
├── orders_signer.go       # EIP-712 signing
├── orders_types.go        # Order, GTCOrderArgs, FAKOrderArgs, FOKOrderArgs types
├── orders_validator.go    # Input validation
├── portfolio.go           # PortfolioFetcher (profile, positions, history)
├── portfolio_types.go     # UserProfile, CLOBPosition, AMMPosition types
├── server_wallets.go      # ServerWalletService (redeem and withdraw)
├── partner_accounts.go    # PartnerAccountService (account creation and allowance recovery)
├── server_wallets_types.go # Server-wallet request and response types
├── retry.go               # Retry logic with exponential backoff
├── websocket.go           # WebSocketClient with typed event handlers
├── websocket_socketio.go  # Socket.IO/Engine.IO v4 protocol implementation
└── websocket_types.go     # WebSocket event and subscription types

examples/
├── active_markets/        # Fetch and display active markets
├── clob_gtc_order/        # Place GTC limit orders (with postOnly)
├── clob_fak_order/        # Place FAK limit orders
├── clob_fok_order/        # Place FOK market orders
├── negrisk_order/         # NegRisk group market trading
├── portfolio/             # Profile and position fetching
├── api_tokens/            # Scoped API-key listing and capability lookup
├── delegated_order/       # Delegated partner order placement
├── delegated_fok_order/   # Delegated FOK partner order placement
├── partner_account_allowances/ # Partner allowance check and retry
├── server_wallet_redeem_withdraw/ # Server-wallet redeem and optional withdraw
├── e2e_fok_flow/          # End-to-end partner delegated FOK flow
├── websocket_orderbook/   # Live orderbook streaming
└── websocket_positions/   # Position and transaction streaming

internal/
└── mathutil/              # Decimal precision utilities
```

## Dependencies

The SDK has minimal external dependencies:

- [`go-ethereum`](https://github.com/ethereum/go-ethereum) — EIP-712 signing and Ethereum utilities
- [`gorilla/websocket`](https://github.com/gorilla/websocket) — WebSocket transport

## Changelog

See [CHANGELOG.md](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/blob/main/CHANGELOG.md) for release notes.

## License

MIT - see [LICENSE](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/blob/main/LICENSE)
