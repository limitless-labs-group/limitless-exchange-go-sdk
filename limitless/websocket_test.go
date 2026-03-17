package limitless

import (
	"context"
	"encoding/json"
	"testing"
)

func TestWebSocketClient_Subscribe_AuthValidation(t *testing.T) {
	t.Parallel()

	ws := NewWebSocketClient(WithWebSocketAPIKey(""), WithAutoReconnect(false))
	ws.mu.Lock()
	ws.state = StateConnected
	ws.sio = &socketIOClient{}
	ws.mu.Unlock()

	err := ws.Subscribe(context.Background(), ChannelSubscribePositions, SubscriptionOptions{})
	if err == nil {
		t.Fatal("expected auth validation error for subscribe_positions without API key, got nil")
	}
}

func TestWebSocketClient_OnOnceOffDispatchLocal(t *testing.T) {
	t.Parallel()

	ws := NewWebSocketClient(WithAutoReconnect(false))

	count := 0
	id := ws.On("connect", func(data json.RawMessage) {
		count++
	})
	ws.dispatchLocal("connect", nil)
	if count != 1 {
		t.Fatalf("expected count=1 after first dispatch, got %d", count)
	}

	ws.Off("connect", id)
	ws.dispatchLocal("connect", nil)
	if count != 1 {
		t.Fatalf("expected handler removed, count should remain 1, got %d", count)
	}

	onceCount := 0
	ws.Once("reconnecting", func(data json.RawMessage) {
		onceCount++
	})
	ws.dispatchLocal("reconnecting", nil)
	ws.dispatchLocal("reconnecting", nil)
	if onceCount != 1 {
		t.Fatalf("expected once handler to fire once, got %d", onceCount)
	}
}

func TestWebSocketClient_OnOrderbookUpdate_Parsing(t *testing.T) {
	t.Parallel()

	ws := NewWebSocketClient()

	var received OrderbookUpdate
	called := false
	ws.OnOrderbookUpdate(func(update OrderbookUpdate) {
		called = true
		received = update
	})

	ws.dispatchLocal("orderbookUpdate", json.RawMessage(`{
		"marketSlug":"btc",
		"orderbook":{
			"bids":[{"price":0.51,"size":100,"side":"buy"}],
			"asks":[{"price":0.52,"size":120,"side":"sell"}],
			"tokenId":"123",
			"adjustedMidpoint":0.515,
			"maxSpread":0.05,
			"minSize":1
		},
		"timestamp":"2026-03-17T00:00:00.000Z"
	}`))

	if !called {
		t.Fatal("expected OnOrderbookUpdate handler to be called")
	}
	if received.MarketSlug != "btc" {
		t.Fatalf("expected marketSlug btc, got %s", received.MarketSlug)
	}
	if len(received.Orderbook.Bids) != 1 || len(received.Orderbook.Asks) != 1 {
		t.Fatalf("expected one bid/ask, got bids=%d asks=%d", len(received.Orderbook.Bids), len(received.Orderbook.Asks))
	}
}

func TestSocketIOClient_HandleSocketIOPacket_EventDispatch(t *testing.T) {
	t.Parallel()

	sio := &socketIOClient{
		namespace: "/markets",
		handlers:  map[string][]handlerEntry{},
		logger:    NewNoOpLogger(),
	}

	called := false
	sio.On("orderbookUpdate", func(data json.RawMessage) {
		called = true
	})

	sio.handleSocketIOPacket(`2/markets,["orderbookUpdate",{"marketSlug":"btc"}]`)
	if !called {
		t.Fatal("expected socket.io event to be dispatched to handler")
	}
}

func TestWebSocketClient_SubscriptionKeyHelpers(t *testing.T) {
	t.Parallel()

	ws := NewWebSocketClient()
	key := ws.subscriptionKey(ChannelSubscribeMarketPrices, SubscriptionOptions{MarketSlug: "btc"})
	if key != "subscribe_market_prices:btc" {
		t.Fatalf("unexpected subscription key: %s", key)
	}

	globalKey := ws.subscriptionKey(ChannelSubscribeMarketPrices, SubscriptionOptions{})
	if globalKey != "subscribe_market_prices:global" {
		t.Fatalf("unexpected global subscription key: %s", globalKey)
	}

	channel := ws.channelFromKey("subscribe_positions:btc")
	if channel != ChannelSubscribePositions {
		t.Fatalf("expected subscribe_positions channel, got %s", channel)
	}
}
