# Changelog

All notable changes to the Limitless Exchange Go SDK will be documented in this file.

## [1.1.0]

### Added

- Typed handlers for the two structured `orderEvent` frames, both delivered on
  the shared `orderEvent` socket.io event and discriminated on `type`:
  - `OnMatchedOrderEvent(func(MatchedOrderEvent))` for pre-settlement per-fill
    `MATCHED` frames (`source: "SETTLEMENT"`). Maker side now reports `0` fee.
  - `OnExecutionOrderEvent(func(ExecutionOrderEvent))` for FAK/FOK terminal
    `EXECUTION` frames (`source: "OME"`, `status` `FILLED`/`PARTIALLY_FILLED`/`KILLED`).
- Public payload types `MatchedOrderEvent` and `ExecutionOrderEvent`.
- `OrderResponse.Execution` (`*OrderExecution`, optional) so consumers can read
  the taker-delay outcome and the settlement/fee summary from the POST /orders
  response. Includes `SettlementStatus` (plain string, known values
  `UNMATCHED`/`MATCHED`/`MINED`/`CONFIRMED`/`RETRYING`/`FAILED`/`DELAYED`),
  the `EligibleAt` taker-delay release timestamp, and the `OrderExecutionTotalsRaw`
  raw integer-string totals. Additive and non-breaking; previously the SDK
  dropped the response `execution` object.
- Partner server-wallet split/merge helpers:
  - `ServerWalletService.SplitPositions` for `POST /portfolio/split`
  - `ServerWalletService.MergePositions` for `POST /portfolio/merge`
  - typed split/merge request and response models using required `conditionId`, `amount`, required `venue.exchange` when no adapter is provided, required `venue.adapter` for NegRisk routing, and optional `onBehalfOf` request fields.
  - raw split/merge API response preservation via `RawJSON()` for backend field verification.
- `AMMService` with typed allowance check/approve/ensure workflows and AMM buy/sell requests for direct or partner-owned server wallets.
- HMAC and Privy identity variants for AMM endpoints, strict integer-string/uint256 validation, optional transaction identifiers, and typed `422`, `425`, and `502`/`503` errors.
- Opt-in raw HTTP responses (status, headers, and the original body) via `...WithRawResponse` methods returning a generic `RawResult[T]{ Data T; Raw *RawResponse }`, added across all API-backed service methods (Markets, Market Pages, Portfolio, API tokens, Partner accounts, Delegated orders, Server wallets, orders, and AMM). Base methods are unchanged and delegate to the raw variant. Raw mode still returns the same typed errors for `>= 400` statuses.
- Atomic cancel-replace for orders via `OrderClient.CancelReplace` / `CancelReplaceBatch` and `DelegatedOrderService.CancelReplace` / `CancelReplaceBatch`, backed by `POST /orders/cancel-replace` and `/orders/cancel-replace/batch`. A single request cancels a resting order and submits its replacement atomically; the batch variant does so for many operations at once. The single-order variant lets the `409` conflict status through as a typed `CancelReplaceResult`. Typed `CancelReplaceParams`, `CancelReplaceRequest`, `CancelReplaceResult`, and `CancelReplaceBatchResponse` models.

These additions are non-breaking: the raw `OnOrderEvent(func(OrderEvent))`
handler and the `OrderEvent = json.RawMessage` alias are unchanged, so existing
raw consumers keep working.

### Changed

- Retry handling now distinguishes retryable HTTP-client timeouts from cancellation or expiry of the caller's context.
- **BREAKING (type):** `OrderBook.LastTradePrice` is now `*float64` (was `float64`). The API sends `lastTradePrice: null` for markets with no trades yet, which previously decoded to a silent `0.0` — indistinguishable from a real zero price. Callers must nil-check; `nil` means "no trade yet."
- README, package documentation, and SDK tracking version now target `v1.1.0`.

## [1.0.11]

### Added

- Optional receive-window controls for normal and delegated order creation:
  - `ReceiveWindowOptions.Timestamp`
  - `ReceiveWindowOptions.RecvWindow`, serialized as top-level `recvWindow`
