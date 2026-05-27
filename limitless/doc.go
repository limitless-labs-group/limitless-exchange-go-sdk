// Package limitless provides a Go SDK for the Limitless Exchange API.
//
// The package supports:
//   - market and orderbook reads for CLOB and NegRisk markets
//   - signed order creation for GTC, FAK, and FOK orders with optional receive-window controls
//   - delegated order placement and partner-account workflows
//   - API token management, portfolio reads, market pages, and WebSocket streams
//
// Typical entrypoints are NewClient for shared HTTP-backed services and
// NewOrderClient for signed order placement with EIP-712 signatures.
package limitless
