package tinode

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Uuenu/taskchat/internal/config"
	"github.com/gorilla/websocket"
)

const (
	tinodeVersion  = "0.22"
	generalTopic   = "grp_general"
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024

	// Reconnect settings
	initialReconnectDelay = 1 * time.Second
	maxReconnectDelay     = 60 * time.Second
	reconnectBackoffMult  = 2.0

	// Response timeout
	responseTimeout = 10 * time.Second
)

// ConnectionState represents the current connection state
type ConnectionState int32

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateAuthenticated
)

// Client handles real-time messaging via Tinode
type Client struct {
	cfg        *config.TinodeConfig
	conn       *websocket.Conn
	mu         sync.RWMutex
	msgID      atomic.Int64
	callbacks  map[string]chan map[string]interface{}
	callbackMu sync.RWMutex

	onMessage    func(author, content string)
	onReconnect  func()
	onDisconnect func(err error)

	state      atomic.Int32
	authToken  string
	serverAddr string

	// Graceful shutdown
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	shutdownMu sync.Mutex
	shutdown   bool

	// Reconnect control
	reconnectMu      sync.Mutex
	reconnectEnabled bool
	serviceLogin     string // Optional service account email
	servicePassword  string // Optional service account password
}

// NewTinodeService creates a new Tinode service instance
func NewTinodeService(cfg *config.TinodeConfig) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		cfg:              cfg,
		callbacks:        make(map[string]chan map[string]interface{}),
		serverAddr:       fmt.Sprintf("ws://%s:%s/v0/channels", cfg.Host, cfg.Port),
		ctx:              ctx,
		cancel:           cancel,
		reconnectEnabled: true,
	}
}

// SetServiceCredentials sets optional service account credentials for auto-login
func (t *Client) SetServiceCredentials(email, password string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.serviceLogin = email
	t.servicePassword = password
}

// SetReconnectEnabled enables or disables auto-reconnect
func (t *Client) SetReconnectEnabled(enabled bool) {
	t.reconnectMu.Lock()
	defer t.reconnectMu.Unlock()
	t.reconnectEnabled = enabled
}

// Connect establishes WebSocket connection to Tinode server with full lifecycle
func (t *Client) Connect() error {
	return t.ConnectWithContext(t.ctx)
}

// ConnectWithContext establishes connection with custom context
func (t *Client) ConnectWithContext(ctx context.Context) error {
	t.shutdownMu.Lock()
	if t.shutdown {
		t.shutdownMu.Unlock()
		return fmt.Errorf("service is shut down")
	}
	t.shutdownMu.Unlock()

	if !t.setState(StateDisconnected, StateConnecting) && !t.setState(StateConnecting, StateConnecting) {
		// Already connected or authenticated
		if t.GetState() >= StateConnected {
			return nil
		}
	}

	if err := t.dial(ctx); err != nil {
		t.setState(StateConnecting, StateDisconnected)
		return err
	}

	// Send hello handshake
	if err := t.hello(ctx); err != nil {
		t.closeConn()
		t.setState(StateConnecting, StateDisconnected)
		return fmt.Errorf("handshake failed: %w", err)
	}

	t.setState(StateConnecting, StateConnected)

	// Start background pumps
	t.wg.Add(2)
	go t.readPump()
	go t.pingPump()

	// Optional: Login with service credentials
	t.mu.RLock()
	serviceLogin := t.serviceLogin
	servicePassword := t.servicePassword
	t.mu.RUnlock()

	if serviceLogin != "" && servicePassword != "" {
		if _, err := t.LoginWithContext(ctx, serviceLogin, servicePassword); err != nil {
			log.Printf("Service login failed (continuing anyway): %v", err)
		} else {
			t.setState(StateConnected, StateAuthenticated)
		}
	}

	// Ensure general topic exists and subscribe
	if err := t.EnsureGeneralTopicWithContext(ctx); err != nil {
		log.Printf("EnsureGeneralTopic failed (continuing): %v", err)
	}

	if err := t.SubscribeToGeneralWithContext(ctx); err != nil {
		log.Printf("SubscribeToGeneral failed (continuing): %v", err)
	}

	return nil
}

func (t *Client) dial(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, t.serverAddr, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Tinode: %w", err)
	}

	t.mu.Lock()
	t.conn = conn
	t.conn.SetReadLimit(maxMessageSize)
	t.conn.SetReadDeadline(time.Now().Add(pongWait))
	t.conn.SetPongHandler(func(string) error {
		t.mu.Lock()
		if t.conn != nil {
			t.conn.SetReadDeadline(time.Now().Add(pongWait))
		}
		t.mu.Unlock()
		return nil
	})
	t.mu.Unlock()

	return nil
}

