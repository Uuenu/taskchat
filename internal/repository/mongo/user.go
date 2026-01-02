package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/Uuenu/taskchat/internal/errors"
	"github.com/Uuenu/taskchat/internal/models"
)

// externalAccountDocument represents an external account in MongoDB
type externalAccountDocument struct {
	Provider   string `bson:"provider"`
	ExternalID string `bson:"external_id"`
}

// userDocument represents the MongoDB document structure for users
type userDocument struct {
	ID               primitive.ObjectID        `bson:"_id,omitempty"`
	Email            string                    `bson:"email"`
	PasswordHash     string                    `bson:"password_hash"`
	ExternalAccounts []externalAccountDocument `bson:"external_accounts,omitempty"`
	CreatedAt        time.Time                 `bson:"created_at"`
	UpdatedAt        time.Time                 `bson:"updated_at"`
}

// toDomain converts MongoDB document to domain model
func (d *userDocument) toDomain() *models.User {
	accounts := make([]models.ExternalAccount, len(d.ExternalAccounts))
	for i, acc := range d.ExternalAccounts {
		accounts[i] = models.ExternalAccount{
			Provider:   acc.Provider,
			ExternalID: acc.ExternalID,
		}
	}

	return &models.User{
		ID:               d.ID.Hex(),
		Email:            d.Email,
		PasswordHash:     d.PasswordHash,
		ExternalAccounts: accounts,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}
}

// UserRepository implements repository.UserRepository for MongoDB
type UserRepository struct {
	collection *mongo.Collection
}

// NewUserRepository creates a new MongoDB user repository
func NewUserRepository(db *mongo.Database) *UserRepository {
	collection := db.Collection("users")

	// Create unique index on email (idempotent operation)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})

	// Create compound unique index on external_accounts (provider + external_id)
	collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "external_accounts.provider", Value: 1},
			{Key: "external_accounts.external_id", Value: 1},
		},
		Options: options.Index().SetUnique(true).SetSparse(true),
	})

	return &UserRepository{collection: collection}
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, email, passwordHash string) (*models.User, error) {
	now := time.Now()
	doc := &userDocument{
		ID:           primitive.NewObjectID(),
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	_, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, errors.ErrUserAlreadyExists
		}
		return nil, err
	}

	return doc.toDomain(), nil
}

// FindByEmail finds a user by email
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var doc userDocument
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.ErrUserNotFound
		}
		return nil, err
	}
	return doc.toDomain(), nil
}

// Exists checks if a user exists
func (r *UserRepository) Exists(ctx context.Context, email string) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"email": email})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// LinkExternalAccount links an external account to a user
func (r *UserRepository) LinkExternalAccount(ctx context.Context, email, provider, externalID string) error {
	// First, remove any existing account with same provider for this user
	// Then add the new one (upsert-like behavior per provider)
	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"email": email},
		bson.M{
			"$pull": bson.M{
				"external_accounts": bson.M{"provider": provider},
			},
		},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return errors.ErrUserNotFound
	}

	// Add the new external account
	_, err = r.collection.UpdateOne(
		ctx,
		bson.M{"email": email},
		bson.M{
			"$push": bson.M{
				"external_accounts": externalAccountDocument{
					Provider:   provider,
					ExternalID: externalID,
				},
			},
			"$set": bson.M{"updated_at": time.Now()},
		},
	)
	return err
}

// FindByExternalID finds a user by external account
func (r *UserRepository) FindByExternalID(ctx context.Context, provider, externalID string) (*models.User, error) {
	var doc userDocument
	err := r.collection.FindOne(ctx, bson.M{
		"external_accounts": bson.M{
			"$elemMatch": bson.M{
				"provider":    provider,
				"external_id": externalID,
			},
		},
	}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.ErrUserNotFound
		}
		return nil, err
	}
	return doc.toDomain(), nil
}
