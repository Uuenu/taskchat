package repository

import (
	"context"

	"github.com/Uuenu/taskchat/internal/models"
)

// UserRepository defines the interface for user data operations.
type UserRepository interface {
	Create(ctx context.Context, email, passwordHash string) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	Exists(ctx context.Context, email string) (bool, error)

	// External account linking (provider-agnostic)
	LinkExternalAccount(ctx context.Context, email, provider, externalID string) error
	FindByExternalID(ctx context.Context, provider, externalID string) (*models.User, error)
}

// MessageRepository defines the interface for message data operations.
type MessageRepository interface {
	Save(ctx context.Context, author, content string) (*models.Message, error)
	GetRecent(ctx context.Context, limit int) ([]models.Message, error)
}

// Repositories aggregates all repository interfaces
type Repositories struct {
	Users    UserRepository
	Messages MessageRepository
}