// hello sends initial handshake to Tinode
func (t *Client) hello(ctx context.Context) error {
	id := t.nextID()
	respChan := t.registerCallback(id)
	defer t.unregisterCallback(id)

	msg := map[string]interface{}{
		"hi": map[string]interface{}{
			"id":   id,
			"ver":  tinodeVersion,
			"ua":   "taskchat/1.0",
			"lang": "en",
		},
	}

	if err := t.sendMessage(msg); err != nil {
		return err
	}

	return t.waitForCtrlSuccess(ctx, respChan, "hello")
}

// CreateAccount creates a new user account in Tinode
func (t *Client) CreateAccount(email, password string) (string, error) {
	return t.CreateAccountWithContext(t.ctx, email, password)
}

// CreateAccountWithContext creates account with custom context
func (t *Client) CreateAccountWithContext(ctx context.Context, email, password string) (string, error) {
	id := t.nextID()
	respChan := t.registerCallback(id)
	defer t.unregisterCallback(id)

	msg := map[string]interface{}{
		"acc": map[string]interface{}{
			"id":     id,
			"user":   "new",
			"scheme": "basic",
			"secret": fmt.Sprintf("%s:%s", email, password),
			"tags":   []string{fmt.Sprintf("email:%s", email)},
			"desc": map[string]interface{}{
				"public": map[string]interface{}{
					"fn": email,
				},
			},
		},
	}

	if err := t.sendMessage(msg); err != nil {
		return "", err
	}

	resp, err := t.waitForResponse(ctx, respChan)
	if err != nil {
		return "", fmt.Errorf("create account: %w", err)
	}

	if ctrl, ok := resp["ctrl"].(map[string]interface{}); ok {
		if code, ok := ctrl["code"].(float64); ok && code >= 200 && code < 300 {
			if params, ok := ctrl["params"].(map[string]interface{}); ok {
				if userID, ok := params["user"].(string); ok {
					return userID, nil
				}
			}
		}
		if text, ok := ctrl["text"].(string); ok {
			return "", fmt.Errorf("tinode error: %s", text)
		}
	}
	return "", fmt.Errorf("unexpected response from Tinode")
}

// Login authenticates user with Tinode
func (t *Client) Login(email, password string) (string, error) {
	return t.LoginWithContext(t.ctx, email, password)
}

// LoginWithContext authenticates with custom context
func (t *Client) LoginWithContext(ctx context.Context, email, password string) (string, error) {
	id := t.nextID()
	respChan := t.registerCallback(id)
	defer t.unregisterCallback(id)

	msg := map[string]interface{}{
		"login": map[string]interface{}{
			"id":     id,
			"scheme": "basic",
			"secret": fmt.Sprintf("%s:%s", email, password),
		},
	}

	if err := t.sendMessage(msg); err != nil {
		return "", err
	}

	resp, err := t.waitForResponse(ctx, respChan)
	if err != nil {
		return "", fmt.Errorf("login: %w", err)
	}

	if ctrl, ok := resp["ctrl"].(map[string]interface{}); ok {
		if code, ok := ctrl["code"].(float64); ok && code >= 200 && code < 300 {
			if params, ok := ctrl["params"].(map[string]interface{}); ok {
				if token, ok := params["token"].(string); ok {
					t.mu.Lock()
					t.authToken = token
					t.mu.Unlock()
					return token, nil
				}
			}
		}
		if text, ok := ctrl["text"].(string); ok {
			return "", fmt.Errorf("tinode login error: %s", text)
		}
	}
	return "", fmt.Errorf("unexpected login response from Tinode")
}

// LoginWithToken authenticates using saved token
func (t *Client) LoginWithToken(token string) error {
	return t.LoginWithTokenContext(t.ctx, token)
}

// LoginWithTokenContext authenticates with token using custom context
func (t *Client) LoginWithTokenContext(ctx context.Context, token string) error {
	id := t.nextID()
	respChan := t.registerCallback(id)
	defer t.unregisterCallback(id)

	msg := map[string]interface{}{
		"login": map[string]interface{}{
			"id":     id,
			"scheme": "token",
			"secret": token,
		},
	}

	if err := t.sendMessage(msg); err != nil {
		return err
	}

	if err := t.waitForCtrlSuccess(ctx, respChan, "token login"); err != nil {
		return err
	}

	t.mu.Lock()
	t.authToken = token
	t.mu.Unlock()

	return nil
}

