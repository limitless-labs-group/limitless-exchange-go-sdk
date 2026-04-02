package limitless

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Engine.IO packet types
const (
	eioOpen    = "0"
	eioClose   = "1"
	eioPing    = "2"
	eioPong    = "3"
	eioMessage = "4"
)

// Socket.IO packet types (prefixed after Engine.IO message type "4")
const (
	sioConnect    = "0"
	sioDisconnect = "1"
	sioEvent      = "2"
	sioAck        = "3"
	sioError      = "4"
)

// engineIOConfig is returned by the Engine.IO handshake.
type engineIOConfig struct {
	SID          string `json:"sid"`
	PingInterval int    `json:"pingInterval"`
	PingTimeout  int    `json:"pingTimeout"`
}

// handlerEntry pairs a handler function with a unique ID for granular removal.
type handlerEntry struct {
	id int64
	fn func(json.RawMessage)
}

// socketIOClient is a minimal Socket.IO client over gorilla/websocket.
type socketIOClient struct {
	conn         *websocket.Conn
	namespace    string
	handlers     map[string][]handlerEntry
	mu           sync.RWMutex
	writeMu      sync.Mutex
	pingDeadline time.Duration // pingInterval + pingTimeout
	lastPing     time.Time     // last time we received a server ping (or connected)
	lastPingMu   sync.Mutex
	done         chan struct{}
	closed       bool
	closeMu      sync.Mutex
	ackID        int
	nextHID      int64 // next handler ID
	ackChans     map[int]chan json.RawMessage
	ackMu        sync.Mutex
	logger       Logger
	emitHook     func(event string, data interface{}) error
	emitAckHook  func(event string, data interface{}, timeout time.Duration) (json.RawMessage, error)
	closeHook    func() error
}

func newSocketIOClient(wsURL, namespace string, apiKey string, hmacCreds *HMACCredentials, logger Logger) (*socketIOClient, error) {
	// Build WebSocket URL with Engine.IO query params
	// Socket.IO uses /socket.io/ path with EIO=4 and transport=websocket
	u := wsURL + "/socket.io/?EIO=4&transport=websocket"

	header, err := buildWebSocketHeaders(apiKey, hmacCreds)
	if err != nil {
		return nil, err
	}

	logger.Debug("Connecting WebSocket", map[string]any{"url": u})

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(u, header)
	if err != nil {
		return nil, fmt.Errorf("websocket dial failed: %w", err)
	}

	s := &socketIOClient{
		conn:      conn,
		namespace: namespace,
		handlers:  make(map[string][]handlerEntry),
		done:      make(chan struct{}),
		ackChans:  make(map[int]chan json.RawMessage),
		logger:    logger,
	}

	// Read Engine.IO open packet
	_, msg, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read EIO open: %w", err)
	}

	msgStr := string(msg)
	if !strings.HasPrefix(msgStr, eioOpen) {
		conn.Close()
		return nil, fmt.Errorf("expected EIO open packet, got: %s", msgStr)
	}

	var config engineIOConfig
	if err := json.Unmarshal([]byte(msgStr[1:]), &config); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to parse EIO config: %w", err)
	}

	logger.Debug("Engine.IO handshake complete", map[string]any{
		"sid":          config.SID,
		"pingInterval": config.PingInterval,
		"pingTimeout":  config.PingTimeout,
	})

	// Send Socket.IO connect to namespace
	connectPacket := eioMessage + sioConnect + namespace + ","
	if err := s.writeMessage(connectPacket); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send SIO connect: %w", err)
	}

	// Read Socket.IO connect ack
	_, msg, err = conn.ReadMessage()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read SIO connect ack: %w", err)
	}

	msgStr = string(msg)
	expectedPrefix := eioMessage + sioConnect + namespace + ","
	if !strings.HasPrefix(msgStr, expectedPrefix) {
		conn.Close()
		return nil, fmt.Errorf("expected SIO connect ack for namespace %s, got: %s", namespace, msgStr)
	}

	logger.Info("Socket.IO connected to namespace", map[string]any{"namespace": namespace})

	// In Engine.IO v4, the SERVER sends pings and the CLIENT responds with pongs.
	// We don't send pings; we only monitor for server ping timeout.
	pingInterval := time.Duration(config.PingInterval) * time.Millisecond
	if pingInterval <= 0 {
		pingInterval = 25 * time.Second
	}
	pingTimeout := time.Duration(config.PingTimeout) * time.Millisecond
	if pingTimeout <= 0 {
		pingTimeout = 20 * time.Second
	}
	s.pingDeadline = pingInterval + pingTimeout
	s.lastPing = time.Now()
	go s.pingMonitor()

	// Start read loop
	go s.readLoop()

	return s, nil
}

