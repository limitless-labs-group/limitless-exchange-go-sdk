# Changelog

All notable changes to the Limitless Exchange Go SDK will be documented in this file.

## [Unreleased]

### Added

- New root `Client` API that wires shared domain services:
  - `Client.Markets`
  - `Client.Portfolio`
  - `Client.Pages`
  - `Client.NewOrderClient(...)`
  - `Client.NewWebSocketClient(...)`
- New explicit offline signing helpers on `OrderClient`:
  - `SignOrderForMarket(...)`
  - `SignOrderWithConfig(...)`
- New tests covering:
  - market-page redirect/property/filter flows
  - offline signing and fallback verifying contract behavior
  - websocket reconnect and API-key rotation with preserved subscriptions
  - retryable transport errors
  - root client service wiring

### Changed

- README and examples now prefer explicit configuration and the root `Client` API over the older fetcher-by-fetcher setup.
- Authenticated SDK operations now fail locally with clear messages when no API key is configured, instead of relying on opaque server-side authentication failures.
- Retry behavior now includes retryable transport/network failures in addition to retryable HTTP status codes.
- URL construction now consistently uses escaped path segments and structured query encoding.
- Validation behavior is aligned with order-building behavior so exported validators and builder logic accept/reject the same payloads.

### Fixed

- Fixed standalone order signing semantics:
  - `CreateOrder(...)` continues to sign with `market.venue.exchange` when available
  - fallback `ContractAddress` is only used when venue exchange data is absent
  - plain `SignOrder(...)` no longer silently signs with an invalid placeholder verifying contract
- Fixed WebSocket subscription bookkeeping:
  - subscription identity now includes the full normalized subscription options rather than only `MarketSlug`
  - reconnect and unsubscribe now operate on the correct subscription entries
  - `SetAPIKey(...)` reconnects without dropping saved subscriptions
  - handler removal remains consistent across reconnects
- Fixed authenticated WebSocket channel checks to cover authenticated order/fill channels in addition to positions/transactions.
- Fixed order-validation drift:
  - FOK amount precision validation now matches builder behavior
  - GTC price tick alignment and size-step validation now match builder behavior
- Fixed HTTP path/query handling for market, portfolio, orders, and market-pages endpoints by escaping IDs/slugs and consistently using encoded query parameters.
- Fixed SDK formatting drift; package files are now `gofmt`-clean.

### Deprecated

- Constructor-based environment fallbacks remain supported in `v1.x` for backward compatibility:
  - `LIMITLESS_API_KEY` for `NewHttpClient()` / `NewWebSocketClient()`
  - `CHAIN_ID` for `NewOrderClient()`
- Prefer explicit configuration via `WithAPIKey(...)` and `WithSigningConfig(...)`.
- These environment fallbacks are scheduled for removal in `v1.0.5`.

## [1.0.3] - 2026-03-23

### Added

#### HTTP Client

- Configurable HTTP client with connection pooling and timeouts
- API key authentication (via option or `LIMITLESS_API_KEY` environment variable)
- Automatic SDK version and user-agent headers (`x-sdk-version`, `user-agent`)
- Raw response support with status codes and headers (`GetRaw`)
- Request/response body logging at Debug level for troubleshooting

#### Market Data

- Fetch single market by slug with venue caching for order signing
- Fetch active markets with pagination and sorting
- Fetch orderbook (bids/asks with adjusted midpoint)
- Fetch user orders for a market (authenticated)
- Fluent API on `Market` struct (`market.GetUserOrders(ctx)`)

#### Market Pages & Navigation

- Navigation tree discovery (`GetNavigation`)
- Market page resolution by path with automatic redirect handling
- Market page markets with filtering, pagination, and cursor support
- Property keys and options for market filtering

#### Order Management

- EIP-712 order signing with private key (Base mainnet & Sepolia)
- GTC (Good-Til-Cancelled) limit orders — buy and sell
- FOK (Fill-or-Kill) market orders — buy and sell
- Automatic user profile fetching for fee rate calculation
- Order cancellation — single order and cancel-all by market
- Build unsigned orders and sign separately for advanced workflows

#### Portfolio

- User profile fetching by wallet address
- Portfolio positions (CLOB and AMM)
- User history with pagination

#### WebSocket (Real-time)

- Socket.IO protocol over WebSocket with Engine.IO ping/pong
- Public channels: orderbook updates, trades, market updates, price data
- Authenticated channels: positions, transactions, orders, fills
- Typed event handlers (`OnOrderbookUpdate`, `OnTrade`, `OnOrder`, `OnFill`, `OnTransaction`, `OnMarket`)
- Generic event handler (`On`, `Once`, `Off`) with handler ID management
- Auto-reconnect with exponential backoff and jitter
- Automatic re-subscription on reconnect
- Connection state management

#### Retry & Error Handling

- Configurable retry with exponential backoff
- Retryable HTTP client wrapper
- Typed errors: `APIError`, `RateLimitError`, `AuthenticationError`, `ValidationError`
- Retry on 429, 500, 502, 503, 504 by default

#### Logging

- Pluggable `Logger` interface (Debug, Info, Warn, Error)
- Built-in `ConsoleLogger` with configurable log level and timestamped output
- Built-in `NoOpLogger` for silent operation

#### Architecture

- Functional options pattern throughout (`WithBaseURL`, `WithAPIKey`, `WithLogger`, etc.)
- Go 1.22+ with generics support (`WithRetry[T]`)
- Base mainnet (chain ID 8453)
- Zero external dependencies beyond `go-ethereum` (signing) and `gorilla/websocket`
