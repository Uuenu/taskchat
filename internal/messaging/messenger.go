package messaging

import "context"

// MessageHandler is called when a new message arrives from the messenger.
// author is the external ID of the message sender (provider-specific).
// content is the message text.
// metadata contains optional provider-specific data.
type MessageHandler func(author, content string, metadata map[string]any)

// Messenger defines a provider-agnostic interface for real-time messaging.
// This abstraction allows swapping out the underlying messaging system
// (e.g., Tinode, Matrix, custom WebSocket) without changing business logic.
type Messenger interface {
	// Provider returns the unique identifier for this messenger (e.g., "tinode", "matrix")
	Provider() string

	// Lifecycle
	Connect(ctx context.Context) error
	Disconnect() error
	IsConnected() bool

	// Users
	CreateUser(ctx context.Context, username, password string) (externalID string, err error)

	// Messaging
	Send(ctx context.Context, content string) error
	SendAsync(content string) error

	// Events
	OnMessage(handler MessageHandler)
	OnReconnect(handler func())
	OnDisconnect(handler func(err error))
}
