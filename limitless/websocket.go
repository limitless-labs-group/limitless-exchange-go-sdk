package limitless

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"
)

// wsHandlerEntry pairs a handler function with a unique ID.
type wsHandlerEntry struct {
	id int64
	fn func(json.RawMessage)
}

// WebSocketClient provides real-time data streaming from Limitless Exchange.
type WebSocketClient struct {
	config            WebSocketConfig
	sio               *socketIOClient
	state             WebSocketState
	logger            Logger
	subscriptions     map[string]SubscriptionOptions // key → options for re-subscribe
	handlers          map[string][]wsHandlerEntry
	mu                sync.RWMutex
	nextHID           int64
	reconnectAttempts int
}

// WebSocketOption configures a WebSocketClient.
type WebSocketOption func(*WebSocketClient)

// WithWebSocketURL sets the WebSocket URL.
func WithWebSocketURL(url string) WebSocketOption {
	return func(ws *WebSocketClient) {
		ws.config.URL = url
	}
}

// WithWebSocketAPIKey sets the API key for authenticated subscriptions.
func WithWebSocketAPIKey(key string) WebSocketOption {
	return func(ws *WebSocketClient) {
		ws.config.APIKey = key
	}
}

// WithAutoReconnect enables or disables auto-reconnect.
func WithAutoReconnect(enabled bool) WebSocketOption {
	return func(ws *WebSocketClient) {
		ws.config.AutoReconnect = enabled
	}
}

// WithReconnectDelay sets the reconnect delay in milliseconds.
func WithReconnectDelay(ms int) WebSocketOption {
	return func(ws *WebSocketClient) {
		ws.config.ReconnectDelayMs = ms
	}
}

// WithMaxReconnectAttempts sets the maximum reconnection attempts (0 = infinite).
func WithMaxReconnectAttempts(n int) WebSocketOption {
	return func(ws *WebSocketClient) {
		ws.config.MaxReconnectAttempts = n
	}
}

// WithWebSocketTimeout sets the connection timeout in milliseconds.
func WithWebSocketTimeout(ms int) WebSocketOption {
	return func(ws *WebSocketClient) {
		ws.config.TimeoutMs = ms
	}
}

// WithWebSocketLogger sets the logger.
func WithWebSocketLogger(l Logger) WebSocketOption {
	return func(ws *WebSocketClient) {
		ws.logger = l
	}
}

// NewWebSocketClient creates a new WebSocket client.
func NewWebSocketClient(opts ...WebSocketOption) *WebSocketClient {
	ws := &WebSocketClient{
		config: WebSocketConfig{
			URL:                  DefaultWSURL,
			APIKey:               os.Getenv("LIMITLESS_API_KEY"),
			AutoReconnect:        true,
			ReconnectDelayMs:     1000,
			MaxReconnectAttempts: 0,
			TimeoutMs:            10000,
		},
		state:         StateDisconnected,
		logger:        NewNoOpLogger(),
		subscriptions: make(map[string]SubscriptionOptions),
		handlers:      make(map[string][]wsHandlerEntry),
	}

	for _, opt := range opts {
		opt(ws)
	}

	return ws
}

// State returns the current connection state.
func (ws *WebSocketClient) State() WebSocketState {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.state
}

// IsConnected returns true if the client is connected.
func (ws *WebSocketClient) IsConnected() bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.state == StateConnected && ws.sio != nil
}

// SetAPIKey sets the API key. If already connected, reconnects with new auth.
func (ws *WebSocketClient) SetAPIKey(key string) {
	ws.config.APIKey = key

	if ws.IsConnected() {
		ws.logger.Info("API key updated, reconnecting...")
		go func() {
			ws.Disconnect()
			_ = ws.Connect(context.Background())
		}()
	}
}