- Unit coverage for omitted defaults, top-level-only payloads, automatic timestamp stamping, and invalid receive-window values before network calls.

### Changed

- `CreateOrderParams` and `CreateDelegatedOrderParams` now accept optional `ReceiveWindow` controls while preserving unchanged payloads when omitted.
- README, package documentation, and SDK tracking version now target `v1.0.11`.

## [1.0.10]

### Added

- `PortfolioFetcher.GetCurrentProfile` for `GET /profiles/me`, fetching the authenticated caller's private profile without passing an address.
- `PartnerAccountService.ListAccounts` for `GET /profiles/partner-accounts`, including optional address recovery and page/limit query params capped at 25.
- Public partner account list types:
  - `ListPartnerAccountsParams`
  - `PartnerAccountListItem`
  - `ListPartnerAccountsResponse`
- `HttpClient.DeleteWithIdentity` and `RetryableClient.DeleteWithIdentity` for identity-token authenticated DELETE requests.
- Unit coverage for `/profiles/me` profile reads and HMAC-only partner account listing, filtering, pagination capping, and invalid query params.

### Changed

- README, package documentation, and SDK tracking version now target `v1.0.10`.

## [1.0.9]

### Added

- WebSocket subscription/event surface for order events, live sports/esports, market lifecycle, oracle price data, and system messages.
- Partner withdrawal destination allowlist helpers:
  - `PartnerAccountService.AddWithdrawalAddress`
  - `PartnerAccountService.DeleteWithdrawalAddress`
  - typed `PartnerWithdrawalAddressInput` and `PartnerWithdrawalAddressResponse` models
- `ServerWalletService.Withdraw` support for destination-only own-wallet withdrawals.
- Documentation for whitelisted server-wallet withdraw destinations and omitted-destination smart-wallet fallback.

### Changed

- Removed unsupported legacy websocket short channel constants and stale typed handlers/types for unsupported events.
- Updated WebSocket docs/examples to use `subscribe_market_prices` + `orderbookUpdate` for CLOB orderbooks and `subscribe_order_events` + `orderEvent` for authenticated order lifecycle/settlement updates.
- README, package documentation, and SDK tracking version now target `v1.0.9`.

## [1.0.8] - 2026-04-30

### Added

- Partner server-wallet allowance recovery helpers:
  - `PartnerAccountService.CheckAllowances`
  - `PartnerAccountService.RetryAllowances`
  - typed allowance summary, target, status, and error-code response models
- New runnable `examples/partner_account_allowances` flow for partner HMAC allowance check and retry operations without admin APIs.

### Changed

- Updated partner allowance recovery models and docs for live-chain retry behavior:
  - target `submitted` status now means the current retry request submitted a sponsored transaction or user operation
  - target-level `IN_FLIGHT_ELSEWHERE`, `RATE_LIMITED`, and `nextRetryAt` modeling was removed
  - success response `retryAfterSeconds` / `nextRetryAt` modeling was removed; `429` retry timing remains available from the raw API error body
  - retry `429` is handled as `RateLimitError`; retry `409` is handled as `ConflictError`
- README, package documentation, and SDK tracking version now target `v1.0.8`.

## [1.0.7]

### Changed

- Migrated portfolio history endpoint from legacy page/limit pagination to cursor-based pagination.
  - `GetUserHistory()` now accepts `cursor string` instead of `page int`.
  - First request should pass empty string for cursor; subsequent requests pass the returned `NextCursor`.
  - Default limit changed from 10 to 20 to match API default.
- Updated `HistoryEntry` struct to match current API response shape (`BlockTimestamp`, `Strategy`, `TransactionHash`, `Market`, etc.).
- Replaced `HistoryResponse.TotalCount` with `NextCursor *string` for cursor-based pagination.
- Added `HistoryMarket` and `HistoryMarketCollateral` structs.

## [1.0.6] - 2026-04-14

### Added

- New server-wallet partner surface for delegated-signing child accounts:
  - `Client.ServerWallets`
  - `ServerWalletService`
  - `RedeemServerWalletParams` / `RedeemServerWalletResponse`
  - `WithdrawServerWalletParams` / `WithdrawServerWalletResponse`
