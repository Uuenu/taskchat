package service

import (
	"context"

	"github.com/Uuenu/taskchat/internal/messaging"
	"github.com/Uuenu/taskchat/internal/models"
	"github.com/Uuenu/taskchat/internal/repository"
)

const (
	// DefaultMessageLimit is the default number of messages to retrieve
	DefaultMessageLimit = 50
)

// ChatService handles chat messaging business logic
type ChatService struct {
	messageRepo repository.MessageRepository
	messenger   messaging.Messenger
}

// NewChatService creates a new chat service
func NewChatService(messageRepo repository.MessageRepository, messenger messaging.Messenger) *ChatService {
	return &ChatService{
		messageRepo: messageRepo,
		messenger:   messenger,
	}
}

// SendMessage sends a message to the general chat room
func (s *ChatService) SendMessage(ctx context.Context, author, content string) error {
	// Always persist to database first (primary storage)
	_, err := s.messageRepo.Save(ctx, author, content)
	if err != nil {
		return err
	}

	// Send to messenger for real-time delivery (best effort)
	// If messenger fails, message is still saved - graceful degradation
	if s.messenger != nil && s.messenger.IsConnected() {
		go func() {
			_ = s.messenger.SendAsync(content)
		}()
	}

	return nil
}

// GetRecentMessages retrieves recent messages from the general chat
func (s *ChatService) GetRecentMessages(ctx context.Context) ([]models.Message, error) {
	return s.messageRepo.GetRecent(ctx, DefaultMessageLimit)
}

// SaveIncomingMessage saves a message received from messenger (real-time)
func (s *ChatService) SaveIncomingMessage(ctx context.Context, author, content string) error {
	_, err := s.messageRepo.Save(ctx, author, content)
	return err
}
