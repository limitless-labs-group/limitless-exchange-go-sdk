package limitless

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

var socketIOClientFactory = newSocketIOClient

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

// WithWebSocketHMACCredentials sets HMAC credentials for authenticated subscriptions.
func WithWebSocketHMACCredentials(creds HMACCredentials) WebSocketOption {
	return func(ws *WebSocketClient) {
		ws.config.HMACCreds = &HMACCredentials{
			TokenID: creds.TokenID,
			Secret:  creds.Secret,
		}
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
			HMACCreds:            nil,
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
	ws.mu.Lock()
	ws.config.APIKey = key
	connected := ws.state == StateConnected && ws.sio != nil
	ws.mu.Unlock()

	if connected {
		ws.logger.Info("API key updated, reconnecting...")
		go func() {
			ws.Disconnect()
			_ = ws.Connect(context.Background())
		}()
	}
}

// SetHMACCredentials sets HMAC credentials. If already connected, reconnects with new auth.
func (ws *WebSocketClient) SetHMACCredentials(creds HMACCredentials) {
	ws.mu.Lock()
	ws.config.HMACCreds = &HMACCredentials{
		TokenID: creds.TokenID,
		Secret:  creds.Secret,
	}
	connected := ws.state == StateConnected && ws.sio != nil
	ws.mu.Unlock()

	if connected {
		ws.logger.Info("HMAC credentials updated, reconnecting...")
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
	config := ws.config
	ws.state = StateConnecting
	ws.mu.Unlock()

	ws.logger.Info("Connecting to WebSocket", map[string]any{"url": config.URL})

	// Create connection with timeout
	type connectResult struct {
		sio *socketIOClient
		err error
	}
	ch := make(chan connectResult, 1)

	go func() {
		sio, err := socketIOClientFactory(config.URL, "/markets", config.APIKey, config.HMACCreds, ws.logger)
		ch <- connectResult{sio, err}
	}()

	timeout := time.Duration(config.TimeoutMs) * time.Millisecond
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
		return fmt.Errorf("connection timeout after %dms", config.TimeoutMs)

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
	sio := ws.sio
	ws.sio = nil
	ws.state = StateDisconnected
	ws.mu.Unlock()

	if sio == nil {
		return
	}

	ws.logger.Info("Disconnecting from WebSocket")
	_ = sio.Close()
}

// Subscribe subscribes to a channel.
func (ws *WebSocketClient) Subscribe(ctx context.Context, channel SubscriptionChannel, options SubscriptionOptions) error {
	if !ws.IsConnected() {
		return fmt.Errorf("WebSocket not connected. Call Connect() first")
	}

	if err := validateSubscriptionChannel(channel); err != nil {
		return err
	}

	// Check auth requirement
	if requiresWebSocketAuth(channel) && ws.config.APIKey == "" && ws.config.HMACCreds == nil {
		return fmt.Errorf(
			"authentication is required for '%s' subscription. "+
				"Please provide WithWebSocketAPIKey(...) or WithWebSocketHMACCredentials(...), or set LIMITLESS_API_KEY for environment-based auto-loading",
			channel,
		)
	}

	normalizedOptions := normalizeSubscriptionOptions(options)
	key := ws.subscriptionKey(channel, normalizedOptions)
	ws.mu.Lock()
	ws.subscriptions[key] = normalizedOptions
	sio := ws.sio
	ws.mu.Unlock()
	if sio == nil {
		return fmt.Errorf("WebSocket not connected. Call Connect() first")
	}

	ws.logger.Info("Subscribing to channel", map[string]any{
		"channel": string(channel),
	})

	if err := sio.Emit(string(channel), normalizedOptions); err != nil {
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

	if err := validateSubscriptionChannel(channel); err != nil {
		return err
	}

	normalizedOptions := normalizeSubscriptionOptions(options)
	key := ws.subscriptionKey(channel, normalizedOptions)
	ws.mu.Lock()
	delete(ws.subscriptions, key)
	sio := ws.sio
	ws.mu.Unlock()
	if sio == nil {
		return fmt.Errorf("WebSocket not connected")
	}

	ws.logger.Info("Unsubscribing from channel", map[string]any{
		"channel": string(channel),
	})

	data := map[string]any{
		"channel": string(channel),
	}
	if normalizedOptions.MarketSlug != "" {
		data["marketSlug"] = normalizedOptions.MarketSlug
	}
	if len(normalizedOptions.MarketSlugs) > 0 {
		data["marketSlugs"] = normalizedOptions.MarketSlugs
	}
	if normalizedOptions.MarketAddress != "" {
		data["marketAddress"] = normalizedOptions.MarketAddress
	}
	if len(normalizedOptions.MarketAddresses) > 0 {
		data["marketAddresses"] = normalizedOptions.MarketAddresses
	}
	if len(normalizedOptions.Filters) > 0 {
		data["filters"] = normalizedOptions.Filters
	}

	resp, err := sio.EmitWithAck("unsubscribe", data, 5*time.Second)
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
	var remaining []wsHandlerEntry
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
			ws.handlers[event] = append([]wsHandlerEntry(nil), filtered...)
			remaining = append([]wsHandlerEntry(nil), ws.handlers[event]...)
		}
	}
	sio := ws.sio
	if len(handlerIDs) == 0 {
		remaining = nil
	} else if len(remaining) == 0 {
		remaining = append([]wsHandlerEntry(nil), ws.handlers[event]...)
	}
	ws.mu.Unlock()

	if sio != nil {
		sio.Off(event)
		for _, entry := range remaining {
			sio.On(event, entry.fn)
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

// OnOraclePriceData registers a handler for oracle price update events.
func (ws *WebSocketClient) OnOraclePriceData(handler func(OraclePriceData)) {
	ws.On("oraclePriceData", func(data json.RawMessage) {
		var update OraclePriceData
		if err := json.Unmarshal(data, &update); err != nil {
			ws.logger.Error("Failed to parse oracle price data", err)
			return
		}
		handler(update)
	})
}

// OnOrderEvent registers a handler for raw order lifecycle events.
func (ws *WebSocketClient) OnOrderEvent(handler func(OrderEvent)) {
	ws.On("orderEvent", handler)
}

// OnMatchedOrderEvent registers a handler for pre-settlement per-fill MATCHED
// order events. It listens on the shared "orderEvent" socket.io event and
// dispatches only frames with type "MATCHED"; other frames are ignored.
func (ws *WebSocketClient) OnMatchedOrderEvent(handler func(MatchedOrderEvent)) {
	ws.On("orderEvent", func(data json.RawMessage) {
		var disc struct {
			Source string `json:"source"`
			Type   string `json:"type"`
		}
		if err := json.Unmarshal(data, &disc); err != nil {
			ws.logger.Error("Failed to parse orderEvent discriminator", err)
			return
		}
		if disc.Type != "MATCHED" {
			return
		}
		var event MatchedOrderEvent
		if err := json.Unmarshal(data, &event); err != nil {
			ws.logger.Error("Failed to parse matched order event", err)
			return
		}
		handler(event)
	})
}

// OnExecutionOrderEvent registers a handler for FAK/FOK terminal EXECUTION
// order events. It listens on the shared "orderEvent" socket.io event and
// dispatches only frames with type "EXECUTION"; other frames are ignored.
func (ws *WebSocketClient) OnExecutionOrderEvent(handler func(ExecutionOrderEvent)) {
	ws.On("orderEvent", func(data json.RawMessage) {
		var disc struct {
			Source string `json:"source"`
			Type   string `json:"type"`
		}
		if err := json.Unmarshal(data, &disc); err != nil {
			ws.logger.Error("Failed to parse orderEvent discriminator", err)
			return
		}
		if disc.Type != "EXECUTION" {
			return
		}
		var event ExecutionOrderEvent
		if err := json.Unmarshal(data, &event); err != nil {
			ws.logger.Error("Failed to parse execution order event", err)
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

// OnMarketCreated registers a handler for market-created lifecycle events.
func (ws *WebSocketClient) OnMarketCreated(handler func(MarketCreatedEvent)) {
	ws.On("marketCreated", func(data json.RawMessage) {
		var event MarketCreatedEvent
		if err := json.Unmarshal(data, &event); err != nil {
			ws.logger.Error("Failed to parse marketCreated event", err)
			return
		}
		handler(event)
	})
}

// OnMarketResolved registers a handler for market-resolved lifecycle events.
func (ws *WebSocketClient) OnMarketResolved(handler func(MarketResolvedEvent)) {
	ws.On("marketResolved", func(data json.RawMessage) {
		var event MarketResolvedEvent
		if err := json.Unmarshal(data, &event); err != nil {
			ws.logger.Error("Failed to parse marketResolved event", err)
			return
		}
		handler(event)
	})
}

// OnLiveSportsUpdate registers a handler for live sports snapshot events.
func (ws *WebSocketClient) OnLiveSportsUpdate(handler func(LiveSportsUpdate)) {
	ws.On("live_sports_update", handler)
}

// OnLiveEsportsUpdate registers a handler for live esports snapshot events.
func (ws *WebSocketClient) OnLiveEsportsUpdate(handler func(LiveEsportsUpdate)) {
	ws.On("live_esports_update", handler)
}

// OnSystem registers a handler for websocket system messages.
func (ws *WebSocketClient) OnSystem(handler func(SystemEvent)) {
	ws.On("system", handler)
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
	normalized := normalizeSubscriptionOptions(options)
	raw, err := json.Marshal(normalized)
	if err != nil {
		return string(channel) + "|" + fmt.Sprintf("%v", normalized)
	}
	return string(channel) + "|" + string(raw)
}

func (ws *WebSocketClient) channelFromKey(key string) SubscriptionChannel {
	if channel, _, ok := strings.Cut(key, "|"); ok {
		return SubscriptionChannel(channel)
	}
	if channel, _, ok := strings.Cut(key, ":"); ok {
		return SubscriptionChannel(channel)
	}
	return SubscriptionChannel(key)
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

func normalizeSubscriptionOptions(options SubscriptionOptions) SubscriptionOptions {
	normalized := SubscriptionOptions{
		MarketSlug:    options.MarketSlug,
		MarketAddress: options.MarketAddress,
	}

	if len(options.MarketSlugs) > 0 {
		normalized.MarketSlugs = append([]string(nil), options.MarketSlugs...)
		sort.Strings(normalized.MarketSlugs)
	}
	if len(options.MarketAddresses) > 0 {
		normalized.MarketAddresses = append([]string(nil), options.MarketAddresses...)
		sort.Strings(normalized.MarketAddresses)
	}
	if len(options.Filters) > 0 {
		normalized.Filters = normalizeFilterMap(options.Filters)
	}

	return normalized
}

func normalizeFilterMap(filters map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{}, len(filters))
	keys := make([]string, 0, len(filters))
	for key := range filters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		normalized[key] = normalizeFilterValue(filters[key])
	}
	return normalized
}

func normalizeFilterValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]any:
		return normalizeFilterMap(v)
	case []string:
		out := append([]string(nil), v...)
		sort.Strings(out)
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalizeFilterValue(item)
		}
		return out
	default:
		return value
	}
}

func requiresWebSocketAuth(channel SubscriptionChannel) bool {
	switch channel {
	case ChannelSubscribePositions, ChannelSubscribeTransactions, ChannelSubscribeOrderEvents:
		return true
	default:
		return false
	}
}

func validateSubscriptionChannel(channel SubscriptionChannel) error {
	switch channel {
	case ChannelSubscribeMarketPrices,
		ChannelSubscribePositions,
		ChannelSubscribeTransactions,
		ChannelSubscribeOrderEvents,
		ChannelSubscribeLiveSports,
		ChannelSubscribeLiveEsports,
		ChannelSubscribeMarketLifecycle,
		ChannelUnsubscribeMarketLifecycle:
		return nil
	default:
		return fmt.Errorf("unsupported websocket subscription channel %q; use a supported websocket channel constant", channel)
	}
}
