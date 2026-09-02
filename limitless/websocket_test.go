package limitless

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

	err = ws.Subscribe(context.Background(), ChannelSubscribeOrderEvents, SubscriptionOptions{})
	if err == nil {
		t.Fatal("expected auth validation error for subscribe_order_events without API key, got nil")
	}
}

func TestWebSocketClient_Subscribe_AuthValidation_AllowsHMAC(t *testing.T) {
	t.Parallel()

	ws := NewWebSocketClient(
		WithWebSocketHMACCredentials(HMACCredentials{TokenID: "token-1", Secret: "secret"}),
		WithAutoReconnect(false),
	)
	ws.mu.Lock()
	ws.state = StateConnected
	ws.sio = &socketIOClient{}
	ws.mu.Unlock()

	if err := ws.Subscribe(context.Background(), ChannelSubscribePositions, SubscriptionOptions{}); err != nil {
		t.Fatalf("expected HMAC-authenticated subscribe to succeed, got %v", err)
	}

	if err := ws.Subscribe(context.Background(), ChannelSubscribeOrderEvents, SubscriptionOptions{}); err != nil {
		t.Fatalf("expected HMAC-authenticated order-event subscribe to succeed, got %v", err)
	}
}

func TestWebSocketClient_Subscribe_RejectsUnknownChannel(t *testing.T) {
	t.Parallel()

	ws := NewWebSocketClient(WithWebSocketAPIKey("test-key"), WithAutoReconnect(false))
	ws.mu.Lock()
	ws.state = StateConnected
	ws.sio = &socketIOClient{}
	ws.mu.Unlock()

	err := ws.Subscribe(context.Background(), SubscriptionChannel("unknown"), SubscriptionOptions{})
	if err == nil {
		t.Fatal("expected unsupported channel error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported websocket subscription channel") {
		t.Fatalf("expected unsupported channel error, got %q", err.Error())
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
			"midpoint":0.51,
			"maxSpread":0.05,
			"minSize":1
		},
		"version":48213,
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
	if received.Version != 48213 {
		t.Fatalf("expected version 48213, got %d", received.Version)
	}
	if received.Orderbook.Midpoint != 0.51 {
		t.Fatalf("expected midpoint 0.51, got %f", received.Orderbook.Midpoint)
	}
}

func TestWebSocketClient_OnOrderbookUpdate_ParsingStringEncodedScalars(t *testing.T) {
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
			"maxSpread":"0.05",
			"minSize":"100000000"
		},
		"timestamp":"2026-03-17T00:00:00.000Z"
	}`))

	if !called {
		t.Fatal("expected OnOrderbookUpdate handler to be called")
	}
	if received.Orderbook.MaxSpread.Float64() != 0.05 {
		t.Fatalf("expected maxSpread 0.05, got %f", received.Orderbook.MaxSpread.Float64())
	}
	if received.Orderbook.MinSize.Float64() != 100000000 {
		t.Fatalf("expected minSize 100000000, got %f", received.Orderbook.MinSize.Float64())
	}
	if received.Version != 0 || received.Orderbook.Midpoint != 0 {
		t.Fatalf("expected zero version/midpoint when absent, got version=%d midpoint=%f", received.Version, received.Orderbook.Midpoint)
	}
}

func TestWebSocketClient_OnMarketCreated_Parsing(t *testing.T) {
	t.Parallel()

	ws := NewWebSocketClient()

	var received MarketCreatedEvent
	called := false
	ws.OnMarketCreated(func(event MarketCreatedEvent) {
		called = true
		received = event
	})

	ws.dispatchLocal("marketCreated", json.RawMessage(`{
		"slug":"market-123",
		"title":"BTC above 120k?",
		"type":"CLOB",
		"groupSlug":"crypto-btc-apr",
		"categoryIds":[1,2],
		"createdAt":"2026-04-02T08:00:00.000Z"
	}`))

	if !called {
		t.Fatal("expected OnMarketCreated handler to be called")
	}
	if received.Slug != "market-123" {
		t.Fatalf("expected slug market-123, got %s", received.Slug)
	}
	if received.Title != "BTC above 120k?" {
		t.Fatalf("expected title to round-trip, got %q", received.Title)
	}
	if received.Type != "CLOB" {
		t.Fatalf("expected type CLOB, got %s", received.Type)
	}
	if received.GroupSlug == nil || *received.GroupSlug != "crypto-btc-apr" {
		t.Fatalf("expected groupSlug crypto-btc-apr, got %+v", received.GroupSlug)
	}
	if len(received.CategoryIDs) != 2 {
		t.Fatalf("expected 2 category ids, got %d", len(received.CategoryIDs))
	}
}

func TestWebSocketClient_OnMarketResolved_Parsing(t *testing.T) {
	t.Parallel()

	ws := NewWebSocketClient()

	var received MarketResolvedEvent
	called := false
	ws.OnMarketResolved(func(event MarketResolvedEvent) {
		called = true
		received = event
	})

	ws.dispatchLocal("marketResolved", json.RawMessage(`{
		"slug":"market-123",
		"type":"CLOB",
		"winningOutcome":"YES",
		"winningIndex":0,
		"resolutionDate":"2026-04-02T09:00:00.000Z"
	}`))

	if !called {
		t.Fatal("expected OnMarketResolved handler to be called")
	}
	if received.Slug != "market-123" {
		t.Fatalf("expected slug market-123, got %s", received.Slug)
	}
	if received.WinningOutcome != "YES" {
		t.Fatalf("expected winningOutcome YES, got %s", received.WinningOutcome)
	}
	if received.WinningIndex != 0 {
		t.Fatalf("expected winningIndex 0, got %d", received.WinningIndex)
	}
}

func TestBuildWebSocketHeaders_IncludesSDKTrackingHeadersWithoutAuth(t *testing.T) {
	t.Parallel()

	headers, err := buildWebSocketHeaders("", nil)
	if err != nil {
		t.Fatalf("expected websocket headers without auth to succeed, got %v", err)
	}

	if got := headers.Get("x-sdk-version"); got == "" {
		t.Fatal("expected x-sdk-version header to be present")
	}
	if got := headers.Get("user-agent"); got == "" {
		t.Fatal("expected user-agent header to be present")
	} else if want := "go/" + runtime.Version(); !strings.Contains(got, want) {
		t.Fatalf("expected user-agent %q to contain %q", got, want)
	}
	if got := headers.Get("X-API-Key"); got != "" {
		t.Fatalf("expected X-API-Key to be omitted without auth, got %q", got)
	}
}

func TestBuildWebSocketHeaders_IncludesTrackingHeadersWithHMAC(t *testing.T) {
	t.Parallel()

	headers, err := buildWebSocketHeaders("", &HMACCredentials{
		TokenID: "token-123",
		Secret:  "c2VjcmV0",
	})
	if err != nil {
		t.Fatalf("expected websocket headers with HMAC to succeed, got %v", err)
	}

	if got := headers.Get("x-sdk-version"); got == "" {
		t.Fatal("expected x-sdk-version header to be present")
	}
	if got := headers.Get("user-agent"); got == "" {
		t.Fatal("expected user-agent header to be present")
	}
	if got := headers.Get("lmts-api-key"); got != "token-123" {
		t.Fatalf("expected lmts-api-key token-123, got %q", got)
	}
	if got := headers.Get("lmts-signature"); got == "" {
		t.Fatal("expected lmts-signature header to be present")
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
	key := ws.subscriptionKey(ChannelSubscribeMarketPrices, SubscriptionOptions{
		MarketSlugs: []string{"eth", "btc"},
	})
	reorderedKey := ws.subscriptionKey(ChannelSubscribeMarketPrices, SubscriptionOptions{
		MarketSlugs: []string{"btc", "eth"},
	})
	if key != reorderedKey {
		t.Fatalf("expected subscription key to be order-independent, got %s vs %s", key, reorderedKey)
	}

	channel := ws.channelFromKey(key)
	if channel != ChannelSubscribePositions {
		channel = ws.channelFromKey("subscribe_positions:btc")
	}
	if channel != ChannelSubscribePositions {
		t.Fatalf("expected subscribe_positions channel, got %s", channel)
	}
}

func TestWebSocketClient_ChannelInventoryIncludesServerSubscriptionEvents(t *testing.T) {
	t.Parallel()

	cases := []struct {
		channel      SubscriptionChannel
		wireName     string
		requiresAuth bool
	}{
		{ChannelSubscribeOrderEvents, "subscribe_order_events", true},
		{ChannelSubscribeLiveSports, "subscribe_live_sports", false},
		{ChannelSubscribeLiveEsports, "subscribe_live_esports", false},
		{ChannelSubscribeMarketLifecycle, "subscribe_market_lifecycle", false},
		{ChannelUnsubscribeMarketLifecycle, "unsubscribe_market_lifecycle", false},
	}

	ws := NewWebSocketClient()
	for _, tc := range cases {
		if string(tc.channel) != tc.wireName {
			t.Fatalf("expected channel %s to use wire name %s", tc.channel, tc.wireName)
		}
		if got := ws.channelFromKey(tc.wireName + "|{}"); got != tc.channel {
			t.Fatalf("expected key to map to %s, got %s", tc.channel, got)
		}
		if got := requiresWebSocketAuth(tc.channel); got != tc.requiresAuth {
			t.Fatalf("expected requires auth=%v for %s, got %v", tc.requiresAuth, tc.channel, got)
		}
	}
}

func TestWebSocketClient_Off_RemovesSocketHandlerAfterInternalHandlers(t *testing.T) {
	t.Parallel()

	ws := NewWebSocketClient(WithAutoReconnect(false))
	ws.sio = &socketIOClient{
		namespace: "/markets",
		handlers:  map[string][]handlerEntry{},
		done:      make(chan struct{}),
		ackChans:  map[int]chan json.RawMessage{},
		logger:    NewNoOpLogger(),
	}
	ws.state = StateConnected
	ws.setupInternalHandlers()

	count := 0
	id := ws.On("customEvent", func(data json.RawMessage) {
		count++
	})
	ws.Off("customEvent", id)
	ws.sio.dispatchEvent("customEvent", json.RawMessage(`{}`))
	if count != 0 {
		t.Fatalf("expected custom handler to be removed, got count=%d", count)
	}
}

func TestWebSocketClient_SetAPIKey_ReconnectPreservesDistinctSubscriptions(t *testing.T) {
	oldFactory := socketIOClientFactory
	defer func() {
		socketIOClientFactory = oldFactory
	}()

	var reconnectEmits int32
	nextSIO := &socketIOClient{
		namespace: "/markets",
		handlers:  map[string][]handlerEntry{},
		done:      make(chan struct{}),
		ackChans:  map[int]chan json.RawMessage{},
		logger:    NewNoOpLogger(),
		emitHook: func(event string, data interface{}) error {
			atomic.AddInt32(&reconnectEmits, 1)
			return nil
		},
	}
	socketIOClientFactory = func(wsURL, namespace string, apiKey string, hmacCreds *HMACCredentials, logger Logger) (*socketIOClient, error) {
		if apiKey != "new-key" {
			t.Fatalf("expected reconnect to use new API key, got %q", apiKey)
		}
		if hmacCreds != nil {
			t.Fatalf("expected HMAC credentials to be nil for API-key reconnect, got %+v", hmacCreds)
		}
		return nextSIO, nil
	}

	ws := NewWebSocketClient(WithWebSocketAPIKey("old-key"), WithAutoReconnect(false))
	ws.sio = &socketIOClient{
		namespace: "/markets",
		handlers:  map[string][]handlerEntry{},
		done:      make(chan struct{}),
		ackChans:  map[int]chan json.RawMessage{},
		logger:    NewNoOpLogger(),
		emitHook: func(event string, data interface{}) error {
			return nil
		},
		closeHook: func() error {
			return nil
		},
	}
	ws.state = StateConnected

	if err := ws.Subscribe(context.Background(), ChannelSubscribeMarketPrices, SubscriptionOptions{
		MarketSlugs: []string{"btc", "eth"},
	}); err != nil {
		t.Fatalf("Subscribe #1 returned error: %v", err)
	}
	if err := ws.Subscribe(context.Background(), ChannelSubscribeMarketPrices, SubscriptionOptions{
		MarketAddresses: []string{"0x2", "0x1"},
		Filters: map[string]interface{}{
			"side": "BUY",
		},
	}); err != nil {
		t.Fatalf("Subscribe #2 returned error: %v", err)
	}

	ws.mu.RLock()
	if got := len(ws.subscriptions); got != 2 {
		ws.mu.RUnlock()
		t.Fatalf("expected two distinct subscriptions before reconnect, got %d", got)
	}
	ws.mu.RUnlock()

	ws.SetAPIKey("new-key")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ws.IsConnected() && atomic.LoadInt32(&reconnectEmits) == 2 {
			ws.mu.RLock()
			subCount := len(ws.subscriptions)
			ws.mu.RUnlock()
			if subCount != 2 {
				t.Fatalf("expected subscriptions to be preserved after reconnect, got %d", subCount)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for reconnect and resubscribe, emits=%d state=%s", atomic.LoadInt32(&reconnectEmits), ws.State())
}

func TestWebSocketClient_Connect_PropagatesHMACCredentials(t *testing.T) {
	oldFactory := socketIOClientFactory
	defer func() {
		socketIOClientFactory = oldFactory
	}()

	socketIOClientFactory = func(wsURL, namespace string, apiKey string, hmacCreds *HMACCredentials, logger Logger) (*socketIOClient, error) {
		if apiKey != "" {
			t.Fatalf("expected API key to be empty, got %q", apiKey)
		}
		if hmacCreds == nil || hmacCreds.TokenID != "token-1" {
			t.Fatalf("expected HMAC credentials to be forwarded, got %+v", hmacCreds)
		}

		return &socketIOClient{
			namespace: "/markets",
			handlers:  map[string][]handlerEntry{},
			done:      make(chan struct{}),
			ackChans:  map[int]chan json.RawMessage{},
			logger:    NewNoOpLogger(),
		}, nil
	}

	ws := NewWebSocketClient(
		WithWebSocketHMACCredentials(HMACCredentials{TokenID: "token-1", Secret: "secret"}),
		WithAutoReconnect(false),
	)
	if err := ws.Connect(context.Background()); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
}