// SubscribeToGeneral subscribes to the general chat room
func (t *Client) SubscribeToGeneral() error {
	return t.SubscribeToGeneralWithContext(t.ctx)
}

// SubscribeToGeneralWithContext subscribes with custom context
func (t *Client) SubscribeToGeneralWithContext(ctx context.Context) error {
	id := t.nextID()
	respChan := t.registerCallback(id)
	defer t.unregisterCallback(id)

	msg := map[string]interface{}{
		"sub": map[string]interface{}{
			"id":    id,
			"topic": generalTopic,
			"get": map[string]interface{}{
				"what": "desc sub data",
				"data": map[string]interface{}{
					"limit": 50,
				},
			},
		},
	}

	if err := t.sendMessage(msg); err != nil {
		return err
	}

	return t.waitForCtrlSuccess(ctx, respChan, "subscribe to general")
}

// SendToGeneral sends a message to the general chat room with confirmation
func (t *Client) SendToGeneral(content string) error {
	return t.SendToGeneralWithContext(t.ctx, content)
}

// SendToGeneralWithContext sends message with custom context and waits for confirmation
func (t *Client) SendToGeneralWithContext(ctx context.Context, content string) error {
	id := t.nextID()
	respChan := t.registerCallback(id)
	defer t.unregisterCallback(id)

	msg := map[string]interface{}{
		"pub": map[string]interface{}{
			"id":      id,
			"topic":   generalTopic,
			"content": content,
		},
	}

	if err := t.sendMessage(msg); err != nil {
		return err
	}

	// Wait for publish confirmation
	return t.waitForCtrlSuccess(ctx, respChan, "publish message")
}

// SendToGeneralAsync sends message without waiting for confirmation (fire-and-forget)
func (t *Client) SendToGeneralAsync(content string) error {
	id := t.nextID()

	msg := map[string]interface{}{
		"pub": map[string]interface{}{
			"id":      id,
			"topic":   generalTopic,
			"content": content,
		},
	}

	return t.sendMessage(msg)
}

// PublishWithMeta sends a message with additional metadata (e.g., author info)
func (t *Client) PublishWithMeta(ctx context.Context, topic string, content interface{}, head map[string]interface{}) error {
	id := t.nextID()
	respChan := t.registerCallback(id)
	defer t.unregisterCallback(id)

	pubMsg := map[string]interface{}{
		"id":      id,
		"topic":   topic,
		"content": content,
	}

	if head != nil {
		pubMsg["head"] = head
	}

	msg := map[string]interface{}{
		"pub": pubMsg,
	}

	if err := t.sendMessage(msg); err != nil {
		return err
	}

	return t.waitForCtrlSuccess(ctx, respChan, "publish with meta")
}

// SetMessageHandler sets the callback for incoming messages
func (t *Client) SetMessageHandler(handler func(author, content string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onMessage = handler
}

// SetReconnectHandler sets callback called after successful reconnect
func (t *Client) SetReconnectHandler(handler func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onReconnect = handler
}

// SetDisconnectHandler sets callback called on disconnect
func (t *Client) SetDisconnectHandler(handler func(err error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onDisconnect = handler
}

// GetState returns current connection state
func (t *Client) GetState() ConnectionState {
	return ConnectionState(t.state.Load())
}

// IsConnected returns true if connected (may not be authenticated)
func (t *Client) IsConnected() bool {
	return t.GetState() >= StateConnected
}

// IsAuthenticated returns true if connected and authenticated
func (t *Client) IsAuthenticated() bool {
	return t.GetState() >= StateAuthenticated
}

// Close gracefully closes the Tinode connection
func (t *Client) Close() error {
	return t.Shutdown()
}

// Shutdown performs graceful shutdown
func (t *Client) Shutdown() error {
	t.shutdownMu.Lock()
	if t.shutdown {
		t.shutdownMu.Unlock()
		return nil
	}
	t.shutdown = true
	t.shutdownMu.Unlock()

	t.SetReconnectEnabled(false)
	t.cancel()
	t.closeConn()

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Printf("Tinode shutdown timeout waiting for goroutines")
	}

	return nil
}

func (t *Client) closeConn() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn != nil {
		t.conn.Close()
		t.conn = nil
	}
	t.state.Store(int32(StateDisconnected))
}

func (t *Client) setState(from, to ConnectionState) bool {
	return t.state.CompareAndSwap(int32(from), int32(to))
}

func (t *Client) sendMessage(msg interface{}) error {
	t.mu.RLock()
	conn := t.conn
	t.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected to Tinode")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn == nil {
		return fmt.Errorf("not connected to Tinode")
	}

	t.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return t.conn.WriteJSON(msg)
}

