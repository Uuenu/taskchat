package models

import (
	"time"
)

// ExternalAccount represents a linked external service account
type ExternalAccount struct {
	Provider   string `bson:"provider" json:"provider"`     // e.g., "tinode", "matrix"
	ExternalID string `bson:"external_id" json:"external_id"`
}

// User represents a registered user in the system
type User struct {
	ID               string            `bson:"_id,omitempty" json:"id,omitempty"`
	Email            string            `bson:"email" json:"email"`
	PasswordHash     string            `bson:"password_hash" json:"-"`
	ExternalAccounts []ExternalAccount `bson:"external_accounts,omitempty" json:"-"`
	CreatedAt        time.Time         `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time         `bson:"updated_at" json:"updated_at"`
}

// GetExternalID returns the external ID for a given provider, or empty string if not found
func (u *User) GetExternalID(provider string) string {
	for _, acc := range u.ExternalAccounts {
		if acc.Provider == provider {
			return acc.ExternalID
		}
	}
	return ""
}

// Message represents a chat message in the general room
type Message struct {
	ID        string    `bson:"_id,omitempty" json:"id,omitempty"`
	Author    string    `bson:"author" json:"author"`
	Content   string    `bson:"content" json:"content"`
	Timestamp time.Time `bson:"timestamp" json:"timestamp"`
}
