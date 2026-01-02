package tinode

import (
	"context"

	"github.com/Uuenu/taskchat/internal/config"
	"github.com/Uuenu/taskchat/internal/messaging"
)

const providerName = "tinode"

// Tinode implements Messenger interface using Tinode as backend.
type Tinode struct {
	svc *Client
}

// Provider returns the provider identifier
func (t *Tinode) Provider() string {
	return providerName
}

// NewTinode creates a new Tinode messenger
func NewTinode(cfg *config.TinodeConfig) *Tinode {
	return &Tinode{
		svc: NewTinodeService(cfg),
	}
}

// Connect establishes connection to Tinode server
func (t *Tinode) Connect(ctx context.Context) error {
	return t.svc.ConnectWithContext(ctx)
}

// Disconnect closes connection to Tinode server
func (t *Tinode) Disconnect() error {
	return t.svc.Shutdown()
}

// IsConnected returns true if connected to Tinode
func (t *Tinode) IsConnected() bool {
	return t.svc.IsConnected()
}

// CreateUser creates a new user account in Tinode
func (t *Tinode) CreateUser(ctx context.Context, username, password string) (string, error) {
	return t.svc.CreateAccountWithContext(ctx, username, password)
}

// Send sends a message and waits for confirmation
func (t *Tinode) Send(ctx context.Context, content string) error {
	return t.svc.SendToGeneralWithContext(ctx, content)
}

// SendAsync sends a message without waiting for confirmation
func (t *Tinode) SendAsync(content string) error {
	return t.svc.SendToGeneralAsync(content)
}

// OnMessage sets callback for incoming messages
func (t *Tinode) OnMessage(handler messaging.MessageHandler) {
	t.svc.SetMessageHandler(func(author, content string) {
		handler(author, content, nil)
	})
}

// OnReconnect sets callback for reconnection events
func (t *Tinode) OnReconnect(handler func()) {
	t.svc.SetReconnectHandler(handler)
}

// OnDisconnect sets callback for disconnection events
func (t *Tinode) OnDisconnect(handler func(err error)) {
	t.svc.SetDisconnectHandler(handler)
}

// SetServiceCredentials sets service account credentials for auto-login
func (t *Tinode) SetServiceCredentials(email, password string) {
	t.svc.SetServiceCredentials(email, password)
}

// SetReconnectEnabled enables or disables auto-reconnect
func (t *Tinode) SetReconnectEnabled(enabled bool) {
	t.svc.SetReconnectEnabled(enabled)
}

// EnsureGeneralTopic ensures the general topic exists
func (t *Tinode) EnsureGeneralTopic(ctx context.Context) error {
	return t.svc.EnsureGeneralTopicWithContext(ctx)
}

// SubscribeToGeneral subscribes to the general chat room
func (t *Tinode) SubscribeToGeneral(ctx context.Context) error {
	return t.svc.SubscribeToGeneralWithContext(ctx)
}

// Login authenticates with Tinode
func (t *Tinode) Login(ctx context.Context, email, password string) (string, error) {
	return t.svc.LoginWithContext(ctx, email, password)
}

// compile-time check
var _ messaging.Messenger = (*Tinode)(nil)
