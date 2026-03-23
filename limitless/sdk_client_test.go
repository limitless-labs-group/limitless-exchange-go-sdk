package limitless

import "testing"

func TestNewClient_WiresSharedServices(t *testing.T) {
	t.Parallel()

	client := NewClient(WithAPIKey("api-key"))
	if client.HTTP == nil || client.Markets == nil || client.Portfolio == nil || client.Pages == nil {
		t.Fatalf("expected client services to be initialized, got %+v", client)
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
}