func (s *socketIOClient) writeMessage(msg string) error {
	if s.conn == nil {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.TextMessage, []byte(msg))
}

func (s *socketIOClient) pingMonitor() {
	// Check every second whether the server has missed its ping deadline.
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.lastPingMu.Lock()
			elapsed := time.Since(s.lastPing)
			deadline := s.pingDeadline
			s.lastPingMu.Unlock()

			if elapsed > deadline {
				s.logger.Warn("Server ping timeout", map[string]any{
					"elapsed":  elapsed.String(),
					"deadline": deadline.String(),
				})
				s.dispatchEvent("disconnect", json.RawMessage(`"ping timeout"`))
				return
			}
		case <-s.done:
			return
		}
	}
}

func (s *socketIOClient) readLoop() {
	for {
		select {
		case <-s.done:
			return
		default:
		}

		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			s.closeMu.Lock()
			closed := s.closed
			s.closeMu.Unlock()
			if !closed {
				s.logger.Error("WebSocket read error", err)
				s.dispatchEvent("disconnect", json.RawMessage(`"read error"`))
			}
			return
		}

		s.handleMessage(string(msg))
	}
}

func (s *socketIOClient) handleMessage(msg string) {
	if len(msg) == 0 {
		return
	}

	switch {
	case msg == eioPong:
		// Pong received, ignore
	case msg == eioPing:
		// Server ping (EIO v4), respond with pong and reset deadline
		s.lastPingMu.Lock()
		s.lastPing = time.Now()
		s.lastPingMu.Unlock()
		_ = s.writeMessage(eioPong)
	case strings.HasPrefix(msg, eioMessage):
		s.handleSocketIOPacket(msg[1:])
	case msg == eioClose:
		s.dispatchEvent("disconnect", json.RawMessage(`"server close"`))
	}
}

func (s *socketIOClient) handleSocketIOPacket(packet string) {
	if len(packet) == 0 {
		return
	}

	packetType := string(packet[0])
	rest := packet[1:]

	// Strip namespace prefix if present
	if strings.HasPrefix(rest, s.namespace+",") {
		rest = rest[len(s.namespace)+1:]
	}

	switch packetType {
	case sioEvent:
		s.handleEvent(rest)
	case sioAck:
		s.handleAck(rest)
	case sioDisconnect:
		s.dispatchEvent("disconnect", json.RawMessage(`"namespace disconnect"`))
	case sioError:
		s.logger.Error("Socket.IO error", fmt.Errorf("server error: %s", rest))
		s.dispatchEvent("error", json.RawMessage(fmt.Sprintf(`"%s"`, rest)))
	}
}

func (s *socketIOClient) handleEvent(data string) {
	// Socket.IO events are JSON arrays: ["eventName", ...args]
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(data), &arr); err != nil {
		s.logger.Error("Failed to parse SIO event", err, map[string]any{"data": data})
		return
	}

	if len(arr) < 1 {
		return
	}

	var eventName string
	if err := json.Unmarshal(arr[0], &eventName); err != nil {
		s.logger.Error("Failed to parse event name", err)
		return
	}

	var eventData json.RawMessage
	if len(arr) > 1 {
		eventData = arr[1]
	}

	s.dispatchEvent(eventName, eventData)
}

func (s *socketIOClient) dispatchEvent(event string, data json.RawMessage) {
	s.mu.RLock()
	handlers, ok := s.handlers[event]
	s.mu.RUnlock()

	if !ok || len(handlers) == 0 {
		return
	}

	for _, h := range handlers {
		h.fn(data)
	}
}

