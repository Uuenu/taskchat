package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/Uuenu/taskchat/internal/models"
)

// messageDocument represents the MongoDB document structure for messages
type messageDocument struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Author    string             `bson:"author"`
	Content   string             `bson:"content"`
	Timestamp time.Time          `bson:"timestamp"`
}

// toDomain converts MongoDB document to domain model
func (d *messageDocument) toDomain() models.Message {
	return models.Message{
		ID:        d.ID.Hex(),
		Author:    d.Author,
		Content:   d.Content,
		Timestamp: d.Timestamp,
	}
}

// MessageRepository implements repository.MessageRepository for MongoDB
type MessageRepository struct {
	collection *mongo.Collection
}

// NewMessageRepository creates a new MongoDB message repository
func NewMessageRepository(db *mongo.Database) *MessageRepository {
	collection := db.Collection("messages")

	// Create index on timestamp for efficient sorting
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "timestamp", Value: -1}},
	})

	return &MessageRepository{collection: collection}
}

// Save persists a new message
func (r *MessageRepository) Save(ctx context.Context, author, content string) (*models.Message, error) {
	doc := &messageDocument{
		ID:        primitive.NewObjectID(),
		Author:    author,
		Content:   content,
		Timestamp: time.Now(),
	}

	_, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}

	msg := doc.toDomain()
	return &msg, nil
}

// GetRecent retrieves the last N messages
func (r *MessageRepository) GetRecent(ctx context.Context, limit int) ([]models.Message, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []messageDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	// Convert to domain models and reverse for chronological order
	messages := make([]models.Message, len(docs))
	for i, doc := range docs {
		messages[len(docs)-1-i] = doc.toDomain()
	}

	return messages, nil
}
