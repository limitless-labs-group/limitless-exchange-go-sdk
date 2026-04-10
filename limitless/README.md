# limitless

`limitless` is the main package for the Limitless Exchange Go SDK.

It provides:

- public market and orderbook reads
- signed CLOB order placement for `GTC`, `FAK`, and `FOK`
- delegated order creation for partner/server-wallet flows
- portfolio, market-page, API-token, and partner-account APIs
- WebSocket clients for real-time streams

## Installation

```bash
go get github.com/limitless-labs-group/limitless-exchange-go-sdk@v1.0.5
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

Full module documentation, release notes, and setup guidance are in the repository root [`README.md`](https://github.com/limitless-labs-group/limitless-exchange-go-sdk/blob/main/README.md).
