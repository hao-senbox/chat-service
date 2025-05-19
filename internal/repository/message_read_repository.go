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
	// GetReadStatus(ctx context.Context, messageIDs []string, groupID primitive.ObjectID) (map[string][]models.ReadReceipt, error)
	// GetUnreadMessages(ctx context.Context, userID string, groupID primitive.ObjectID) ([]primitive.ObjectID, error)
	// GetReadByUserIDs(ctx context.Context, messageID primitive.ObjectID, groupID primitive.ObjectID) ([]string, error)
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

// func (r *readMessageRepository) GetReadByUserIDs(ctx context.Context, messageID primitive.ObjectID, groupID primitive.ObjectID) ([]string, error) {
// 	filter := bson.M{
// 		"message_id": messageID,
// 		"group_id":   groupID,
// 	}

// 	var result bson.M
// 	err := r.collection.FindOne(ctx, filter).Decode(&result)
// 	if err != nil {
// 		if err == mongo.ErrNoDocuments {
// 			return []string{}, nil
// 		}
// 		return nil, err
// 	}

// 	readByRaw, ok := result["read_by"].(bson.M)
// 	if !ok {
// 		return nil, fmt.Errorf("read_by field not found or invalid")
// 	}

// 	var userIDs []string
// 	for userID := range readByRaw {
// 		userIDs = append(userIDs, userID)
// 	}

// 	return userIDs, nil
// }

// func (r *readMessageRepository) GetReadStatus(ctx context.Context, messageIDs []string, groupID primitive.ObjectID) (map[string][]models.ReadReceipt, error) {
// 	var objectIDs []primitive.ObjectID
// 	for _, id := range messageIDs {
// 		oid, err := primitive.ObjectIDFromHex(id)
// 		if err != nil {
// 			continue // hoặc log nếu cần
// 		}
// 		objectIDs = append(objectIDs, oid)
// 	}
	
// 	filter := bson.M{
// 		"group_id": groupID,
// 		"message_id": bson.M{
// 			"$in": objectIDs,
// 		},
// 	}
	
// 	cursor, err := r.collection.Find(ctx, filter)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer cursor.Close(ctx)

// 	results := make(map[string][]models.ReadReceipt)

// 	for cursor.Next(ctx) {

// 		var doc struct {
// 			MessageID primitive.ObjectID            `bson:"message_id"`
// 			GroupID   primitive.ObjectID            `bson:"group_id"`
// 			ReadBy    map[string]primitive.DateTime `bson:"read_by"`
// 		}

// 		if err := cursor.Decode(&doc); err != nil {
// 			return nil, err
// 		}

// 		var receipts []models.ReadReceipt

// 		for userID, timestamp := range doc.ReadBy {
// 			receipts = append(receipts, models.ReadReceipt{
// 				UserID: userID,
// 				ReadAt: timestamp.Time(),
// 			})
// 		}

// 		results[doc.MessageID.Hex()] = receipts

// 	}

// 	return results, nil
// }

// func (r *readMessageRepository) GetUnreadMessages(ctx context.Context, userID string, groupID primitive.ObjectID) ([]primitive.ObjectID, error) {
	
// 	filter := bson.M {
// 		"group_id": groupID,
// 		"sender_id": bson.M{"$ne": userID},
// 		"read_by." + userID: bson.M{"$exists": false},
// 	}

// 	cursor, err := r.collection.Find(ctx, filter)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer cursor.Close(ctx)

// 	var messageIDs []primitive.ObjectID
	
// 	for cursor.Next(ctx) {
	
// 		var doc struct {
// 			MessageID primitive.ObjectID `bson:"message_id" json:"message_id"`
// 		}

// 		if err := cursor.Decode(&doc); err != nil {
// 			return nil, err
// 		}

// 		messageIDs = append(messageIDs, doc.MessageID)
// 	}

// 	if err := cursor.Err(); err != nil {
// 		return nil, err
// 	}

// 	return messageIDs, nil
// }

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