// Connect establishes a WebSocket connection.
func (ws *WebSocketClient) Connect(ctx context.Context) error {
	ws.mu.Lock()
	if ws.state == StateConnecting || ws.state == StateConnected {
		ws.mu.Unlock()
		ws.logger.Info("Already connected or connecting")
		return nil
	}
	ws.state = StateConnecting
	ws.mu.Unlock()

	ws.logger.Info("Connecting to WebSocket", map[string]any{"url": ws.config.URL})

	// Create connection with timeout
	type connectResult struct {
		sio *socketIOClient
		err error
	}
	ch := make(chan connectResult, 1)

	go func() {
		sio, err := newSocketIOClient(ws.config.URL, "/markets", ws.config.APIKey, ws.logger)
		ch <- connectResult{sio, err}
	}()

	timeout := time.Duration(ws.config.TimeoutMs) * time.Millisecond
	select {
	case result := <-ch:
		if result.err != nil {
			ws.mu.Lock()
			ws.state = StateError
			ws.mu.Unlock()
			return result.err
		}

		ws.mu.Lock()
		ws.sio = result.sio
		ws.state = StateConnected
		ws.reconnectAttempts = 0
		ws.mu.Unlock()

		// Wire internal event handlers
		ws.setupInternalHandlers()

		// Forward all registered handlers to the new socket
		ws.mu.RLock()
		for event, entries := range ws.handlers {
			for _, e := range entries {
				ws.sio.On(event, e.fn)
			}
		}
		ws.mu.RUnlock()

		// Re-subscribe to previous subscriptions
		ws.resubscribeAll()

		ws.logger.Info("WebSocket connected")
		ws.dispatchLocal("connect", nil)
		return nil

	case <-time.After(timeout):
		ws.mu.Lock()
		ws.state = StateError
		ws.mu.Unlock()
		return fmt.Errorf("connection timeout after %dms", ws.config.TimeoutMs)

	case <-ctx.Done():
		ws.mu.Lock()
		ws.state = StateError
		ws.mu.Unlock()
		return ctx.Err()
	}
}

// Disconnect closes the WebSocket connection.
func (ws *WebSocketClient) Disconnect() {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.sio == nil {
		return
	}

	ws.logger.Info("Disconnecting from WebSocket")
	_ = ws.sio.Close()
	ws.sio = nil
	ws.state = StateDisconnected
	ws.subscriptions = make(map[string]SubscriptionOptions)
}



// Subscribe subscribes to a channel.
func (ws *WebSocketClient) Subscribe(ctx context.Context, channel SubscriptionChannel, options SubscriptionOptions) error {
	if !ws.IsConnected() {
		return fmt.Errorf("WebSocket not connected. Call Connect() first")
	}

	// Check auth requirement
	if (channel == ChannelSubscribePositions || channel == ChannelSubscribeTransactions) && ws.config.APIKey == "" {
		return fmt.Errorf(
			"API key is required for '%s' subscription. "+
				"Please provide an API key via WithWebSocketAPIKey or set LIMITLESS_API_KEY environment variable",
			channel,
		)
	}

	key := ws.subscriptionKey(channel, options)
	ws.mu.Lock()
	ws.subscriptions[key] = options
	ws.mu.Unlock()

	ws.logger.Info("Subscribing to channel", map[string]any{
		"channel": string(channel),
	})

	if err := ws.sio.Emit(string(channel), options); err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	ws.logger.Info("Subscription request sent", map[string]any{"channel": string(channel)})
	return nil
}

// Unsubscribe unsubscribes from a channel with server acknowledgment.
func (ws *WebSocketClient) Unsubscribe(ctx context.Context, channel SubscriptionChannel, options SubscriptionOptions) error {
	if !ws.IsConnected() {
		return fmt.Errorf("WebSocket not connected")
	}

	key := ws.subscriptionKey(channel, options)
	ws.mu.Lock()
	delete(ws.subscriptions, key)
	ws.mu.Unlock()

	ws.logger.Info("Unsubscribing from channel", map[string]any{
		"channel": string(channel),
	})

	data := map[string]any{
		"channel": string(channel),
	}
	if options.MarketSlug != "" {
		data["marketSlug"] = options.MarketSlug
	}
	if len(options.MarketSlugs) > 0 {
		data["marketSlugs"] = options.MarketSlugs
	}
	if options.MarketAddress != "" {
		data["marketAddress"] = options.MarketAddress
	}
	if len(options.MarketAddresses) > 0 {
		data["marketAddresses"] = options.MarketAddresses
	}
	if len(options.Filters) > 0 {
		data["filters"] = options.Filters
	}

	resp, err := ws.sio.EmitWithAck("unsubscribe", data, 5*time.Second)
	if err != nil {
		return fmt.Errorf("unsubscribe failed: %w", err)
	}

	// Check for error in response
	var result map[string]any
	if json.Unmarshal(resp, &result) == nil {
		if errMsg, ok := result["error"]; ok {
			return fmt.Errorf("unsubscribe failed: %v", errMsg)
		}
	}

	return nil
}

