package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Uuenu/taskchat/internal/mocks"
	"github.com/Uuenu/taskchat/internal/models"
)

func TestChatService_SendMessage(t *testing.T) {
	t.Run("successful send with messenger connected", func(t *testing.T) {
		msgRepo := mocks.NewMockMessageRepository()
		messenger := mocks.NewMockMessenger()
		svc := NewChatService(msgRepo, messenger)

		err := svc.SendMessage(context.Background(), "user@test.com", "Hello world")

		assert.NoError(t, err)
		assert.Len(t, msgRepo.Messages, 1)
		assert.Equal(t, "user@test.com", msgRepo.Messages[0].Author)
		assert.Equal(t, "Hello world", msgRepo.Messages[0].Content)
	})

	t.Run("successful send with messenger disconnected", func(t *testing.T) {
		msgRepo := mocks.NewMockMessageRepository()
		messenger := mocks.NewMockMessenger()
		messenger.Connected = false
		svc := NewChatService(msgRepo, messenger)

		err := svc.SendMessage(context.Background(), "user@test.com", "Hello world")

		// Should still succeed - graceful degradation
		assert.NoError(t, err)
		assert.Len(t, msgRepo.Messages, 1)
	})

	t.Run("successful send with nil messenger", func(t *testing.T) {
		msgRepo := mocks.NewMockMessageRepository()
		svc := NewChatService(msgRepo, nil)

		err := svc.SendMessage(context.Background(), "user@test.com", "Hello world")

		assert.NoError(t, err)
		assert.Len(t, msgRepo.Messages, 1)
	})

	t.Run("repository error", func(t *testing.T) {
		msgRepo := mocks.NewMockMessageRepository()
		msgRepo.SaveFunc = func(ctx context.Context, author, content string) (*models.Message, error) {
			return nil, errors.New("database error")
		}
		svc := NewChatService(msgRepo, nil)

		err := svc.SendMessage(context.Background(), "user@test.com", "Hello")

		assert.Error(t, err)
	})
}

func TestChatService_GetRecentMessages(t *testing.T) {
	t.Run("returns messages in order", func(t *testing.T) {
		msgRepo := mocks.NewMockMessageRepository()
		svc := NewChatService(msgRepo, nil)

		// Add some messages
		msgRepo.Save(context.Background(), "user1", "First")
		msgRepo.Save(context.Background(), "user2", "Second")
		msgRepo.Save(context.Background(), "user1", "Third")

		messages, err := svc.GetRecentMessages(context.Background())

		require.NoError(t, err)
		assert.Len(t, messages, 3)
		assert.Equal(t, "First", messages[0].Content)
		assert.Equal(t, "Third", messages[2].Content)
	})

	t.Run("empty repository", func(t *testing.T) {
		msgRepo := mocks.NewMockMessageRepository()
		svc := NewChatService(msgRepo, nil)

		messages, err := svc.GetRecentMessages(context.Background())

		require.NoError(t, err)
		assert.Empty(t, messages)
	})
}
