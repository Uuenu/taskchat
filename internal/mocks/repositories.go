package mocks

import (
	"context"
	"time"

	"github.com/Uuenu/taskchat/internal/errors"
	"github.com/Uuenu/taskchat/internal/messaging"
	"github.com/Uuenu/taskchat/internal/models"
)

// MockUserRepository is a mock implementation of UserRepository
type MockUserRepository struct {
	Users                   map[string]*models.User
	ExternalAccounts        map[string]map[string]string // provider -> externalID -> email
	CreateFunc              func(ctx context.Context, email, passwordHash string) (*models.User, error)
	FindByEmailFunc         func(ctx context.Context, email string) (*models.User, error)
	LinkExternalAccountFunc func(ctx context.Context, email, provider, externalID string) error
	FindByExternalIDFunc    func(ctx context.Context, provider, externalID string) (*models.User, error)
}

// NewMockUserRepository creates a new mock user repository
func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		Users:            make(map[string]*models.User),
		ExternalAccounts: make(map[string]map[string]string),
	}
}

func (m *MockUserRepository) Create(ctx context.Context, email, passwordHash string) (*models.User, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, email, passwordHash)
	}
	if _, exists := m.Users[email]; exists {
		return nil, errors.ErrUserAlreadyExists
	}
	user := &models.User{
		ID:           "test-id",
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	m.Users[email] = user
	return user, nil
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.FindByEmailFunc != nil {
		return m.FindByEmailFunc(ctx, email)
	}
	user, exists := m.Users[email]
	if !exists {
		return nil, errors.ErrUserNotFound
	}
	return user, nil
}

func (m *MockUserRepository) Exists(ctx context.Context, email string) (bool, error) {
	_, exists := m.Users[email]
	return exists, nil
}

func (m *MockUserRepository) LinkExternalAccount(ctx context.Context, email, provider, externalID string) error {
	if m.LinkExternalAccountFunc != nil {
		return m.LinkExternalAccountFunc(ctx, email, provider, externalID)
	}
	user, exists := m.Users[email]
	if !exists {
		return errors.ErrUserNotFound
	}
	// Add external account to user
	user.ExternalAccounts = append(user.ExternalAccounts, models.ExternalAccount{
		Provider:   provider,
		ExternalID: externalID,
	})
	// Track for reverse lookup
	if m.ExternalAccounts[provider] == nil {
		m.ExternalAccounts[provider] = make(map[string]string)
	}
	m.ExternalAccounts[provider][externalID] = email
	return nil
}

func (m *MockUserRepository) FindByExternalID(ctx context.Context, provider, externalID string) (*models.User, error) {
	if m.FindByExternalIDFunc != nil {
		return m.FindByExternalIDFunc(ctx, provider, externalID)
	}
	if m.ExternalAccounts[provider] == nil {
		return nil, errors.ErrUserNotFound
	}
	email, exists := m.ExternalAccounts[provider][externalID]
	if !exists {
		return nil, errors.ErrUserNotFound
	}
	return m.Users[email], nil
}

// MockMessageRepository is a mock implementation of MessageRepository
type MockMessageRepository struct {
	Messages []models.Message
	SaveFunc func(ctx context.Context, author, content string) (*models.Message, error)
}

// NewMockMessageRepository creates a new mock message repository
func NewMockMessageRepository() *MockMessageRepository {
	return &MockMessageRepository{
		Messages: make([]models.Message, 0),
	}
}

func (m *MockMessageRepository) Save(ctx context.Context, author, content string) (*models.Message, error) {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, author, content)
	}
	msg := &models.Message{
		ID:        "test-msg-id",
		Author:    author,
		Content:   content,
		Timestamp: time.Now(),
	}
	m.Messages = append(m.Messages, *msg)
	return msg, nil
}

func (m *MockMessageRepository) GetRecent(ctx context.Context, limit int) ([]models.Message, error) {
	if len(m.Messages) <= limit {
		return m.Messages, nil
	}
	return m.Messages[len(m.Messages)-limit:], nil
}

// MockMessenger is a mock implementation of messaging.Messenger
type MockMessenger struct {
	ProviderName      string
	Connected         bool
	SentMessages      []string
	CreatedUsers      map[string]string // username -> externalID
	messageHandler    messaging.MessageHandler
	reconnectHandler  func()
	disconnectHandler func(err error)
}

func NewMockMessenger() *MockMessenger {
	return &MockMessenger{
		ProviderName: "mock",
		Connected:    true,
		SentMessages: make([]string, 0),
		CreatedUsers: make(map[string]string),
	}
}

func (m *MockMessenger) Provider() string {
	return m.ProviderName
}

func (m *MockMessenger) Connect(ctx context.Context) error {
	m.Connected = true
	return nil
}

func (m *MockMessenger) Disconnect() error {
	m.Connected = false
	return nil
}

func (m *MockMessenger) IsConnected() bool {
	return m.Connected
}

func (m *MockMessenger) CreateUser(ctx context.Context, username, password string) (string, error) {
	externalID := "ext-" + username
	m.CreatedUsers[username] = externalID
	return externalID, nil
}

func (m *MockMessenger) Send(ctx context.Context, content string) error {
	m.SentMessages = append(m.SentMessages, content)
	return nil
}

func (m *MockMessenger) SendAsync(content string) error {
	m.SentMessages = append(m.SentMessages, content)
	return nil
}

func (m *MockMessenger) OnMessage(handler messaging.MessageHandler) {
	m.messageHandler = handler
}

func (m *MockMessenger) OnReconnect(handler func()) {
	m.reconnectHandler = handler
}

func (m *MockMessenger) OnDisconnect(handler func(err error)) {
	m.disconnectHandler = handler
}

// SimulateMessage simulates receiving a message (for testing)
func (m *MockMessenger) SimulateMessage(author, content string) {
	if m.messageHandler != nil {
		m.messageHandler(author, content, nil)
	}
}

// SimulateReconnect simulates a reconnection event (for testing)
func (m *MockMessenger) SimulateReconnect() {
	if m.reconnectHandler != nil {
		m.reconnectHandler()
	}
}