// On registers an event handler. Can be called before Connect().
// Returns a handler ID that can be passed to Off for granular removal.
func (ws *WebSocketClient) On(event string, handler func(json.RawMessage)) int64 {
	ws.mu.Lock()
	ws.nextHID++
	id := ws.nextHID
	ws.handlers[event] = append(ws.handlers[event], wsHandlerEntry{id: id, fn: handler})
	ws.mu.Unlock()

	// If already connected, attach to socket immediately
	ws.mu.RLock()
	sio := ws.sio
	ws.mu.RUnlock()

	if sio != nil {
		sio.On(event, handler)
	}

	return id
}

// Once registers a one-time event handler that auto-removes after first invocation.
func (ws *WebSocketClient) Once(event string, handler func(json.RawMessage)) int64 {
	var once sync.Once
	var hid int64
	wrapper := func(data json.RawMessage) {
		once.Do(func() {
			handler(data)
			ws.Off(event, hid)
		})
	}
	hid = ws.On(event, wrapper)
	return hid
}

// Off removes handlers for an event. If handlerIDs are provided, only those handlers
// are removed. If no handlerIDs are provided, all handlers for the event are removed.
func (ws *WebSocketClient) Off(event string, handlerIDs ...int64) {
	ws.mu.Lock()
	if len(handlerIDs) == 0 {
		delete(ws.handlers, event)
	} else {
		idSet := make(map[int64]bool, len(handlerIDs))
		for _, id := range handlerIDs {
			idSet[id] = true
		}
		entries := ws.handlers[event]
		filtered := entries[:0]
		for _, e := range entries {
			if !idSet[e.id] {
				filtered = append(filtered, e)
			}
		}
		if len(filtered) == 0 {
			delete(ws.handlers, event)
		} else {
			ws.handlers[event] = filtered
		}
	}
	ws.mu.Unlock()

	ws.mu.RLock()
	sio := ws.sio
	ws.mu.RUnlock()

	if sio != nil {
		if len(handlerIDs) == 0 {
			sio.Off(event)
		} else {
			sio.Off(event, handlerIDs...)
		}
	}
}

// OnOrderbookUpdate registers a handler for orderbook update events.
func (ws *WebSocketClient) OnOrderbookUpdate(handler func(OrderbookUpdate)) {
	ws.On("orderbookUpdate", func(data json.RawMessage) {
		var update OrderbookUpdate
		if err := json.Unmarshal(data, &update); err != nil {
			ws.logger.Error("Failed to parse orderbook update", err)
			return
		}
		handler(update)
	})
}

// OnNewPriceData registers a handler for AMM price update events.
func (ws *WebSocketClient) OnNewPriceData(handler func(NewPriceData)) {
	ws.On("newPriceData", func(data json.RawMessage) {
		var update NewPriceData
		if err := json.Unmarshal(data, &update); err != nil {
			ws.logger.Error("Failed to parse price data", err)
			return
		}
		handler(update)
	})
}

// OnTrade registers a handler for trade events.
func (ws *WebSocketClient) OnTrade(handler func(TradeEvent)) {
	ws.On("trade", func(data json.RawMessage) {
		var event TradeEvent
		if err := json.Unmarshal(data, &event); err != nil {
			ws.logger.Error("Failed to parse trade event", err)
			return
		}
		handler(event)
	})
}

// OnOrder registers a handler for order update events.
func (ws *WebSocketClient) OnOrder(handler func(OrderUpdate)) {
	ws.On("order", func(data json.RawMessage) {
		var event OrderUpdate
		if err := json.Unmarshal(data, &event); err != nil {
			ws.logger.Error("Failed to parse order event", err)
			return
		}
		handler(event)
	})
}

// OnFill registers a handler for fill events.
func (ws *WebSocketClient) OnFill(handler func(FillEvent)) {
	ws.On("fill", func(data json.RawMessage) {
		var event FillEvent
		if err := json.Unmarshal(data, &event); err != nil {
			ws.logger.Error("Failed to parse fill event", err)
			return
		}
		handler(event)
	})
}