func (t *Client) nextID() string {
	return fmt.Sprintf("%d", t.msgID.Add(1))
}

func (t *Client) registerCallback(id string) chan map[string]interface{} {
	ch := make(chan map[string]interface{}, 1)
	t.callbackMu.Lock()
	t.callbacks[id] = ch
	t.callbackMu.Unlock()
	return ch
}

func (t *Client) unregisterCallback(id string) {
	t.callbackMu.Lock()
	delete(t.callbacks, id)
	t.callbackMu.Unlock()
}

func (t *Client) waitForResponse(ctx context.Context, respChan chan map[string]interface{}) (map[string]interface{}, error) {
	select {
	case resp := <-respChan:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(responseTimeout):
		return nil, fmt.Errorf("timeout waiting for response")
	}
}

func (t *Client) waitForCtrlSuccess(ctx context.Context, respChan chan map[string]interface{}, operation string) error {
	resp, err := t.waitForResponse(ctx, respChan)
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}

	if ctrl, ok := resp["ctrl"].(map[string]interface{}); ok {
		if code, ok := ctrl["code"].(float64); ok {
			if code >= 200 && code < 300 {
				return nil
			}
			text := "unknown error"
			if t, ok := ctrl["text"].(string); ok {
				text = t
			}
			return fmt.Errorf("%s failed (code %.0f): %s", operation, code, text)
		}
	}

	return nil // No ctrl in response, assume success
}

func (t *Client) readPump() {
	defer t.wg.Done()

	var readErr error

	defer func() {
		t.closeConn()

		// Notify disconnect handler
		t.mu.RLock()
		handler := t.onDisconnect
		t.mu.RUnlock()
		if handler != nil {
			handler(readErr)
		}

		// Try reconnect if enabled
		t.tryReconnect()
	}()

	for {
		t.mu.RLock()
		conn := t.conn
		t.mu.RUnlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			readErr = err
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				log.Printf("Tinode read error: %v", err)
			}
			return
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Failed to parse Tinode message: %v", err)
			continue
		}

		t.handleMessage(msg)
	}
}

func (t *Client) handleMessage(msg map[string]interface{}) {
	// Handle control messages with callbacks
	if ctrl, ok := msg["ctrl"].(map[string]interface{}); ok {
		if id, ok := ctrl["id"].(string); ok {
			t.callbackMu.RLock()
			ch, exists := t.callbacks[id]
			t.callbackMu.RUnlock()
			if exists {
				select {
				case ch <- msg:
				default:
					// Channel full, drop message
				}
			}
		}
	}

	// Handle incoming data messages
	if data, ok := msg["data"].(map[string]interface{}); ok {
		t.handleDataMessage(data)
	}

	// Handle meta messages (topic info, etc.)
	if meta, ok := msg["meta"].(map[string]interface{}); ok {
		t.handleMetaMessage(meta)
	}

	// Handle presence notifications
	if pres, ok := msg["pres"].(map[string]interface{}); ok {
		t.handlePresMessage(pres)
	}
}

func (t *Client) handleDataMessage(data map[string]interface{}) {
	content := ""
	author := "unknown"

	// Try to extract content
	switch c := data["content"].(type) {
	case string:
		content = c
	case map[string]interface{}:
		// Content might be structured JSON
		if text, ok := c["text"].(string); ok {
			content = text
		} else {
			// Serialize the content
			if b, err := json.Marshal(c); err == nil {
				content = string(b)
			}
		}
	}

	// Extract author
	if from, ok := data["from"].(string); ok {
		author = from
	}

	// Check head for additional author info
	if head, ok := data["head"].(map[string]interface{}); ok {
		if authorName, ok := head["author"].(string); ok {
			author = authorName
		}
	}

	if content == "" {
		return
	}

	t.mu.RLock()
	handler := t.onMessage
	t.mu.RUnlock()

	if handler != nil {
		handler(author, content)
	}
}

func (t *Client) handleMetaMessage(meta map[string]interface{}) {
	// Can be extended to handle topic metadata updates
	_ = meta
}

func (t *Client) handlePresMessage(pres map[string]interface{}) {
	// Can be extended to handle presence notifications (online/offline)
	_ = pres
}

func (t *Client) pingPump() {
	defer t.wg.Done()

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			if t.GetState() < StateConnected {
				return
			}

			t.mu.Lock()
			conn := t.conn
			if conn == nil {
				t.mu.Unlock()
				return
			}
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := conn.WriteMessage(websocket.PingMessage, nil)
			t.mu.Unlock()

			if err != nil {
				log.Printf("Ping failed: %v", err)
				return
			}
		}
	}
}

