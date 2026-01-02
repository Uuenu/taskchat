package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/Uuenu/taskchat/internal/repository"
)

// Client wraps MongoDB client and provides repository access
type Client struct {
	client   *mongo.Client
	database *mongo.Database
}

// NewClient creates a new MongoDB client and connects to the database
func NewClient(ctx context.Context, uri, dbName string) (*Client, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	// Verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return &Client{
		client:   client,
		database: client.Database(dbName),
	}, nil
}

// Close disconnects from MongoDB
func (c *Client) Close(ctx context.Context) error {
	return c.client.Disconnect(ctx)
}

// Repositories returns all repository implementations
func (c *Client) Repositories() *repository.Repositories {
	return &repository.Repositories{
		Users:    NewUserRepository(c.database),
		Messages: NewMessageRepository(c.database),
	}
}

// Ping checks if the database connection is alive
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx, nil)
}
