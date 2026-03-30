package limitless

import "testing"

func TestNewClient_WiresSharedServices(t *testing.T) {
	t.Parallel()

	client := NewClient(
		WithAPIKey("api-key"),
		WithHMACCredentials(HMACCredentials{
			TokenID: "token-1",
			Secret:  "secret",
		}),
	)
	if client.HTTP == nil || client.Markets == nil || client.Portfolio == nil || client.Pages == nil {
		t.Fatalf("expected client services to be initialized, got %+v", client)
	}
	if client.ApiTokens == nil || client.PartnerAccounts == nil || client.DelegatedOrders == nil {
		t.Fatalf("expected new client services to be initialized, got %+v", client)
	}

	orderClient, err := client.NewOrderClient(testPrivateKeyHex)
	if err != nil {
		t.Fatalf("NewOrderClient returned error: %v", err)
	}
	if orderClient.marketFetcher != client.Markets {
		t.Fatal("expected order client to reuse shared MarketFetcher")
	}
	if orderClient.portfolioFetcher != client.Portfolio {
		t.Fatal("expected order client to reuse shared PortfolioFetcher")
	}

	ws := client.NewWebSocketClient()
	if ws.config.APIKey != "api-key" {
		t.Fatalf("expected websocket client to inherit API key, got %q", ws.config.APIKey)
	}
	if ws.config.HMACCreds == nil || ws.config.HMACCreds.TokenID != "token-1" {
		t.Fatalf("expected websocket client to inherit HMAC credentials, got %+v", ws.config.HMACCreds)
	}
}