func (t *Client) tryReconnect() {
	t.reconnectMu.Lock()
	enabled := t.reconnectEnabled
	t.reconnectMu.Unlock()

	if !enabled {
		return
	}

	t.shutdownMu.Lock()
	shutdown := t.shutdown
	t.shutdownMu.Unlock()

	if shutdown {
		return
	}

	go t.reconnectLoop()
}

func (t *Client) reconnectLoop() {
	delay := initialReconnectDelay

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		t.reconnectMu.Lock()
		enabled := t.reconnectEnabled
		t.reconnectMu.Unlock()

		if !enabled {
			return
		}

		log.Printf("Attempting to reconnect to Tinode in %v...", delay)

		select {
		case <-t.ctx.Done():
			return
		case <-time.After(delay):
		}

		err := t.Connect()
		if err == nil {
			log.Printf("Successfully reconnected to Tinode")

			t.mu.RLock()
			handler := t.onReconnect
			t.mu.RUnlock()
			if handler != nil {
				handler()
			}
			return
		}

		log.Printf("Reconnect failed: %v", err)

		// Exponential backoff
		delay = time.Duration(float64(delay) * reconnectBackoffMult)
		if delay > maxReconnectDelay {
			delay = maxReconnectDelay
		}
	}
}

// EnsureGeneralTopic ensures the general topic exists (tries to get info, creates if not found)
func (t *Client) EnsureGeneralTopic() error {
	return t.EnsureGeneralTopicWithContext(t.ctx)
}

// EnsureGeneralTopicWithContext ensures general topic with custom context
func (t *Client) EnsureGeneralTopicWithContext(ctx context.Context) error {
	// First, try to get topic info
	id := t.nextID()
	respChan := t.registerCallback(id)
	defer t.unregisterCallback(id)

	msg := map[string]interface{}{
		"get": map[string]interface{}{
			"id":    id,
			"topic": generalTopic,
			"what":  "desc",
		},
	}

	if err := t.sendMessage(msg); err != nil {
		return err
	}

	resp, err := t.waitForResponse(ctx, respChan)
	if err != nil {
		// Topic might not exist, try to create
		return t.createGeneralTopic(ctx)
	}

	// Check if we got an error (topic doesn't exist)
	if ctrl, ok := resp["ctrl"].(map[string]interface{}); ok {
		if code, ok := ctrl["code"].(float64); ok && code >= 400 {
			// Topic doesn't exist, create it
			return t.createGeneralTopic(ctx)
		}
	}

	// Topic exists
	return nil
}

func (t *Client) createGeneralTopic(ctx context.Context) error {
	id := t.nextID()
	respChan := t.registerCallback(id)
	defer t.unregisterCallback(id)

	msg := map[string]interface{}{
		"sub": map[string]interface{}{
			"id":    id,
			"topic": "new",
			"set": map[string]interface{}{
				"desc": map[string]interface{}{
					"public": map[string]interface{}{
						"fn": "General Chat",
					},
					"defacs": map[string]interface{}{
						"auth": "JRWPS",
						"anon": "JR",
					},
				},
				"tags": []string{"general", generalTopic},
			},
		},
	}

	if err := t.sendMessage(msg); err != nil {
		return err
	}

	return t.waitForCtrlSuccess(ctx, respChan, "create general topic")
}

// GetAuthToken returns current auth token if authenticated
func (t *Client) GetAuthToken() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.authToken
}

// SubscribeToTopic subscribes to an arbitrary topic
func (t *Client) SubscribeToTopic(ctx context.Context, topic string) error {
	id := t.nextID()
	respChan := t.registerCallback(id)
	defer t.unregisterCallback(id)

	msg := map[string]interface{}{
		"sub": map[string]interface{}{
			"id":    id,
			"topic": topic,
		},
	}

	if err := t.sendMessage(msg); err != nil {
		return err
	}

	return t.waitForCtrlSuccess(ctx, respChan, "subscribe to "+topic)
}

// Leave unsubscribes from a topic
func (t *Client) Leave(ctx context.Context, topic string) error {
	id := t.nextID()
	respChan := t.registerCallback(id)
	defer t.unregisterCallback(id)

	msg := map[string]interface{}{
		"leave": map[string]interface{}{
			"id":    id,
			"topic": topic,
		},
	}

	if err := t.sendMessage(msg); err != nil {
		return err
	}

	return t.waitForCtrlSuccess(ctx, respChan, "leave "+topic)
}
