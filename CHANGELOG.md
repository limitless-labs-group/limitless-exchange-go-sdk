# Changelog

All notable changes to the Limitless Exchange Go SDK will be documented in this file.

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
- Base mainnet (chain ID 8453) and Base Sepolia testnet (84532) support
- Zero external dependencies beyond `go-ethereum` (signing) and `gorilla/websocket`
