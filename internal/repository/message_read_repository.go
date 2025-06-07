package repository

import (
	"context"
	"log"
	"time"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ReadMessageRepository interface {
	MarkAsRead(ctx context.Context, messageID primitive.ObjectID, userID string, senderID string, groupID primitive.ObjectID) error
	EnsureIndexes(ctx context.Context) error
}

type readMessageRepository struct {
	collection *mongo.Collection
}

func NewReadMessageRepository(collection *mongo.Collection) ReadMessageRepository {
	return &readMessageRepository{
		collection: collection,
	}
}

func (r *readMessageRepository) MarkAsRead(ctx context.Context, messageID primitive.ObjectID, userID string, senderID string, groupID primitive.ObjectID) error {

	now := primitive.NewDateTimeFromTime(time.Now())

	filter := bson.M{
		"message_id": messageID,
		"group_id":   groupID,
	}

	update := bson.M{
		"$set": bson.M{
			"react_by." + userID: now,
		},
		"$setOnInsert": bson.M{
			"sender_id": senderID,
		},
	}

	opts := options.Update().SetUpsert(true)

	_, err := r.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			_, err = r.collection.UpdateOne(ctx, filter, update, nil)
			if err != nil {
				return err 
			}
		}
	}

	return nil
}

func (r *readMessageRepository) EnsureIndexes(ctx context.Context) error {
    // Create a unique compound index on message_id and group_id
    indexModel := mongo.IndexModel{
        Keys: bson.D{
            {Key: "message_id", Value: 1},
            {Key: "group_id", Value: 1},
        },
        Options: options.Index().SetUnique(true),
    }
    
    _, err := r.collection.Indexes().CreateOne(ctx, indexModel)
    if err != nil {
        log.Printf("Failed to create unique index: %v", err)
        return err
    }
    
    log.Println("Successfully created unique index on message_id and group_id")
    return nil
}