// OnTransaction registers a handler for transaction events.
func (ws *WebSocketClient) OnTransaction(handler func(TransactionEvent)) {
	ws.On("tx", func(data json.RawMessage) {
		var event TransactionEvent
		if err := json.Unmarshal(data, &event); err != nil {
			ws.logger.Error("Failed to parse transaction event", err)
			return
		}
		handler(event)
	})
}

// OnMarket registers a handler for market update events.
func (ws *WebSocketClient) OnMarket(handler func(MarketUpdateEvent)) {
	ws.On("market", func(data json.RawMessage) {
		var event MarketUpdateEvent
		if err := json.Unmarshal(data, &event); err != nil {
			ws.logger.Error("Failed to parse market event", err)
			return
		}
		handler(event)
	})
}

func (ws *WebSocketClient) setupInternalHandlers() {
	ws.sio.On("disconnect", func(data json.RawMessage) {
		ws.mu.Lock()
		ws.state = StateDisconnected
		ws.mu.Unlock()

		ws.logger.Info("WebSocket disconnected")

		if ws.config.AutoReconnect {
			go ws.reconnect()
		}
	})
}

func (ws *WebSocketClient) reconnect() {
	ws.mu.Lock()
	ws.state = StateReconnecting
	ws.mu.Unlock()

	delay := time.Duration(ws.config.ReconnectDelayMs) * time.Millisecond
	maxDelay := 60 * time.Second

	for {
		ws.mu.RLock()
		attempts := ws.reconnectAttempts
		maxAttempts := ws.config.MaxReconnectAttempts
		ws.mu.RUnlock()

		if maxAttempts > 0 && attempts >= maxAttempts {
			ws.logger.Error("Max reconnection attempts reached", fmt.Errorf("attempts: %d", attempts))
			ws.mu.Lock()
			ws.state = StateError
			ws.mu.Unlock()
			return
		}

		ws.mu.Lock()
		ws.reconnectAttempts++
		ws.mu.Unlock()

		ws.logger.Info("Reconnecting...", map[string]any{"attempt": attempts + 1})
		ws.dispatchLocal("reconnecting", json.RawMessage(fmt.Sprintf(`%d`, attempts+1)))

		if err := ws.Connect(context.Background()); err != nil {
			ws.logger.Error("Reconnection failed", err)

			// Exponential backoff with ±20% jitter and cap
			jitter := time.Duration(float64(delay) * 0.2 * (rand.Float64()*2 - 1))
			time.Sleep(delay + jitter)
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
			continue
		}

		ws.logger.Info("Reconnected successfully")
		return
	}
}

func (ws *WebSocketClient) resubscribeAll() {
	ws.mu.RLock()
	subs := make(map[string]SubscriptionOptions, len(ws.subscriptions))
	for k, v := range ws.subscriptions {
		subs[k] = v
	}
	ws.mu.RUnlock()

	if len(subs) == 0 {
		return
	}

	ws.logger.Info("Re-subscribing to channels", map[string]any{"count": len(subs)})

	for key, options := range subs {
		channel := ws.channelFromKey(key)
		if err := ws.Subscribe(context.Background(), channel, options); err != nil {
			ws.logger.Error("Failed to re-subscribe", err, map[string]any{"channel": string(channel)})
		}
	}
}

func (ws *WebSocketClient) subscriptionKey(channel SubscriptionChannel, options SubscriptionOptions) string {
	slug := options.MarketSlug
	if slug == "" {
		slug = "global"
	}
	return string(channel) + ":" + slug
}

func (ws *WebSocketClient) channelFromKey(key string) SubscriptionChannel {
	parts := splitFirst(key, ":")
	return SubscriptionChannel(parts)
}

func splitFirst(s, sep string) string {
	idx := 0
	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			return s[:idx]
		}
		idx++
	}
	return s
}

func (ws *WebSocketClient) dispatchLocal(event string, data json.RawMessage) {
	ws.mu.RLock()
	entries, ok := ws.handlers[event]
	ws.mu.RUnlock()

	if !ok {
		return
	}

	for _, e := range entries {
		e.fn(data)
	}
}