- New `ScopeWithdrawal` constant for api-token derivation flows that need `/portfolio/withdraw`.
- New focused tests covering server-wallet validation, HMAC-only auth enforcement, and root client composition.
- New runnable `examples/server_wallet_redeem_withdraw` flow plus README/package-doc coverage for server-wallet redeem and optional withdraw operations.

## [1.0.5] - 2026-04-09

### Added

- `FAK` (Fill-And-Kill) limit-order support alongside existing `GTC` and `FOK` flows:
  - `OrderTypeFAK`
  - `FAKOrderArgs`
  - shared limit-order amount construction and validation for `GTC` + `FAK`
- New public examples and README coverage for:
  - `FAK` limit-order placement
  - `GTC` `postOnly` usage

### Changed

- `postOnly` is now documented and demonstrated as a `GTC`-only flag.
- README installation and release metadata now target `v1.0.5`.

## [1.0.4] - 2026-04-02

### Added

- API token and partner-account surface for the new `apiToken` feature set:
  - `Client.ApiTokens`
  - `Client.PartnerAccounts`
  - `Client.DelegatedOrders`
- New HMAC authentication support for scoped API tokens:
  - `WithHMACCredentials(...)`
  - request signing with `lmts-api-key`, `lmts-timestamp`, and `lmts-signature`
  - WebSocket handshake support for HMAC-authenticated subscriptions
- New HTTP helpers needed by the api-token flows:
  - `Patch(...)`
  - `PostWithHeaders(...)`
  - `PostWithIdentity(...)`
  - `GetWithIdentity(...)`
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
  - HMAC signing and transport auth precedence
  - Privy identity-header overrides
  - api-token self-service endpoints
  - partner account creation flows
  - delegated order submission
  - market-page redirect/property/filter flows
  - offline signing and fallback verifying contract behavior
  - websocket reconnect and API-key rotation with preserved subscriptions
  - retryable transport errors
  - root client service wiring

### Changed

- Authenticated SDK operations now accept either legacy API-key auth or new HMAC api-token auth through the shared HTTP client.
- The public Go SDK no longer exposes `admin/*` api-token endpoints; internal admin mutation flows remain only in the integration project via bare HTTP helpers.
- README and examples now prefer explicit configuration and the root `Client` API over the older fetcher-by-fetcher setup.
- Authenticated SDK operations now fail locally with clear messages when no API key is configured, instead of relying on opaque server-side authentication failures.
- Retry behavior now includes retryable transport/network failures in addition to retryable HTTP status codes.
- URL construction now consistently uses escaped path segments and structured query encoding.
- Validation behavior is aligned with order-building behavior so exported validators and builder logic accept/reject the same payloads.
- README now clarifies that partner HMAC credentials are intended for backend/BFF usage; browser apps should keep public reads in the frontend and route partner-authenticated actions through their own backend.

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
- Fixed authenticated surface mismatches for the api-token feature:
  - Privy-only bootstrap endpoints use `identity: Bearer <token>` instead of the default client auth mode
  - root `Client.NewWebSocketClient()` now carries HMAC credentials into the WS client
  - delegated partner order placement is modeled as a separate `DelegatedOrderService`, avoiding hidden coupling to local private keys and `/profiles/:account`
- Fixed HTTP error typing to expose `ConflictError` for 409 responses used by partner-account and client-order-id flows.
- Fixed partner-account creation payload and validation:
  - `CreatePartnerAccountInput` now sends `displayName` instead of the rejected legacy `label` field
  - `CreateAccount(...)` now validates the backend 44-character `displayName` limit before sending the request
  - partner-account tests now cover server-wallet mode, duplicate-address `409` conflicts, and EOA self-address rejection
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
- Public channels: CLOB orderbook updates, AMM/oracle price data, live sports/esports, market lifecycle
- Authenticated channels: positions, transactions, order lifecycle
- Typed event handlers (`OnOrderbookUpdate`, `OnNewPriceData`, `OnOraclePriceData`, `OnOrderEvent`, `OnTransaction`, `OnMarketCreated`, `OnMarketResolved`)
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