// On registers an event handler and returns a handler ID for later removal.
func (s *socketIOClient) On(event string, handler func(json.RawMessage)) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextHID++
	id := s.nextHID
	s.handlers[event] = append(s.handlers[event], handlerEntry{id: id, fn: handler})
	return id
}

// Off removes handlers for an event. If handlerIDs are provided, only those are removed.
// If no handlerIDs are provided, all handlers for the event are removed.
func (s *socketIOClient) Off(event string, handlerIDs ...int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(handlerIDs) == 0 {
		delete(s.handlers, event)
		return
	}
	idSet := make(map[int64]bool, len(handlerIDs))
	for _, id := range handlerIDs {
		idSet[id] = true
	}
	entries := s.handlers[event]
	filtered := entries[:0]
	for _, e := range entries {
		if !idSet[e.id] {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) == 0 {
		delete(s.handlers, event)
	} else {
		s.handlers[event] = filtered
	}
}

// Emit sends an event to the server.
func (s *socketIOClient) Emit(event string, data interface{}) error {
	if s.emitHook != nil {
		return s.emitHook(event, data)
	}

	var payload []interface{}
	payload = append(payload, event)
	if data != nil {
		payload = append(payload, data)
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal emit data: %w", err)
	}

	packet := eioMessage + sioEvent + s.namespace + "," + string(b)
	return s.writeMessage(packet)
}

// EmitWithAck sends an event and waits for an acknowledgment response.
func (s *socketIOClient) EmitWithAck(event string, data interface{}, timeout time.Duration) (json.RawMessage, error) {
	if s.emitAckHook != nil {
		return s.emitAckHook(event, data, timeout)
	}

	s.ackMu.Lock()
	s.ackID++
	ackID := s.ackID
	ch := make(chan json.RawMessage, 1)
	s.ackChans[ackID] = ch
	s.ackMu.Unlock()

	defer func() {
		s.ackMu.Lock()
		delete(s.ackChans, ackID)
		s.ackMu.Unlock()
	}()

	var payload []interface{}
	payload = append(payload, event)
	if data != nil {
		payload = append(payload, data)
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal emit data: %w", err)
	}

	// Socket.IO ack format: "42<ackID><namespace>,<data>"
	packet := eioMessage + sioEvent + fmt.Sprintf("%d", ackID) + s.namespace + "," + string(b)
	if err := s.writeMessage(packet); err != nil {
		return nil, fmt.Errorf("failed to send emit: %w", err)
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("ack timeout after %v", timeout)
	case <-s.done:
		return nil, fmt.Errorf("connection closed while waiting for ack")
	}
}

// handleAck processes an acknowledgment packet.
func (s *socketIOClient) handleAck(data string) {
	// Parse ack ID from the beginning of data
	ackIDEnd := 0
	for ackIDEnd < len(data) && data[ackIDEnd] >= '0' && data[ackIDEnd] <= '9' {
		ackIDEnd++
	}
	if ackIDEnd == 0 {
		return
	}

	var ackID int
	if _, err := fmt.Sscanf(data[:ackIDEnd], "%d", &ackID); err != nil {
		return
	}

	rest := data[ackIDEnd:]
	var ackData json.RawMessage
	if len(rest) > 0 {
		// Parse the ack response data (JSON array)
		var arr []json.RawMessage
		if err := json.Unmarshal([]byte(rest), &arr); err == nil && len(arr) > 0 {
			ackData = arr[0]
		} else {
			ackData = json.RawMessage(rest)
		}
	}

	s.ackMu.Lock()
	ch, ok := s.ackChans[ackID]
	s.ackMu.Unlock()

	if ok {
		select {
		case ch <- ackData:
		default:
		}
	}
}

// Close disconnects from the server.
func (s *socketIOClient) Close() error {
	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()
		return nil
	}
	s.closed = true
	s.closeMu.Unlock()

	if s.done != nil {
		close(s.done)
	}

	if s.closeHook != nil {
		return s.closeHook()
	}

	if s.conn == nil {
		return nil
	}

	// Send Socket.IO disconnect
	_ = s.writeMessage(eioMessage + sioDisconnect + s.namespace + ",")

	return s.conn.Close()
}
